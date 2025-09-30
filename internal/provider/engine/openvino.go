//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

package engine

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/version"
)

const (
	ModelScopeSCHEME               = "https"
	ModelScopeEndpointCN           = "www.modelscope.cn"
	ModelScopeEndpointAI           = "www.modelscope.ai"
	ModelScopeGetModelFilesReqPath = "/api/v1/models/%s/repo/files?Revision=%s&Recursive=%s"
	ModelScopeModelDownloadReqPath = "/api/v1/models/%s/repo?Revision=%s&FilePath=%s"
	ModelScopeRevision             = "master"
	BufferSize                     = 64 * 1024

	// OpenVINO Server configuration
	OpenvinoGRPCPort     = "9000"
	OpenvinoGRPCHost     = constants.DefaultHost + ":" + OpenvinoGRPCPort
	OpenvinoHTTPPort     = "16666"
	OpenvinoHTTPHost     = constants.DefaultHost + ":" + OpenvinoHTTPPort
	OpenvinoVersion      = "2025.0.0"
	OpenvinoDefaultModel = "stable-diffusion-v-1-5-ov-fp16"

	// Download URLs
	OVMSWindowsDownloadURL       = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ovms_windows.zip"
	OVMSLinuxRedHatDownloadURL   = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ovms_redhat_python_on.tar.gz"
	OVMSLinuxUbuntu22DownloadURL = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ovms_ubuntu22_python_on.tar.gz"
	OVMSLinuxUbuntu24DownloadURL = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ovms_ubuntu24_python_on.tar.gz"

	ScriptsDownloadURL = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/scripts.zip"

	// Required Version
	OpenvinoMinVersion = "2025.2.0"
)

type OpenvinoProvider struct {
	EngineConfig   *types.EngineRecommendConfig
	processManager *process.EngineProcessManager
}

func QuotePlus(s string) string {
	// First perform standard URL encoding
	encoded := url.QueryEscape(s)
	// Replace spaces with + signs (Python quote_plus behavior)
	encoded = strings.ReplaceAll(encoded, "%20", "+")
	// Special handling for plus signs themselves
	encoded = strings.ReplaceAll(encoded, "+", "%2B")
	return encoded
}

type ModelScopeFileReqData struct {
	ModelName string `json:"model_name"`
	Revision  string `json:"revision"`
	Recursive string `json:"recursive"`
	FilePath  string `json:"file_path"`
	Stream    bool   `json:"stream"`
}

type ModelScopeFileRespData struct {
	Code int                `json:"Code"`
	Data ModelScopeFileData `json:"Data"`
}

type ModelScopeFileData struct {
	Files []ModelScopeFile `json:"Files"`
}

type ModelScopeFile struct {
	Name     string `json:"Name"`
	Path     string `json:"Path"`
	Digest   string `json:"Sha256"`
	Size     int64  `json:"Size"`
	IsLFS    bool   `json:"IsLFS"`
	Revision string `json:"Revision"`
	Type     string `json:"Type"`
}

type AsyncDownloadModelFileData struct {
	ModelName      string
	ModelType      string
	DataCh         chan []byte
	ErrCh          chan error
	ModelFiles     []ModelScopeFile
	LocalModelPath string
}

func CheckFileDigest(ExceptDigest string, FilePath string) bool {
	// open file
	file, err := os.Open(FilePath)
	if err != nil {
		os.RemoveAll(FilePath)
		return false
	}
	defer file.Close()

	// create SHA-256
	hash := sha256.New()

	buf := make([]byte, BufferSize)

	// Read the file in chunks and update the hash
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return false
		}
		if n == 0 { // read finish
			break
		}

		hash.Write(buf[:n]) // update hash
	}
	hexDigest := hex.EncodeToString(hash.Sum(nil))
	if ExceptDigest != hexDigest {
		os.RemoveAll(FilePath)
		return false
	}
	return true
}

func GetHttpClient() *client.Client {
	d := GetModelScopeDomain(true)
	return client.NewClient(&url.URL{
		Scheme: ModelScopeSCHEME,
		Host:   d,
	}, http.DefaultClient)
}

func GetModelScopeDomain(cnSite bool) string {
	if cnSite {
		return ModelScopeEndpointCN
	} else {
		return ModelScopeEndpointAI
	}
}

func GetModelFiles(ctx context.Context, reqData *ModelScopeFileReqData) ([]ModelScopeFile, error) {
	c := GetHttpClient()
	filesReqPath := fmt.Sprintf(ModelScopeGetModelFilesReqPath, reqData.ModelName, reqData.Revision, reqData.Recursive)
	var resp *ModelScopeFileRespData
	err := c.Do(ctx, "GET", filesReqPath, nil, &resp)
	if err != nil {
		return []ModelScopeFile{}, err
	}
	var newResp []ModelScopeFile
	for _, file := range resp.Data.Files {
		if file.Name == ".gitignore" || file.Name == ".gitmodules" || file.Type == "tree" {
			continue
		}
		newResp = append(newResp, file)
	}
	return newResp, err
}

func DownloadModelFileRequest(ctx context.Context, reqData *ModelScopeFileReqData, reqHeader map[string]string) (chan []byte, chan error) {
	c := GetHttpClient()
	modelReqPath := fmt.Sprintf(ModelScopeModelDownloadReqPath, reqData.ModelName, reqData.Revision, reqData.FilePath)
	dataCh, errCh := c.StreamResponse(ctx, "GET", modelReqPath, nil, reqHeader)
	return dataCh, errCh
}

func AsyncDownloadModelFile(ctx context.Context, a AsyncDownloadModelFileData, engine *OpenvinoProvider) {
	defer close(a.DataCh)
	defer close(a.ErrCh)

	for _, fileData := range a.ModelFiles {
		if err := downloadSingleFile(ctx, a, fileData); err != nil {
			a.ErrCh <- err
			logger.EngineLogger.Error("[OpenVINO] Failed to download file: " + fileData.Name + " " + err.Error())
			return
		}
		logger.EngineLogger.Debug("[OpenVINO] Downloaded file: " + fileData.Name)
	}

	logger.EngineLogger.Debug("[OpenVINO] Generating graph.pbtxt for model: " + a.ModelName)
	if err := engine.generateGraphPBTxt(a.ModelName, a.ModelType); err != nil {
		slog.Error("Failed to generate graph.pbtxt", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to generate graph.pbtxt: " + err.Error())
		a.ErrCh <- errors.New("Failed to generate graph.pbtxt: " + err.Error())
		return
	}

	logger.EngineLogger.Info("[OpenVINO] Pull model completed: " + a.ModelName)
	resp := types.ProgressResponse{Status: "success"}
	if data, err := json.Marshal(resp); err == nil {
		a.DataCh <- data
	} else {
		a.ErrCh <- err
	}
}

func downloadSingleFile(ctx context.Context, a AsyncDownloadModelFileData, fileData ModelScopeFile) error {
	filePath := filepath.Join(a.LocalModelPath, fileData.Path)

	// Create directory (if needed)
	if strings.Contains(fileData.Path, "/") {
		if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil {
			return err
		}
	}

	// Open file (append mode)
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Get current file length (for resume download)
	partSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// If file already exists and size matches, perform hash verification
	if partSize >= fileData.Size {
		if CheckFileDigest(fileData.Digest, filePath) {
			return nil // Skip download
		}
		// Delete corrupted file, re-download
		_ = os.Remove(filePath)
		return downloadSingleFile(ctx, a, fileData)
	}

	// Construct request
	headers := map[string]string{
		"Range":               fmt.Sprintf("bytes=%d-%d", partSize, fileData.Size-1),
		"snapshot-identifier": uuid.New().String(),
		"X-Request-ID":        strings.ReplaceAll(uuid.New().String(), "-", ""),
	}
	fp := QuotePlus(fileData.Path)
	reqData := &ModelScopeFileReqData{
		ModelName: a.ModelName,
		Revision:  ModelScopeRevision,
		FilePath:  fp,
		Stream:    true,
	}

	reqDataCh, reqErrCh := DownloadModelFileRequest(ctx, reqData, headers)

	// Download content
	digest := sha256.New()
	for {
		select {
		case data, ok := <-reqDataCh:
			if !ok {
				// Check file integrity
				if partSize != fileData.Size {
					return fmt.Errorf("file %s incomplete: got %d bytes, expected %d", fileData.Name, partSize, fileData.Size)
				}

				// First check the digest calculated during download
				downloadHash := hex.EncodeToString(digest.Sum(nil))
				if downloadHash != fileData.Digest {
					logger.EngineLogger.Warn("[OpenVINO] Download digest mismatch for file %s, recalculating from file: expected %s, got %s",
						fileData.Name, fileData.Digest, downloadHash)

					// Re-read file to calculate digest
					if CheckFileDigest(fileData.Digest, filePath) {
						logger.EngineLogger.Info("[OpenVINO] File digest verification passed after recalculation for file: %s", fileData.Name)
						return nil
					} else {
						// Delete corrupted file and re-download
						logger.EngineLogger.Error("[OpenVINO] File digest verification failed after recalculation for file: %s, will retry download", fileData.Name)
						_ = os.Remove(filePath)
						return downloadSingleFile(ctx, a, fileData)
					}
				}

				logger.EngineLogger.Debug("[OpenVINO] File download completed successfully: %s", fileData.Name)
				return nil // Completed
			}
			if len(data) == 0 {
				continue
			}
			n, err := f.Write(data)
			if err != nil {
				return err
			}
			digest.Write(data)
			partSize += int64(n)

			// Write progress
			progress := types.ProgressResponse{
				Status:    fmt.Sprintf("pulling %s", fileData.Name),
				Digest:    fileData.Digest,
				Total:     fileData.Size,
				Completed: partSize,
			}
			if dataBytes, err := json.Marshal(progress); err == nil {
				a.DataCh <- dataBytes
			}
		case err := <-reqErrCh:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func NewOpenvinoProvider(config *types.EngineRecommendConfig) *OpenvinoProvider {
	if config != nil {
		provider := &OpenvinoProvider{
			EngineConfig: config,
		}
		provider.processManager = process.NewEngineProcessManager("openvino", config)
		return provider
	}

	AOGDir, err := utils.GetAOGDataDir()
	if err != nil {
		logger.EngineLogger.Error("[OpenVINO] Get AOG data dir failed: " + err.Error())
		return nil
	}

	openvinoPath := fmt.Sprintf("%s/%s/%s", AOGDir, "engine", "openvino")
	if _, err := os.Stat(openvinoPath); os.IsNotExist(err) {
		err := os.MkdirAll(openvinoPath, 0o750)
		if err != nil {
			logger.EngineLogger.Error("[OpenVINO] Create openvino path failed: " + err.Error())
			return nil
		}
	}

	openvinoProvider := new(OpenvinoProvider)
	openvinoProvider.EngineConfig = openvinoProvider.GetConfig()
	if openvinoProvider.EngineConfig == nil {
		logger.EngineLogger.Error("[OpenVINO] OpenVINO engine is not available")
		return nil
	}
	openvinoProvider.processManager = process.NewEngineProcessManager("openvino", openvinoProvider.EngineConfig)

	return openvinoProvider
}

func (o *OpenvinoProvider) GetDefaultClient() *client.GRPCClient {
	grpcClient, err := client.NewGRPCClient(OpenvinoGRPCHost)
	if err != nil {
		logger.EngineLogger.Error("[OpenVINO] Failed to create gRPC client: " + err.Error())
		return nil
	}

	return grpcClient
}

func (o *OpenvinoProvider) StartEngine(mode string) error {
	// Always use new process manager
	if o.processManager == nil {
		o.processManager = process.NewEngineProcessManager("openvino", o.EngineConfig)
	}

	if err := o.processManager.StartEngine(mode, o.HealthCheck); err != nil {
		// If engine not found, this is expected behavior - just log and return success
		if strings.Contains(err.Error(), "executable not found") {
			logger.EngineLogger.Info("[OpenVINO] Engine not installed, skipping startup")
			return nil
		}
		// For other errors, return the error directly
		return fmt.Errorf("failed to start openvino engine: %v", err)
	}

	return nil
}

func (o *OpenvinoProvider) StopEngine() error {
	// Always use new process manager
	if o.processManager != nil {
		return o.processManager.StopEngine()
	}

	// If process manager is not initialized, nothing to stop
	return nil
}

// SetProcessManager sets the process manager for the provider
func (o *OpenvinoProvider) SetProcessManager(pm *process.EngineProcessManager) {
	o.processManager = pm
}

func (o *OpenvinoProvider) GetConfig() *types.EngineRecommendConfig {
	downloadPath, err := utils.GetDownloadDir()
	if _, err = os.Stat(downloadPath); os.IsNotExist(err) {
		err = os.MkdirAll(downloadPath, 0o755)
		if err != nil {
			slog.Error("Create download path failed: " + err.Error())
			logger.EngineLogger.Error("[OpenVINO] Create download path failed: " + err.Error())
			return nil
		}
	}

	AOGDir, err := utils.GetAOGDataDir()
	if err != nil {
		slog.Error("Get AOG data dir failed: " + err.Error())
		logger.EngineLogger.Error("[OpenVINO] Get AOG data dir failed: " + err.Error())
		return nil
	}
	execFile := ""
	execPath := ""
	downloadUrl := ""
	enginePath := ""
	switch runtime.GOOS {
	case "windows":
		execPath = fmt.Sprintf("%s/%s", AOGDir, "engine/openvino/ovms")
		execFile = "ovms.exe"
		downloadUrl = OVMSWindowsDownloadURL
		enginePath = fmt.Sprintf("%s/%s", AOGDir, "engine/openvino")
	case "linux":
		execFile = "ovms"
		execPath = "/opt/aog/engine/openvino"
		enginePath = fmt.Sprintf("%s/%s", AOGDir, "engine/openvino")

		// Detect Linux distribution and version
		distro, version, err := detectLinuxDistribution()
		if err != nil {
			slog.Error("Failed to detect Linux distribution: " + err.Error())
			logger.EngineLogger.Error("[OpenVINO] Failed to detect Linux distribution: " + err.Error())
			return nil
		}

		// Select download URL based on distribution and version
		switch distro {
		case "ubuntu":
			switch version {
			case "22.04":
				downloadUrl = OVMSLinuxUbuntu22DownloadURL
			case "24.04":
				downloadUrl = OVMSLinuxUbuntu24DownloadURL
			default:
				slog.Error("Unsupported Ubuntu version: " + version)
				logger.EngineLogger.Error("[OpenVINO] Unsupported Ubuntu version: " + version)
				return nil
			}
		case "rhel", "centos", "rocky", "almalinux":
			// RedHat-based distributions
			downloadUrl = OVMSLinuxRedHatDownloadURL
		case "deepin":
			downloadUrl = OVMSLinuxUbuntu22DownloadURL
		default:
			slog.Error("Unsupported Linux distribution: " + distro)
			logger.EngineLogger.Error("[OpenVINO] Unsupported Linux distribution: " + distro)
			return nil
		}
	case "darwin":
		execFile = "ovms"
		execPath = fmt.Sprintf("%s/%s", AOGDir, "engine/openvino/ovms")
		downloadUrl = ""
		enginePath = fmt.Sprintf("%s/%s", AOGDir, "engine/openvino")
	default:
		slog.Error("Unsupported OS: " + runtime.GOOS)
		logger.EngineLogger.Error("[OpenVINO] Unsupported OS: " + runtime.GOOS)
		return nil
	}

	return &types.EngineRecommendConfig{
		Host:           OpenvinoGRPCHost,
		Origin:         "127.0.0.1",
		Scheme:         types.ProtocolHTTP,
		RecommendModel: OpenvinoDefaultModel,
		DownloadUrl:    downloadUrl,
		DownloadPath:   downloadPath,
		EnginePath:     enginePath,
		ExecPath:       execPath,
		ExecFile:       execFile,
	}
}

func (o *OpenvinoProvider) HealthCheck() error {
	c := o.GetDefaultClient()
	health, err := c.ServerLive()
	if err != nil || !health.GetLive() {
		logger.EngineLogger.Debug("[OpenVINO] OpenVINO Model Server is not healthy: " + err.Error())
		return err
	}

	logger.EngineLogger.Info("[OpenVINO] OpenVINO Model Server is healthy")
	return nil
}

func (o *OpenvinoProvider) GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error) {
	// Add to PATH
	setupVarsPath := filepath.Join(o.EngineConfig.ExecPath, "setupvars.bat")
	ovmsExePath := filepath.Join(o.EngineConfig.ExecPath, o.EngineConfig.ExecFile)

	batchContent := fmt.Sprintf(`@echo off
call "%s"
"%s" --version
`, setupVarsPath, ovmsExePath)

	tmpBatchFile := filepath.Join(os.TempDir(), "get_ovms_version.bat")
	if err := os.WriteFile(tmpBatchFile, []byte(batchContent), 0o644); err != nil {
		return nil, fmt.Errorf("failed to create temp batch file: %v", err)
	}
	defer os.Remove(tmpBatchFile)

	cmd := exec.Command("cmd", "/C", tmpBatchFile)
	cmd.Dir = o.EngineConfig.ExecPath

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		logger.EngineLogger.Error("[OpenVINO] GetVersion command failed: " + err.Error() + ", output: " + out.String())
		return nil, fmt.Errorf("failed to get ovms version: %v, output: %s", err, out.String())
	}

	lines := strings.Split(out.String(), "\n")
	versionStr := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "OpenVINO backend") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				rawVer := parts[2]
				verParts := strings.Split(rawVer, ".")
				if len(verParts) >= 3 {
					versionStr = fmt.Sprintf("%s.%s.%s", verParts[0], verParts[1], verParts[2])
				} else {
					versionStr = rawVer
				}
				break
			}
		}
	}
	if versionStr == "" {
		logger.EngineLogger.Error("[OpenVINO] Failed to parse version from output: " + out.String())
		return nil, fmt.Errorf("failed to parse version from output: %s", out.String())
	}

	resp.Version = versionStr
	return resp, nil
}

// CheckEngine checks if OpenVINO engine is installed
func (o *OpenvinoProvider) CheckEngine() bool {
	if o.EngineConfig == nil {
		return false
	}

	// Only supports Windows and Linux
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return false
	}

	// Check if model directory exists
	modelDir := fmt.Sprintf("%s/models", o.EngineConfig.EnginePath)
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		return false
	}

	// Check if config file exists
	configFile := fmt.Sprintf("%s/config.json", modelDir)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return false
	}

	// Check if executable file exists (use correct path and filename based on platform)
	var execPath string
	switch runtime.GOOS {
	case "windows":
		// Windows: ExecPath + ovms.exe
		execPath = filepath.Join(o.EngineConfig.ExecPath, o.EngineConfig.ExecFile)
	case "linux":
		// Linux: ExecPath + ovms/bin/ovms
		execPath = filepath.Join(o.EngineConfig.ExecPath, "ovms", "bin", o.EngineConfig.ExecFile)
	}

	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		return false
	}

	// Additional check for Python script dependencies (Linux specific)
	if runtime.GOOS == "linux" {
		pythonDir := filepath.Join(o.EngineConfig.ExecPath, "ovms", "lib", "python")
		if _, err := os.Stat(pythonDir); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func (o *OpenvinoProvider) InstallEngine(cover bool) error {
	modelDir := fmt.Sprintf("%s/models", o.EngineConfig.EnginePath)

	if runtime.GOOS == "linux" {
		logger.EngineLogger.Error("[OpenVINO] Install models on Linux is not supported currently")
		return fmt.Errorf("openVINO Model Server installation is only supported on Windows currently")
	}
	// When cover installing, first delete the entire EnginePath directory (including models, ovms, scripts, etc.)
	if cover {
		if _, err := os.Stat(o.EngineConfig.EnginePath); err == nil {
			maxRetry := 5
			wait := time.Second
			var lastErr error
			for i := 0; i < maxRetry; i++ {
				lastErr = removeDirWithSkipLockedFiles(o.EngineConfig.EnginePath)
				if lastErr == nil {
					break
				}
				if strings.Contains(lastErr.Error(), "Access is denied") {
					logger.EngineLogger.Warn(fmt.Sprintf("[OpenVINO] RemoveAll Access is denied, retrying... (%d/%d)", i+1, maxRetry))
					time.Sleep(wait)
					continue
				}
				break
			}
			if lastErr != nil {
				slog.Error("Failed to remove old engine directory", "error", lastErr)
				logger.EngineLogger.Error("[OpenVINO] Failed to remove old engine directory: " + lastErr.Error())
				return fmt.Errorf("failed to remove old engine directory: %v", lastErr)
			}
		}
	}

	// Recreate models directory
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		err := os.MkdirAll(modelDir, 0o750)
		if err != nil {
			slog.Error("Failed to create models directory", "error", err)
			logger.EngineLogger.Error("[OpenVINO] Failed to create models directory: " + err.Error())
			return err
		}
	}
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		err := os.MkdirAll(modelDir, 0o750)
		if err != nil {
			slog.Error("Failed to create models directory", "error", err)
			logger.EngineLogger.Error("[OpenVINO] Failed to create models directory: " + err.Error())
			return err
		}
	}

	// Create empty config.json file
	configFile := fmt.Sprintf("%s/config.json", modelDir)
	_, err := os.Create(configFile)
	if err != nil {
		slog.Error("Failed to create config.json", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to create config.json: " + err.Error())
		return fmt.Errorf("failed to create config.json: %v", err)
	}
	// Write default config configuration
	defaultConfig := OpenvinoModelServerConfig{
		MediapipeConfigList: []ModelConfig{},
		ModelConfigList:     []interface{}{},
	}
	err = o.saveConfig(&defaultConfig)
	if err != nil {
		slog.Error("Failed to save config.json", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to save config.json: " + err.Error())
		return fmt.Errorf("failed to save config.json: %v", err)
	}

	file, err := utils.DownloadFile(o.EngineConfig.DownloadUrl, o.EngineConfig.DownloadPath, cover)
	if err != nil {
		slog.Error("Failed to download OpenVINO Model Server", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to download OpenVINO Model Server: " + err.Error())
		return fmt.Errorf("failed to download ovms: %v", err)
	}

	// Extract ovms files
	if runtime.GOOS == "linux" {
		err = utils.UnzipFile(file, o.EngineConfig.ExecPath)
	} else {
		err = utils.UnzipFile(file, o.EngineConfig.EnginePath)
	}
	if err != nil {
		slog.Error("Failed to unzip OpenVINO Model Server", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to unzip OpenVINO Model Server: " + err.Error())
		return fmt.Errorf("failed to unzip ovms: %v", err)
	}

	// Download Python script file archive
	scriptZipUrl := ScriptsDownloadURL
	scriptZipFile, err := utils.DownloadFile(scriptZipUrl, o.EngineConfig.EnginePath, cover)
	if err != nil {
		slog.Error("Failed to download scripts.zip", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to download scripts.zip: " + err.Error())
		return fmt.Errorf("failed to download scripts.zip: %v", err)
	}

	// Extract Python script files
	err = utils.UnzipFile(scriptZipFile, o.EngineConfig.EnginePath)
	if err != nil {
		slog.Error("Failed to unzip scripts.zip", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to unzip scripts.zip: " + err.Error())
		return fmt.Errorf("failed to unzip scripts.zip: %v", err)
	}

	// Cross-platform handling: choose different script execution methods based on operating system
	var cmd *exec.Cmd
	var tmpScriptFile string
	var stdout, stderr bytes.Buffer

	if runtime.GOOS == "windows" {
		// Windows processing logic
		execPath := strings.Replace(o.EngineConfig.ExecPath, "/", "\\", -1)
		enginePath := strings.Replace(o.EngineConfig.EnginePath, "/", "\\", -1)

		// 1. Construct batch commands (ensure all commands execute in the same session)
		batchContent := fmt.Sprintf(InitShellWin, execPath, execPath, enginePath)

		logger.EngineLogger.Debug("[OpenVINO] Batch content: " + batchContent)

		// 2. Create temporary batch file
		tmpScriptFile = filepath.Join(os.TempDir(), "run_install.bat")
		if err := os.WriteFile(tmpScriptFile, []byte(batchContent), 0o644); err != nil {
			slog.Error("Failed to create temp batch file", "error", err)
			logger.EngineLogger.Error("[OpenVINO] Failed to create temp batch file: " + err.Error())
			return fmt.Errorf("failed to create temp batch file: %v", err)
		}

		// 3. Execute batch file
		cmd = exec.Command("cmd", "/C", tmpScriptFile)
		cmd.Dir = o.EngineConfig.EnginePath
	} else {
		// Linux processing logic
		execPath := o.EngineConfig.ExecPath
		enginePath := o.EngineConfig.EnginePath

		libPath := fmt.Sprintf("%s/ovms/lib", execPath)
		envPath := fmt.Sprintf("%s/ovms/bin", execPath)
		pythonPath := fmt.Sprintf("%s/python", libPath)

		// Detect Linux distribution and version
		distro, version, err := detectLinuxDistribution()
		if err != nil {
			slog.Error("Failed to detect Linux distribution: " + err.Error())
			logger.EngineLogger.Error("[OpenVINO] Failed to detect Linux distribution: " + err.Error())
			return nil
		}

		// 1. Construct shell script commands (ensure all commands execute in the same session)
		shellContent := ""

		// Select download URL based on distribution and version
		switch distro {
		case "ubuntu":
			switch version {
			case "22.04":
				shellContent = fmt.Sprintf(InitShellLinuxUbuntu2204, libPath, envPath, pythonPath, enginePath)
			case "24.04":
				shellContent = fmt.Sprintf(InitShellLinuxUbuntu2404, libPath, envPath, pythonPath, enginePath)
			default:
				slog.Error("Unsupported Ubuntu version: " + version)
				logger.EngineLogger.Error("[OpenVINO] Unsupported Ubuntu version: " + version)
				return nil
			}
		case "rhel", "centos", "rocky", "almalinux":
			shellContent = fmt.Sprintf(InitShellLinuxREHL96, libPath, envPath, pythonPath, enginePath)
		default:
			slog.Error("Unsupported Linux distribution: " + distro)
			logger.EngineLogger.Error("[OpenVINO] Unsupported Linux distribution: " + distro)
			return nil
		}

		logger.EngineLogger.Debug("[OpenVINO] Shell content: " + shellContent)

		// 2. Create temporary shell script file
		tmpScriptFile = filepath.Join(os.TempDir(), "run_install.sh")
		if err := os.WriteFile(tmpScriptFile, []byte(shellContent), 0o755); err != nil {
			slog.Error("Failed to create temp shell script", "error", err)
			logger.EngineLogger.Error("[OpenVINO] Failed to create temp shell script: " + err.Error())
			return fmt.Errorf("failed to create temp shell script: %v", err)
		}

		// 3. Execute shell script file
		cmd = exec.Command("/bin/bash", tmpScriptFile)
		cmd.Dir = enginePath
	}

	defer os.Remove(tmpScriptFile) // Delete temporary file after execution

	// Real-time output of stdout and stderr
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout) // Output to both console and buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr) // Output to both console and buffer

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to run script", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to run script: " + err.Error())
		return fmt.Errorf("failed to run script: %v\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	slog.Info("[Install Engine] openvino model engine install completed")
	logger.EngineLogger.Info("[OpenVINO] OpenVINO Model Server install completed")

	return nil
}

func removeDirWithSkipLockedFiles(dir string) error {
	var firstErr error
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		e := os.Remove(path)
		if e != nil {
			if strings.Contains(e.Error(), "Access is denied") {
				logger.EngineLogger.Warn("[OpenVINO] Skip locked file: " + path)
				return nil
			}
			if firstErr == nil {
				firstErr = e
			}
		}
		return nil
	})
	_ = os.RemoveAll(dir)
	return firstErr
}

func (o *OpenvinoProvider) InitEnv() error {
	// TODO: set env
	return nil
}

type ModelConfig struct {
	Name      string `json:"name"`
	BasePath  string `json:"base_path"`
	GraphPath string `json:"graph_path"`
}

type OpenvinoModelServerConfig struct {
	MediapipeConfigList []ModelConfig `json:"mediapipe_config_list"`
	ModelConfigList     []interface{} `json:"model_config_list"`
}

func (o *OpenvinoProvider) getConfigPath() string {
	return fmt.Sprintf("%s/models/config.json", o.EngineConfig.EnginePath)
}

func (o *OpenvinoProvider) loadConfig() (*OpenvinoModelServerConfig, error) {
	configPath := o.getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("Failed to read config file", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to read config file: " + err.Error())
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config OpenvinoModelServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		slog.Error("Failed to unmarshal config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to unmarshal config: " + err.Error())
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return &config, nil
}

func (o *OpenvinoProvider) saveConfig(config *OpenvinoModelServerConfig) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		slog.Error("Failed to marshal config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to marshal config: " + err.Error())
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	return os.WriteFile(o.getConfigPath(), data, 0o644)
}

func (o *OpenvinoProvider) ListModels(ctx context.Context) (*types.ListResponse, error) {
	config, err := o.loadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to load config: " + err.Error())
		return nil, err
	}

	modelList := make([]types.ListModelResponse, 0)
	for _, model := range config.MediapipeConfigList {
		modelList = append(modelList, types.ListModelResponse{
			Name: model.Name,
		})
	}

	return &types.ListResponse{
		Models: modelList,
	}, nil
}

func (o *OpenvinoProvider) PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error) {
	ctx, cancel := context.WithCancel(ctx)
	modelArray := append(client.ModelClientMap["openvino_"+req.Model], cancel)
	client.ModelClientMap["openvino_"+req.Model] = modelArray
	dataCh := make(chan []byte)
	errCh := make(chan error)
	defer close(dataCh)
	defer close(errCh)
	localModelPath := fmt.Sprintf("%s/models/%s", o.EngineConfig.EnginePath, req.Model)
	if _, err := os.Stat(localModelPath); err != nil {
		_ = os.MkdirAll(localModelPath, 0o755)
	}
	fileReq := &ModelScopeFileReqData{
		ModelName: req.Model,
		Revision:  ModelScopeRevision,
		Recursive: "True",
	}
	modelFiles, err := GetModelFiles(ctx, fileReq)
	if err != nil {
		errCh <- err
		return dataCh, errCh
	}
	if len(modelFiles) == 0 {
		errCh <- errors.New("no model files found")
		return dataCh, errCh
	}
	sort.Slice(modelFiles, func(i, j int) bool {
		return modelFiles[i].Size > modelFiles[j].Size
	})

	newDataCh := make(chan []byte)
	newErrorCh := make(chan error, 1)

	AsyncDownloadFuncParams := &AsyncDownloadModelFileData{
		ModelFiles:     modelFiles,
		ModelName:      req.Model,
		DataCh:         newDataCh,
		ErrCh:          newErrorCh,
		LocalModelPath: localModelPath,
		ModelType:      req.ModelType,
	}
	go AsyncDownloadModelFile(ctx, *AsyncDownloadFuncParams, o)

	return newDataCh, newErrorCh
}

func (o *OpenvinoProvider) DeleteModel(ctx context.Context, req *types.DeleteRequest) error {
	err := o.UnloadModel(ctx, &types.UnloadModelRequest{Models: []string{req.Model}})
	if err != nil {
		slog.Error("Failed to unload model", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to unload model: " + err.Error())
		return err
	}

	modelDir := fmt.Sprintf("%s/models/%s", o.EngineConfig.EnginePath, req.Model)
	if err := os.RemoveAll(modelDir); err != nil {
		slog.Error("Failed to remove model directory", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to remove model directory: " + err.Error())
		return err
	}

	return nil
}

func (o *OpenvinoProvider) addModelToConfig(modelName, modelType string) error {
	config, err := o.loadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to load config: " + err.Error())
		return err
	}

	for _, model := range config.MediapipeConfigList {
		if model.Name == modelName {
			return nil
		}
	}

	newModel := ModelConfig{
		Name: modelName,
		// BasePath:  o.EngineConfig.EnginePath + "/models",
		GraphPath: "graph.pbtxt",
	}
	config.MediapipeConfigList = append(config.MediapipeConfigList, newModel)

	return o.saveConfig(config)
}

func (o *OpenvinoProvider) generateGraphPBTxt(modelName, modelType string) error {
	modelDir := fmt.Sprintf("%s/models/%s", o.EngineConfig.EnginePath, modelName)
	if err := os.MkdirAll(modelDir, 0o750); err != nil {
		slog.Error("Failed to create model directory", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to create model directory: " + err.Error())
		return err
	}

	enginePath := strings.Replace(o.EngineConfig.EnginePath, "\\", "/", -1)

	var template string
	switch modelType {
	case types.ServiceTextToImage:
		template = fmt.Sprintf(GraphPBTxtTextToImage, modelName, enginePath)
	case types.ServiceSpeechToText:
		template = fmt.Sprintf(GraphPBTxtSpeechToText, modelName, enginePath)
	case types.ServiceSpeechToTextWS:
		template = fmt.Sprintf(GraphPBTxtSpeechToText, modelName, enginePath)
	case types.ServiceTextToSpeech:
		template = fmt.Sprintf(GraphPBTxtTextToSpeech, modelName, enginePath)
	default:
		slog.Error("Unsupported model type: " + modelType)
		logger.EngineLogger.Error("[OpenVINO] Unsupported model type: " + modelType)
		return fmt.Errorf("unsupported model type: %s", modelType)
	}

	graphPath := fmt.Sprintf("%s/graph.pbtxt", modelDir)
	return os.WriteFile(graphPath, []byte(template), 0o644)
}

func (o *OpenvinoProvider) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
	ctx, cancel := context.WithCancel(ctx)
	modelArray := append(client.ModelClientMap["openvino_"+req.Model], cancel)
	client.ModelClientMap["openvino_"+req.Model] = modelArray
	localModelPath := fmt.Sprintf("%s/models/%s", o.EngineConfig.EnginePath, req.Model)
	if _, err := os.Stat(localModelPath); err != nil {
		_ = os.MkdirAll(localModelPath, 0o755)
	}

	fileReq := &ModelScopeFileReqData{
		ModelName: req.Model,
		Revision:  ModelScopeRevision,
		Recursive: "True",
	}
	modelFiles, err := GetModelFiles(ctx, fileReq)
	if err != nil {
		return nil, err
	}

	logger.EngineLogger.Debug("[OpenVINO] modelFiles: " + fmt.Sprintf("%+v", modelFiles))

	if len(modelFiles) == 0 {
		return nil, errors.New("no model files found")
	}
	sort.Slice(modelFiles, func(i, j int) bool {
		return modelFiles[i].Size > modelFiles[j].Size
	})

	newDataCh := make(chan []byte)
	newErrorCh := make(chan error, 1)

	AsyncDownloadFuncParams := &AsyncDownloadModelFileData{
		ModelFiles:     modelFiles,
		ModelType:      req.ModelType,
		ModelName:      req.Model,
		DataCh:         newDataCh,
		ErrCh:          newErrorCh,
		LocalModelPath: localModelPath,
	}
	go AsyncDownloadModelFile(ctx, *AsyncDownloadFuncParams, o)

	// Flag to mark if download completed successfully
	downloadDone := false

	for {
		select {
		case data, ok := <-newDataCh:
			if !ok {
				// dataCh closed -> download completed
				if data == nil {
					downloadDone = true
				}
			}
			// data can be used for progress notification
			if fn != nil && data != nil {
				// fn(data) // Progress callback
				fmt.Printf("Progress callback")
			}
		case err, ok := <-newErrorCh:
			if ok && err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		// Download completed and error channel closed
		if downloadDone && len(newErrorCh) == 0 {
			break
		}
	}
	return &types.ProgressResponse{}, nil
}

const (
	GraphPBTxtSpeechToText = `input_stream: "OVMS_PY_TENSOR:audio"
input_stream: "OVMS_PY_TENSOR_PARAMS:params"
output_stream: "OVMS_PY_TENSOR:result"

node {
  name: "%s"
  calculator: "PythonExecutorCalculator"
  input_side_packet: "PYTHON_NODE_RESOURCES:py"

  input_stream: "INPUT:audio"
  input_stream: "PARAMS:params"
  output_stream: "OUTPUT:result"
  node_options: {
    [type.googleapis.com/mediapipe.PythonExecutorCalculatorOptions]: {
      handler_path: "%s/scripts/speech-to-text/whisper.py"
    }
  }
}`

	GraphPBTxtTextToImage = `input_stream: "OVMS_PY_TENSOR:prompt"
input_stream: "OVMS_PY_TENSOR_BATCH:batch"
input_stream: "OVMS_PY_TENSOR_HEIGHT:height"
input_stream: "OVMS_PY_TENSOR_WIDTH:width"
output_stream: "OVMS_PY_TENSOR:image"

node {
  name: "%s"
  calculator: "PythonExecutorCalculator"
  input_side_packet: "PYTHON_NODE_RESOURCES:py"

  input_stream: "INPUT:prompt"
  input_stream: "BATCH:batch"
  input_stream: "HEIGHT:height"
  input_stream: "WIDTH:width"
  output_stream: "OUTPUT:image"
  node_options: {
    [type.googleapis.com/mediapipe.PythonExecutorCalculatorOptions]: {
      handler_path: "%s/scripts/text-to-image/stable_diffusion.py"
    }
  }
}`

	GraphPBTxtTextToSpeech = `input_stream: "OVMS_PY_TENSOR:text"
input_stream: "OVMS_PY_TENSOR_VOICE:voice"
input_stream: "OVMS_PY_TENSOR_PARAMS:params"
output_stream: "OVMS_PY_TENSOR:audio"

node {
  name: "%s"
  calculator: "PythonExecutorCalculator"
  input_side_packet: "PYTHON_NODE_RESOURCES:py"

  input_stream: "INPUT:text"
  input_stream: "VOICE:voice"
  input_stream: "PARAMS:params"
  output_stream: "OUTPUT:audio"
  node_options: {
    [type.googleapis.com/mediapipe.PythonExecutorCalculatorOptions]: {
      handler_path: "%s/scripts/text-to-speech/text-to-speech.py"
    }
  }
}`

	InitShellWin = `@echo on
call "%s\setupvars.bat"
set PATH=%s\python\Scripts;%%PATH%%
python -m pip install -r "%s\scripts\requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/`

	InitShellLinuxUbuntu2204 = `#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo apt -y install libpython3.10
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/`

	InitShellLinuxUbuntu2404 = `#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo apt -y install libpython3.12
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/`
	InitShellLinuxREHL96 = `#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo yum install -y python39-libs
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/`

	StartShellWin = `@echo on
	call "%s\\setupvars.bat"
	set PATH=%s\\python\\Scripts;%%PATH%%
	set HF_HOME=%s\\.cache
	set HF_ENDPOINT=https://hf-mirror.com
	%s --port 9000 --grpc_bind_address 127.0.0.1 --config_path %s\\config.json`
	StartShellLinux = `#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
ovms --port 9000 --grpc_bind_address 127.0.0.1 --config_path %s/config.json`
)

func (o *OpenvinoProvider) checkModelMetadata(modelName string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.EngineLogger.Error("[OpenVINO] Panic caught in ModelMetadata: " + fmt.Sprintf("%v", r))
			err = fmt.Errorf("panic caught in ModelMetadata: %v", r)
			return
		}
	}()

	grpcClient, err := client.NewGRPCClient(o.EngineConfig.Host)
	if err != nil {
		slog.Error("Failed to create GRPC client: %v", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to create GRPC client: " + err.Error())
		return err
	}

	_, err = grpcClient.ModelMetadata(modelName, "")
	if err != nil {
		logger.EngineLogger.Error("[OpenVINO] ModelMetadata failed with error: %v", err)
		return err
	}

	return nil
}

func (o *OpenvinoProvider) GetRunningModels(ctx context.Context) (*types.ListResponse, error) {
	config, err := o.loadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to load config: " + err.Error())
		return nil, err
	}

	modelList := make([]types.ListModelResponse, 0)
	for _, model := range config.MediapipeConfigList {
		modelList = append(modelList, types.ListModelResponse{
			Name: model.Name,
		})
	}

	return &types.ListResponse{
		Models: modelList,
	}, nil
}

func (o *OpenvinoProvider) LoadModel(ctx context.Context, req *types.LoadRequest) error {
	config, err := o.loadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to load config: " + err.Error())
		return err
	}

	for _, model := range config.MediapipeConfigList {
		if model.Name == req.Model {
			return nil
		}
	}

	modelPath := o.EngineConfig.EnginePath + "/models/" + req.Model
	if _, err := os.Stat(modelPath); err != nil {
		logger.EngineLogger.Error("[OpenVINO] Model not found: " + err.Error())
		return err
	}

	if err := o.addModelToConfig(req.Model, ""); err != nil {
		logger.EngineLogger.Error("[OpenVINO] Failed to add model to config: " + err.Error())
		return err
	}

	// Check whether the model has been successfully loaded from the OVMS loading.
	// Add timeout mechanism to avoid infinite waiting
	timeout := 5 * time.Minute
	startTime := time.Now()

	for {
		// Check timeout
		if time.Since(startTime) > timeout {
			logger.EngineLogger.Error("[OpenVINO] Timeout waiting for model to load: " + req.Model)
			return fmt.Errorf("timeout waiting for model %s to load after %v", req.Model, timeout)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			logger.EngineLogger.Warn("[OpenVINO] Context cancelled while waiting for model to load: " + req.Model)
			return ctx.Err()
		default:
		}

		if err := o.checkModelMetadata(req.Model); err == nil {
			logger.EngineLogger.Debug("[OpenVINO] Model " + req.Model + " has been loaded from OVMS")
			break
		}

		logger.EngineLogger.Debug("[OpenVINO] Waiting for model to be loaded from OVMS: " + req.Model)

		// Use interruptible sleep
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	logger.EngineLogger.Debug("[OpenVINO] Model loaded: " + req.Model)

	return nil
}

func (o *OpenvinoProvider) UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error {
	config, err := o.loadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		logger.EngineLogger.Error("[OpenVINO] Failed to load config: " + err.Error())
		return err
	}

	for i, model := range config.MediapipeConfigList {
		for _, reqModel := range req.Models {
			if model.Name == reqModel {
				config.MediapipeConfigList = append(config.MediapipeConfigList[:i], config.MediapipeConfigList[i+1:]...)
				err = o.saveConfig(config)
				if err != nil {
					slog.Error("Failed to save config after deleting model", "error", err)
					logger.EngineLogger.Error("[OpenVINO] Failed to save config after deleting model: " + err.Error())
					return err
				}

				// Check whether the model has been successfully unloaded from the OVMS loading.
				// Add timeout mechanism to avoid infinite waiting
				timeout := 2 * time.Minute
				startTime := time.Now()

				for {
					// Check timeout
					if time.Since(startTime) > timeout {
						logger.EngineLogger.Error("[OpenVINO] Timeout waiting for model to unload: " + reqModel)
						return fmt.Errorf("timeout waiting for model %s to unload after %v", reqModel, timeout)
					}

					// Check context cancellation
					select {
					case <-ctx.Done():
						logger.EngineLogger.Warn("[OpenVINO] Context cancelled while waiting for model to unload: " + reqModel)
						return ctx.Err()
					default:
					}

					if err := o.checkModelMetadata(reqModel); err != nil {
						logger.EngineLogger.Debug("[OpenVINO] Model " + reqModel + " has been unloaded from OVMS")
						break
					}

					logger.EngineLogger.Debug("[OpenVINO] Waiting for model to be unloaded from OVMS: " + reqModel)

					// Use interruptible sleep
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(1 * time.Second):
					}
				}

				logger.EngineLogger.Debug("[OpenVINO] Model unloaded: " + reqModel)
				return nil
			}
		}
	}

	return nil
}

// detectLinuxDistribution detects the Linux distribution and version
func detectLinuxDistribution() (string, string, error) {
	// Try to read /etc/os-release first (standard method)
	if file, err := os.Open("/etc/os-release"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)

		var id, versionID string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "ID=") {
				id = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			} else if strings.HasPrefix(line, "VERSION_ID=") {
				versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			}
		}

		if id != "" {
			return normalizeDistroName(id), versionID, nil
		}
	}

	// Fallback to /etc/lsb-release (Ubuntu/Debian)
	if file, err := os.Open("/etc/lsb-release"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)

		var distroID, release string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "DISTRIB_ID=") {
				distroID = strings.Trim(strings.TrimPrefix(line, "DISTRIB_ID="), `"`)
			} else if strings.HasPrefix(line, "DISTRIB_RELEASE=") {
				release = strings.Trim(strings.TrimPrefix(line, "DISTRIB_RELEASE="), `"`)
			}
		}

		if distroID != "" {
			return normalizeDistroName(distroID), release, nil
		}
	}

	// Fallback to checking specific release files
	releaseFiles := map[string]string{
		"/etc/redhat-release":    "rhel",
		"/etc/centos-release":    "centos",
		"/etc/rocky-release":     "rocky",
		"/etc/almalinux-release": "almalinux",
	}

	for file, distro := range releaseFiles {
		if content, err := os.ReadFile(file); err == nil {
			// Extract version number from release file
			re := regexp.MustCompile(`(\d+)\.(\d+)`)
			matches := re.FindStringSubmatch(string(content))
			if len(matches) >= 2 {
				version := matches[1] + "." + matches[2]
				return distro, version, nil
			}
			return distro, "", nil
		}
	}

	return "", "", fmt.Errorf("unable to detect Linux distribution")
}

// normalizeDistroName normalizes distribution names to standard format
func normalizeDistroName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "ubuntu":
		return "ubuntu"
	case "rhel", "redhat":
		return "rhel"
	case "centos":
		return "centos"
	case "rocky":
		return "rocky"
	case "almalinux", "alma":
		return "almalinux"
	default:
		return name
	}
}

func (o *OpenvinoProvider) UpgradeEngine() error {
	// Get current engine version
	var resp types.EngineVersionResponse
	verResp, err := o.GetVersion(context.Background(), &resp)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] GetVersion failed: " + err.Error())
		return fmt.Errorf("get current engine version failed: %v", err)
	}
	currentVersion := verResp.Version
	minVersion := OpenvinoMinVersion
	slog.Info("Openvino version check", "current_version", currentVersion, "min_version", minVersion)

	// Check if upgrade is needed
	if VersionCompare(currentVersion, minVersion) >= 0 {
		logger.EngineLogger.Info("[OpenVINO] Current version is up-to-date, no upgrade needed.")
		return nil
	}

	logger.EngineLogger.Info(fmt.Sprintf("[OpenVINO] Upgrading engine from %s to %s", currentVersion, minVersion))

	// Stop the engine and stop keeping alive
	if err := o.StopEngine(); err != nil {
		logger.EngineLogger.Error("[OpenVINO] StopEngine failed: " + err.Error())
		return fmt.Errorf("stop engine failed: %v", err)
	}
	o.SetOperateStatus(0)

	// Install new version
	if err := o.InstallEngine(true); err != nil {
		logger.EngineLogger.Error("[OpenVINO] InstallEngine failed: " + err.Error())
		return fmt.Errorf("install engine failed: %v", err)
	}
	defer o.SetOperateStatus(1) // keep alive

	logger.EngineLogger.Info("[OpenVINO] Engine upgrade completed.")
	return nil
}

var OpenvinoOperateStatus = 1

func (o *OpenvinoProvider) GetOperateStatus() int {
	return OpenvinoOperateStatus
}

func (o *OpenvinoProvider) SetOperateStatus(status int) {
	OpenvinoOperateStatus = status
	slog.Info("Openvino operate status set to", "status", OpenvinoOperateStatus)
}
