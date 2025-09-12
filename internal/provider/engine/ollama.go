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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/version"
)

const (
	// Default configuration
	DefaultPort = "16677"
	DefaultHost = constants.DefaultHost + ":" + DefaultPort

	// ipex-llm-ollama related
	IpexLlamaDir     = "ipex-llm-ollama"
	OllamaBatchFile  = "ollama-serve.bat"
	OllamaStartShell = "start-ollama.sh"

	// Windows download URLs
	WindowsAllGPUURL   = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ollama-windows-amd64-all.zip"
	WindowsNvidiaURL   = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ollama-windows-amd64.zip"
	WindowsAMDURL      = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ollama-windows-amd64-rocm.zip"
	WindowsIntelArcURL = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ipex-llm-ollama.zip"
	WindowsBaseURL     = constants.BaseDownloadURL + constants.UrlDirPathWindows + "/" + version.AOGVersion + "/ollama-windows-amd64-base.zip"

	// Linux download URLs
	LinuxNvidiaURL   = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ollama-cuda-linux-amd64.tgz"
	LinuxAMDURL      = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ollama-rocm-linux-amd64.tgz"
	LinuxBaseURL     = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ollama-linux-amd64.tgz"
	LinuxARMURL      = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ollama-cuda-linux-arm64.tgz"
	LinuxARMBaseURL  = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ollama-linux-arm64.tgz"
	LinuxIntelArcURL = constants.BaseDownloadURL + constants.UrlDirPathLinux + "/" + version.AOGVersion + "/ipex-llm-ollama.zip"

	// macOS download URLs
	MacOSIntelURL = constants.BaseDownloadURL + constants.UrlDirPathMacOS + "/Ollama-darwin.zip"

	// Archive commands
	TarCommand     = "tar"
	TarExtractFlag = "-xf"
	TarDestFlag    = "-C"
	UnzipCommand   = "unzip"
	UnzipDestFlag  = "-d"
	MoveCommand    = "mv"

	OllamaStartShellWin   = `%s\\ollama.exe serve`
	OllamaStartShellLinux = `%s/ollama serve`
	OllamaStartShellMac   = `%s/ollama serve`

	OllamaModelDirLinux = "/var/lib/aog/engine/ollama/models"

	// Required version
	OllamaMinVersion = "0.7.1"
)

type OllamaProvider struct {
	EngineConfig   *types.EngineRecommendConfig
	processManager *process.EngineProcessManager
}

func NewOllamaProvider(config *types.EngineRecommendConfig) *OllamaProvider {
	if config != nil {
		provider := &OllamaProvider{
			EngineConfig: config,
		}
		provider.processManager = process.NewEngineProcessManager("ollama", config)
		return provider
	}

	AOGDir, err := utils.GetAOGDataDir()
	if err != nil {
		slog.Error("Get AOG data dir failed", "error", err)
		logger.EngineLogger.Error("[Ollama] Get AOG data dir failed: " + err.Error())
		return nil
	}

	downloadPath := fmt.Sprintf("%s/%s/%s", AOGDir, "engine", "ollama")
	if _, err := os.Stat(downloadPath); os.IsNotExist(err) {
		err := os.MkdirAll(downloadPath, 0o750)
		if err != nil {
			logger.EngineLogger.Error("[Ollama] Create download dir failed: " + err.Error())
			return nil
		}
	}

	ollamaProvider := new(OllamaProvider)
	ollamaProvider.EngineConfig = ollamaProvider.GetConfig()
	ollamaProvider.processManager = process.NewEngineProcessManager("ollama", ollamaProvider.EngineConfig)

	return ollamaProvider
}

func (o *OllamaProvider) GetDefaultClient() *client.Client {
	// default host
	host := DefaultHost
	if o.EngineConfig.Host != "" {
		host = o.EngineConfig.Host
	}

	// default scheme
	scheme := types.ProtocolHTTP
	if o.EngineConfig.Scheme == types.ProtocolHTTPS {
		scheme = types.ProtocolHTTPS
	}

	return client.NewClient(&url.URL{
		Scheme: scheme,
		Host:   host,
	}, http.DefaultClient)
}

func (o *OllamaProvider) StartEngine(mode string) error {
	// Always use new process manager
	if o.processManager == nil {
		o.processManager = process.NewEngineProcessManager("ollama", o.EngineConfig)
	}

	if err := o.processManager.StartEngine(mode, o.HealthCheck); err != nil {
		// If engine not found, this is expected behavior - just log and return success
		if strings.Contains(err.Error(), "executable not found") {
			logger.EngineLogger.Info("[Ollama] Engine not installed, skipping startup")
			return nil
		}
		// For other errors, return the error directly
		return fmt.Errorf("failed to start ollama engine: %v", err)
	}

	return nil
}

func (o *OllamaProvider) StopEngine() error {
	ctx := context.Background()

	// First try to unload running models gracefully
	runningModels, err := o.GetRunningModels(ctx)
	if err == nil {
		runningModelList := []string{}
		for _, model := range runningModels.Models {
			runningModelList = append(runningModelList, model.Name)
		}
		if len(runningModelList) > 0 {
			err = o.UnloadModel(ctx, &types.UnloadModelRequest{Models: runningModelList})
			if err != nil {
				logger.EngineLogger.Warn("[Ollama] Failed to unload models: " + err.Error())
				// Continue with engine shutdown even if model unload fails
			}
		}
	}

	// Use new process manager if available
	if o.processManager != nil {
		return o.processManager.StopEngine()
	}

	return nil
}

// SetProcessManager sets the process manager for the provider
func (o *OllamaProvider) SetProcessManager(pm *process.EngineProcessManager) {
	o.processManager = pm
}

// stopEngineOldWay fallback to old PID-based stopping method
func (o *OllamaProvider) stopEngineOldWay() error {
	rootPath, err := utils.GetAOGDataDir()
	if err != nil {
		logger.EngineLogger.Error("[Ollama] failed get aog dir: " + err.Error())
		return fmt.Errorf("failed get aog dir: %v", err)
	}
	pidFile := fmt.Sprintf("%s/ollama.pid", rootPath)
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		logger.EngineLogger.Info("[Ollama] Stop ollama not found pidfile: " + pidFile)
		return nil
	}

	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] failed to read pid file: " + err.Error())
		return fmt.Errorf("failed to read pid file: %v", err)
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		logger.EngineLogger.Error("[Ollama] invalid pid format: " + err.Error())
		return fmt.Errorf("invalid pid format: %v", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] failed to find process: " + err.Error())
		return fmt.Errorf("failed to find process: %v", err)
	}

	if err := proc.Kill(); err != nil {
		logger.EngineLogger.Error("[Ollama] failed to kill process: " + err.Error())
		return fmt.Errorf("failed to kill process: %v", err)
	}

	if err := os.Remove(pidFile); err != nil {
		logger.EngineLogger.Error("[Ollama] failed to remove pid file: " + err.Error())
		return fmt.Errorf("failed to remove pid file: %v", err)
	}

	return nil
}

func (o *OllamaProvider) GetConfig() *types.EngineRecommendConfig {
	if o.EngineConfig != nil {
		return o.EngineConfig
	}

	userDir, err := os.UserHomeDir()
	if err != nil {
		logger.EngineLogger.Error("[Ollama] Get user home dir failed", "error", err)
		return nil
	}

	downloadPath, _ := utils.GetDownloadDir()
	if _, err := os.Stat(downloadPath); os.IsNotExist(err) {
		err := os.MkdirAll(downloadPath, 0o755)
		if err != nil {
			logger.EngineLogger.Error("[Ollama] Create download dir failed", "error", err)
			return nil
		}
	}
	dataDir, err := utils.GetAOGDataDir()
	if err != nil {
		slog.Error("Get AOG data dir failed", "error", err)
		return nil
	}

	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return nil
	}

	execFile := ""
	execPath := ""
	downloadUrl := ""
	enginePath := fmt.Sprintf("%s/%s", dataDir, "engine/ollama")
	switch runtime.GOOS {
	case "windows":
		execFile = "ollama.exe"
		execPath = fmt.Sprintf("%s/%s", userDir, "ollama")

		switch utils.DetectGpuModel() {
		case types.GPUTypeNvidia + "," + types.GPUTypeAmd:
			downloadUrl = WindowsAllGPUURL
		case types.GPUTypeNvidia:
			downloadUrl = WindowsNvidiaURL
		case types.GPUTypeAmd:
			downloadUrl = WindowsAMDURL
		case types.GPUTypeIntelArc:
			execPath = fmt.Sprintf("%s/%s", userDir, IpexLlamaDir)
			downloadUrl = WindowsIntelArcURL
		default:
			downloadUrl = WindowsBaseURL
		}

	case "linux":
		execFile = "ollama"
		execPath = "/opt/aog/engine/ollama"
		enginePath = filepath.Join(dataDir, "engine/ollama")

		// Determine GPU type and select appropriate download URL
		switch utils.DetectGpuModel() {
		case types.GPUTypeNvidia:
			if arch == "arm64" {
				downloadUrl = LinuxARMURL // CUDA for ARM64
			} else {
				downloadUrl = LinuxNvidiaURL // CUDA for AMD64
			}
		case types.GPUTypeAmd:
			downloadUrl = LinuxAMDURL // ROCm (AMD64 only)
		case types.GPUTypeNone:
			if arch == "arm64" {
				downloadUrl = LinuxARMBaseURL // Base for ARM64
			} else {
				downloadUrl = LinuxBaseURL // Base for AMD64
			}
		}
	case "darwin":
		execFile = "ollama"
		execPath = fmt.Sprintf("/%s/%s/%s/%s", "Applications", "Ollama.app", "Contents", "Resources")
		downloadUrl = MacOSIntelURL
	default:
		return nil
	}

	return &types.EngineRecommendConfig{
		Host:           DefaultHost,
		Origin:         constants.DefaultHost,
		Scheme:         types.ProtocolHTTP,
		EnginePath:     enginePath,
		RecommendModel: constants.RecommendModel,
		DownloadUrl:    downloadUrl,
		DownloadPath:   downloadPath,
		ExecFile:       execFile,
		ExecPath:       execPath,
	}
}

func (o *OllamaProvider) HealthCheck() error {
	c := o.GetDefaultClient()
	if err := c.Do(context.Background(), http.MethodHead, "/", nil, nil); err != nil {
		logger.EngineLogger.Error("[Ollama] Health check failed: " + err.Error())
		return err
	}
	logger.EngineLogger.Info("[Ollama] Ollama server health")

	return nil
}

func (o *OllamaProvider) GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error) {
	c := o.GetDefaultClient()
	if err := c.Do(ctx, http.MethodGet, "/api/version", nil, resp); err != nil {
		slog.Error("Get engine version : " + err.Error())
		return nil, err
	}
	return resp, nil
}

// CheckEngine checks if Ollama engine is installed
func (o *OllamaProvider) CheckEngine() bool {
	if o.EngineConfig == nil {
		return false
	}

	// Choose different check logic based on the operating system
	switch runtime.GOOS {
	case "windows":
		return o.checkEngineWindows()
	case "linux":
		return o.checkEngineLinux()
	case "darwin":
		return o.checkEngineMacOS()
	default:
		return false
	}
}

// checkEngineWindows Windows platform check logic
func (o *OllamaProvider) checkEngineWindows() bool {
	isIntel := utils.IpexOllamaSupportGPUStatus()

	if isIntel {
		// Check Intel chip's ipex-llm batch file
		batchFile := filepath.Join(o.EngineConfig.ExecPath, IpexLlamaDir, OllamaBatchFile)
		if _, err := os.Stat(batchFile); err != nil {
			return false
		}
		return true
	} else {
		// Check non-Intel chip's standard ollama executable
		execFile := filepath.Join(o.EngineConfig.ExecPath, o.EngineConfig.ExecFile)
		if _, err := os.Stat(execFile); err != nil {
			return false
		}
		// Try executing -h command to verify executability
		cmd := exec.Command(execFile, "-h")
		err := cmd.Run()
		return err == nil
	}
}

// checkEngineLinux Linux platform check logic
func (o *OllamaProvider) checkEngineLinux() bool {
	isIntel := utils.IpexOllamaSupportGPUStatus()

	if isIntel {
		// Check Intel chip's ipex-llm script file
		scriptFile := filepath.Join(o.EngineConfig.ExecPath, IpexLlamaDir, OllamaStartShell)
		if _, err := os.Stat(scriptFile); err != nil {
			return false
		}
		// Check if script file has execute permissions
		fileInfo, err := os.Stat(scriptFile)
		if err != nil {
			return false
		}
		return fileInfo.Mode()&0o111 != 0 // Check execute permissions
	} else {
		// Check non-Intel chip's standard ollama executable
		execFile := filepath.Join(o.EngineConfig.ExecPath, "ollama/bin", o.EngineConfig.ExecFile)
		if _, err := os.Stat(execFile); err != nil {
			return false
		}
		// Try executing -h command to verify executability
		cmd := exec.Command(execFile, "-h")
		err := cmd.Run()
		return err == nil
	}
}

// checkEngineMacOS macOS platform check logic
func (o *OllamaProvider) checkEngineMacOS() bool {
	var execFile string

	// Prefer configured path, use system installed Ollama if not found
	if o.EngineConfig.ExecPath != "" && o.EngineConfig.ExecFile != "" {
		execFile = filepath.Join(o.EngineConfig.ExecPath, o.EngineConfig.ExecFile)
		if _, err := os.Stat(execFile); err == nil {
			// Verify executability
			cmd := exec.Command(execFile, "-h")
			return cmd.Run() == nil
		}
		// Configured path doesn't exist, try system path
		execFile = "/Applications/Ollama.app/Contents/Resources/ollama"
	} else {
		// Use system installed Ollama
		execFile = "/Applications/Ollama.app/Contents/Resources/ollama"
	}

	if _, err := os.Stat(execFile); err != nil {
		return false
	}
	// Verify executability
	cmd := exec.Command(execFile, "-h")
	return cmd.Run() == nil
}

func (o *OllamaProvider) InstallEngine(cover bool) error {
	file, err := utils.DownloadFile(o.EngineConfig.DownloadUrl, o.EngineConfig.DownloadPath, cover)
	if err != nil {
		return fmt.Errorf("failed to download ollama: %v, url: %v", err, o.EngineConfig.DownloadUrl)
	}

	slog.Info("[Install Engine] start install...")

	// Call corresponding installation method based on operating system
	switch runtime.GOOS {
	case "darwin":
		err = o.installEngineMacOS(file, cover)
	case "windows":
		err = o.installEngineWindows(file, cover)
	case "linux":
		err = o.installEngineLinux(file, cover)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return fmt.Errorf("failed to install engine on %s: %v", runtime.GOOS, err)
	}

	slog.Info("[Install Engine] model engine install completed")
	return nil
}

// installEngineMacOS macOS platform installation logic
func (o *OllamaProvider) installEngineMacOS(file string, cover bool) error {
	logger.EngineLogger.Info("[Ollama] Installing engine on macOS platform")
	appPath := filepath.Join(o.EngineConfig.DownloadPath, "Ollama.app")
	applicationPath := filepath.Join("/Applications/", "Ollama.app")

	// Delete Ollama.app and /Applications/Ollama.app first when cover install
	if cover {
		if _, err := os.Stat(appPath); err == nil {
			os.RemoveAll(appPath)
		}
		if _, err := os.Stat(applicationPath); err == nil {
			os.RemoveAll(applicationPath)
		}
	}

	files, err := os.ReadDir(o.EngineConfig.DownloadPath)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] Failed to read directory: " + err.Error())
		slog.Error("[Install Engine] read dir failed: ", o.EngineConfig.DownloadPath)
		return err
	}
	for _, f := range files {
		if f.IsDir() && f.Name() == "__MACOSX" {
			fPath := filepath.Join(o.EngineConfig.DownloadPath, f.Name())
			os.RemoveAll(fPath)
		}
	}
	if _, err = os.Stat(appPath); os.IsNotExist(err) {
		unzipCmd := exec.Command(UnzipCommand, file, UnzipDestFlag, o.EngineConfig.DownloadPath)
		if err := unzipCmd.Run(); err != nil {
			logger.EngineLogger.Error("[Ollama] Failed to unzip file: " + err.Error())
			return fmt.Errorf("failed to unzip file: %v", err)
		}
	}
	if _, err = os.Stat(applicationPath); os.IsNotExist(err) {
		mvCmd := exec.Command(MoveCommand, appPath, "/Applications/")
		if err := mvCmd.Run(); err != nil {
			logger.EngineLogger.Error("[Ollama] Failed to move ollama to Applications: " + err.Error())
			return fmt.Errorf("failed to move ollama to Applications: %v", err)
		}
	}
	return nil
}

// installEngineWindows Windows platform installation logic
func (o *OllamaProvider) installEngineWindows(file string, cover bool) error {
	logger.EngineLogger.Info("[Ollama] Installing engine on Windows platform")

	// Delete target directory first when cover install
	if cover {
		if _, err := os.Stat(o.EngineConfig.ExecPath); err == nil {
			_ = os.RemoveAll(o.EngineConfig.ExecPath)
		}
	}

	if _, err := os.Stat(o.EngineConfig.ExecPath); os.IsNotExist(err) {
		if err := os.MkdirAll(o.EngineConfig.ExecPath, 0o755); err != nil {
			return fmt.Errorf("failed to create exec directory: %v", err)
		}

		// Use unified extraction method
		if err := utils.UnzipFile(file, o.EngineConfig.ExecPath); err != nil {
			return fmt.Errorf("failed to extract ollama: %v", err)
		}
		logger.EngineLogger.Info("[Ollama] ollama installed to: " + o.EngineConfig.ExecPath)
	}

	return nil
}

// installEngineLinux Linux platform installation logic
func (o *OllamaProvider) installEngineLinux(file string, cover bool) error {
	logger.EngineLogger.Info("[Ollama] Installing engine on Linux platform")
	targetPath := ""

	if utils.IpexOllamaSupportGPUStatus() {
		logger.EngineLogger.Info("[Ollama] Intel GPU detected, installing ipex-llm-ollama")
		// Intel GPU: extract to ipex-llm-ollama folder in user directory
		userDir, err := os.UserHomeDir()
		if err != nil {
			slog.Error("Get user home dir failed", "error", err)
			return fmt.Errorf("failed to get user home dir: %v", err)
		}
		targetPath = filepath.Join(userDir, IpexLlamaDir)
	} else {
		logger.EngineLogger.Info("[Ollama] Non-Intel GPU, installing standard ollama")
		// Non-Intel GPU: extract to configured execution path
		targetPath = o.EngineConfig.ExecPath
	}

	// Delete target directory first when cover install
	if cover {
		if _, err := os.Stat(targetPath); err == nil {
			_ = os.RemoveAll(targetPath)
		}
	}

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		if err := os.MkdirAll(targetPath, 0o755); err != nil {
			return fmt.Errorf("failed to create ipex directory: %v", err)
		}

		// Use unified extraction method
		if err := utils.UnzipFile(file, targetPath); err != nil {
			return fmt.Errorf("failed to extract ollama: %v", err)
		}

		logger.EngineLogger.Info("[Ollama] ollama installed to: " + targetPath)
	}

	return nil
}

func (o *OllamaProvider) InitEnv() error {
	err := os.Setenv("OLLAMA_HOST", o.EngineConfig.Host)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] failed to set OLLAMA_HOST: " + err.Error())
		return fmt.Errorf("failed to set OLLAMA_HOST: %w", err)
	}

	err = os.Setenv("OLLAMA_ORIGIN", o.EngineConfig.Origin)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] failed to set OLLAMA_ORIGIN: " + err.Error())
		return fmt.Errorf("failed to set OLLAMA_ORIGIN: %w", err)
	}
	if runtime.GOOS == "linux" {
		err = os.Setenv("OLLAMA_MODELS", OllamaModelDirLinux)
		if err != nil {
			logger.EngineLogger.Error("[Ollama] failed to set OLLAMA_MODELS: " + err.Error())
			return fmt.Errorf("failed to set OLLAMA_MODELS: %w", err)
		}
	}
	return nil
}

func (o *OllamaProvider) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
	logger.EngineLogger.Info("[Ollama] Pull model: " + req.Name)

	c := o.GetDefaultClient()
	ctx, cancel := context.WithCancel(ctx)
	modelArray := append(client.ModelClientMap[req.Model], cancel)
	client.ModelClientMap[req.Model] = modelArray

	var resp types.ProgressResponse
	if err := c.Do(ctx, http.MethodPost, "/api/pull", req, &resp); err != nil {
		logger.EngineLogger.Error("[Ollama] Pull model failed : ", err)
		return &resp, err
	}
	logger.EngineLogger.Info("[Ollama] Pull model success: " + req.Name)

	return &resp, nil
}

func (o *OllamaProvider) PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error) {
	logger.EngineLogger.Info("[Ollama] Pull model: " + req.Name + " , mode: stream")

	c := o.GetDefaultClient()
	ctx, cancel := context.WithCancel(ctx)
	modelArray := append(client.ModelClientMap[req.Model], cancel)
	client.ModelClientMap[req.Model] = modelArray
	reqHeader := make(map[string]string)
	reqHeader["Content-Type"] = "application/json"
	reqHeader["Accept"] = "application/json"
	dataCh, errCh := c.StreamResponse(ctx, http.MethodPost, "/api/pull", req, reqHeader)
	logger.EngineLogger.Info("[Ollama] Pull model success: " + req.Name + " , mode: stream")

	return dataCh, errCh
}

func (o *OllamaProvider) DeleteModel(ctx context.Context, req *types.DeleteRequest) error {
	logger.EngineLogger.Info("[Ollama] Delete model: " + req.Model)

	c := o.GetDefaultClient()
	if err := c.Do(ctx, http.MethodDelete, "/api/delete", req, nil); err != nil {
		logger.EngineLogger.Error("[Ollama] Delete model failed : " + err.Error())
		return err
	}
	logger.EngineLogger.Info("[Ollama] Delete model success: " + req.Model)

	return nil
}

func (o *OllamaProvider) ListModels(ctx context.Context) (*types.ListResponse, error) {
	c := o.GetDefaultClient()
	var lr types.ListResponse
	if err := c.Do(ctx, http.MethodGet, "/api/tags", nil, &lr); err != nil {
		logger.EngineLogger.Error("[Ollama] Get model list failed :" + err.Error())
		return nil, err
	}

	return &lr, nil
}

func (o *OllamaProvider) GetRunningModels(ctx context.Context) (*types.ListResponse, error) {
	c := o.GetDefaultClient()
	var lr types.ListResponse
	if err := c.Do(ctx, http.MethodGet, "/api/ps", nil, &lr); err != nil {
		logger.EngineLogger.Error("[Ollama] Get run model list failed :" + err.Error())
		return nil, err
	}
	return &lr, nil
}

func (o *OllamaProvider) UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error {
	c := o.GetDefaultClient()
	for _, model := range req.Models {
		reqBody := &types.OllamaUnloadModelRequest{
			Model:     model,
			KeepAlive: 0,
		}
		if err := c.Do(ctx, http.MethodPost, "/api/generate", reqBody, nil); err != nil {
			logger.EngineLogger.Error("[Ollama] Unload model failed: " + model + " , error: " + err.Error())
			return err
		}
		logger.EngineLogger.Info("[Ollama] Unload model success: " + model)
	}
	return nil
}

func (o *OllamaProvider) LoadModel(ctx context.Context, req *types.LoadRequest) error {
	// Since ollama automatically loads on request, loading by the agent would make one more API request, which would be slower, so ignore it here
	// c := o.GetDefaultClient()
	// lr := &types.OllamaLoadModelRequest{
	// 	Model: req.Model,
	// }
	// if err := c.Do(ctx, http.MethodPost, "/api/generate", lr, nil); err != nil {
	// 	logger.EngineLogger.Error("[Ollama] Load model failed: " + req.Model + " , error: " + err.Error())
	// 	return err
	// }
	return nil
}

func VersionCompare(v1, v2 string) int {
	s1 := strings.Split(v1, ".")
	s2 := strings.Split(v2, ".")
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(s1) {
			n1, _ = strconv.Atoi(s1[i])
		}
		if i < len(s2) {
			n2, _ = strconv.Atoi(s2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

func (o *OllamaProvider) UpgradeEngine() error {
	// Get current engine version
	var resp types.EngineVersionResponse
	verResp, err := o.GetVersion(context.Background(), &resp)
	if err != nil {
		logger.EngineLogger.Error("[Ollama] GetVersion failed: " + err.Error())
		return fmt.Errorf("get current engine version failed: %v", err)
	}
	currentVersion := verResp.Version
	minVersion := OllamaMinVersion
	slog.Info("ollama version check", "current_version", currentVersion, "min_version", minVersion)

	// Check if upgrade is needed
	if VersionCompare(currentVersion, minVersion) >= 0 {
		logger.EngineLogger.Info("[Ollama] Current version is up-to-date, no upgrade needed.")
		return nil
	}

	logger.EngineLogger.Info(fmt.Sprintf("[Ollama] Upgrading engine from %s to %s", currentVersion, minVersion))

	// Stop the engine and stop keeping alive
	if err := o.StopEngine(); err != nil {
		logger.EngineLogger.Error("[Ollama] StopEngine failed: " + err.Error())
		return fmt.Errorf("stop engine failed: %v", err)
	}
	o.SetOperateStatus(0)

	// Install new version
	if err := o.InstallEngine(true); err != nil {
		logger.EngineLogger.Error("[Ollama] InstallEngine failed: " + err.Error())
		return fmt.Errorf("install engine failed: %v", err)
	}
	defer o.SetOperateStatus(1) // keep alive

	logger.EngineLogger.Info("[Ollama] Engine upgrade completed.")
	return nil
}

var OllamaOperateStatus = 1

func (o *OllamaProvider) GetOperateStatus() int {
	return OllamaOperateStatus
}

func (o *OllamaProvider) SetOperateStatus(status int) {
	OllamaOperateStatus = status
	logger.EngineLogger.Info("Ollama operate status set to", "status", OllamaOperateStatus)
}
