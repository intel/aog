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

package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdktypes "github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/config"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/grpc_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ModelScope API constants
const (
	ModelScopeAPIBase              = "https://modelscope.cn/api/v1"
	ModelScopeDownloadBase         = "https://modelscope.cn/models"
	ModelScopeModelDownloadReqPath = "%s/%s/repo?Revision=%s&FilePath=%s"
)

// ModelInfo represents model metadata
type ModelInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Downloaded bool   `json:"downloaded"`
	Path       string `json:"path"`
	ConfigPath string `json:"config_path"`
	Loaded     bool   `json:"loaded"`
}

// ModelScopeFile represents a file in ModelScope (matches API response)
type ModelScopeFile struct {
	Name     string `json:"Name"`
	Path     string `json:"Path"`
	Digest   string `json:"Sha256"`
	Size     int64  `json:"Size"`
	IsLFS    bool   `json:"IsLFS"`
	Revision string `json:"Revision"`
	Type     string `json:"Type"`
}

// ModelScopeFileRespData API response structure
type ModelScopeFileRespData struct {
	Code int                `json:"Code"`
	Data ModelScopeFileData `json:"Data"`
}

// ModelScopeFileData file data
type ModelScopeFileData struct {
	Files []ModelScopeFile `json:"Files"`
}

// AsyncDownloadModelFileData async download data structure
type AsyncDownloadModelFileData struct {
	ModelName      string
	ModelType      string
	DataCh         chan []byte
	ErrCh          chan error
	ModelFiles     []ModelScopeFile
	LocalModelPath string
}

// PullModel downloads a model from ModelScope
func (p *OvmsProvider) PullModel(ctx context.Context, req *sdktypes.PullModelRequest, fn sdktypes.PullProgressFunc) (*sdktypes.ProgressResponse, error) {
	p.LogInfo(fmt.Sprintf("PullModel called: %s", req.Model))

	if req.Model == "" {
		return nil, fmt.Errorf("model name is required")
	}

	modelName := req.Model
	localModelPath := filepath.Join(p.config.EngineDir, "models", modelName)

	// Create model directory
	if err := os.MkdirAll(localModelPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create model directory: %w", err)
	}

	// Call real ModelScope API to get file list
	modelFiles, err := p.getModelFilesFromAPI(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to get model files: %w", err)
	}

	if len(modelFiles) == 0 {
		return nil, fmt.Errorf("no model files found")
	}

	// Sort by size (large files first)
	sort.Slice(modelFiles, func(i, j int) bool {
		return modelFiles[i].Size > modelFiles[j].Size
	})

	// Create async download channels
	dataCh := make(chan []byte)
	errCh := make(chan error, 1)

	downloadData := AsyncDownloadModelFileData{
		ModelName:      modelName,
		ModelType:      req.ModelType,
		DataCh:         dataCh,
		ErrCh:          errCh,
		ModelFiles:     modelFiles,
		LocalModelPath: localModelPath,
	}

	// Start async download
	go p.asyncDownloadModelFile(ctx, downloadData)

	// Handle progress and errors
	downloadDone := false
	for {
		select {
		case data, ok := <-dataCh:
			if !ok {
				downloadDone = true
			} else if fn != nil && data != nil {
				// Call progress callback
				var progress sdktypes.ProgressResponse
				if err := json.Unmarshal(data, &progress); err == nil {
					fn(progress)
				}
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		if downloadDone && len(errCh) == 0 {
			break
		}
	}

	// Add model to OVMS config
	if err := p.addModelToConfig(modelName, localModelPath); err != nil {
		p.LogError("Failed to add model to config", err)
	}

	return &sdktypes.ProgressResponse{Status: "success"}, nil
}

// PullModelStream downloads a model with streaming progress
func (p *OvmsProvider) PullModelStream(ctx context.Context, req *sdktypes.PullModelRequest) (chan []byte, chan error) {
	dataCh := make(chan []byte, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)

		// Use PullModel with progress callback
		progressFn := func(resp sdktypes.ProgressResponse) error {
			if data, err := json.Marshal(resp); err == nil {
				dataCh <- data
			}
			return nil
		}

		_, err := p.PullModel(ctx, req, progressFn)
		if err != nil {
			errCh <- err
		}
	}()

	return dataCh, errCh
}

// DeleteModel removes a model
func (p *OvmsProvider) DeleteModel(ctx context.Context, req *sdktypes.DeleteRequest) error {
	p.LogInfo(fmt.Sprintf("DeleteModel called: %s", req.Model))

	modelName := req.Model
	modelPath := filepath.Join(p.config.EngineDir, "models", modelName)

	// Remove model directory
	if err := os.RemoveAll(modelPath); err != nil {
		return fmt.Errorf("failed to delete model directory: %w", err)
	}

	// Remove model from OVMS config
	if err := p.removeModelFromConfig(modelName); err != nil {
		p.LogError("Failed to remove model from config", err)
	}

	p.LogInfo(fmt.Sprintf("Model deleted: %s", modelName))
	return nil
}

// ListModels lists all available models
func (p *OvmsProvider) ListModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	// Read from config.json instead of file system
	config, err := p.loadConfig()
	if err != nil {
		p.LogError("Failed to load config", err)
		return &sdktypes.ListResponse{Models: []sdktypes.ModelInfo{}}, nil
	}

	var models []sdktypes.ModelInfo
	for _, m := range config.MediapipeConfigList {
		models = append(models, sdktypes.ModelInfo{
			Name: m.Name,
		})
	}

	return &sdktypes.ListResponse{Models: models}, nil
}

// GetRunningModels returns currently loaded models
func (p *OvmsProvider) GetRunningModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	// Read from config.json
	config, err := p.loadConfig()
	if err != nil {
		p.LogError("Failed to load config", err)
		return &sdktypes.ListResponse{Models: []sdktypes.ModelInfo{}}, nil
	}

	var models []sdktypes.ModelInfo
	for _, m := range config.MediapipeConfigList {
		models = append(models, sdktypes.ModelInfo{
			Name: m.Name,
		})
	}

	return &sdktypes.ListResponse{Models: models}, nil
}

// LoadModel loads a model into OVMS
func (p *OvmsProvider) LoadModel(ctx context.Context, req *sdktypes.LoadRequest) error {
	p.LogInfo(fmt.Sprintf("LoadModel called: %s", req.Model))

	modelName := req.Model
	modelPath := filepath.Join(p.config.EngineDir, "models", modelName)

	// Check if model is already loaded
	config, err := p.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	for _, model := range config.MediapipeConfigList {
		if model.Name == modelName {
			// Model is in config, check if it's actually loaded in OVMS
			if err := p.checkModelMetadata(modelName); err == nil {
				p.LogInfo(fmt.Sprintf("Model already loaded: %s", modelName))
				return nil
			}
			// Model is in config but not loaded yet, wait for it
			break
		}
	}

	// Verify model exists on disk
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model not found: %s", modelName)
	}

	// Update OVMS config to load the model
	if err := p.addModelToConfig(modelName, modelPath); err != nil {
		return fmt.Errorf("failed to add model to config: %w", err)
	}

	// Wait for OVMS to load the model (with timeout)
	timeout := 5 * time.Minute
	startTime := time.Now()

	p.LogInfo(fmt.Sprintf("Waiting for model to be loaded by OVMS: %s", modelName))

	for {
		// Check timeout
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for model %s to load after %v", modelName, timeout)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			p.LogInfo(fmt.Sprintf("Context cancelled while waiting for model to load: %s", modelName))
			return ctx.Err()
		default:
		}

		// Check if model is loaded via gRPC ModelMetadata
		if err := p.checkModelMetadata(modelName); err == nil {
			p.LogInfo(fmt.Sprintf("Model successfully loaded by OVMS: %s", modelName))
			break
		}

		// Use interruptible sleep
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	p.LogInfo(fmt.Sprintf("Model loaded: %s", modelName))
	return nil
}

// UnloadModel unloads a model from OVMS
func (p *OvmsProvider) UnloadModel(ctx context.Context, req *sdktypes.UnloadModelRequest) error {
	p.LogInfo(fmt.Sprintf("UnloadModel called: %v", req.Models))

	for _, modelName := range req.Models {
		if err := p.removeModelFromConfig(modelName); err != nil {
			p.LogError("Failed to remove model from config", err)
		}
	}

	p.LogInfo("Models unloaded")
	return nil
}

// Helper functions

// getModelFilesFromAPI calls ModelScope API to get file list
func (p *OvmsProvider) getModelFilesFromAPI(ctx context.Context, modelName string) ([]ModelScopeFile, error) {
	// Construct API URL
	apiURL := fmt.Sprintf("https://www.modelscope.cn/api/v1/models/%s/repo/files?Revision=%s&Recursive=True", modelName, config.ModelScopeRevision)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set request headers
	req.Header.Set("Accept", "application/json")

	// Send request (no timeout for API calls)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp ModelScopeFileRespData
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter files
	var files []ModelScopeFile
	for _, file := range apiResp.Data.Files {
		if file.Name == ".gitignore" || file.Name == ".gitmodules" || file.Type == "tree" {
			continue
		}
		files = append(files, file)
	}

	return files, nil
}

// asyncDownloadModelFile asynchronously downloads all model files
func (p *OvmsProvider) asyncDownloadModelFile(ctx context.Context, data AsyncDownloadModelFileData) {
	defer close(data.DataCh)
	defer close(data.ErrCh)

	// Download all files
	for _, file := range data.ModelFiles {
		if err := p.downloadModelFile(ctx, data.ModelName, file, data.LocalModelPath); err != nil {
			p.LogError("Failed to download file", err)
			data.ErrCh <- err
			return
		}

		// Send progress
		progress := sdktypes.ProgressResponse{
			Status:    fmt.Sprintf("downloaded %s", file.Name),
			Digest:    file.Digest,
			Total:     file.Size,
			Completed: file.Size,
		}
		if progressData, err := json.Marshal(progress); err == nil {
			data.DataCh <- progressData
		}
	}

	// Generate graph.pbtxt
	modelType := data.ModelType
	if modelType == "" {
		modelType = p.inferModelType(data.ModelName)
	}
	if err := p.generateGraphPBTxt(data.ModelName, modelType); err != nil {
		p.LogError("Failed to generate graph.pbtxt", err)
		data.ErrCh <- fmt.Errorf("failed to generate graph.pbtxt: %w", err)
		return
	}

	// Send completion message
	p.LogInfo(fmt.Sprintf("Pull model completed: %s", data.ModelName))
	success := sdktypes.ProgressResponse{Status: "success"}
	if successData, err := json.Marshal(success); err == nil {
		data.DataCh <- successData
	}
}

func (p *OvmsProvider) downloadModelFile(ctx context.Context, modelName string, file ModelScopeFile, localPath string) error {
	filePath := filepath.Join(localPath, file.Path)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if file already exists and has correct hash
	if _, err := os.Stat(filePath); err == nil {
		if p.verifyFileHash(filePath, file.Digest) {
			return nil
		}
		// Delete corrupted file
		os.Remove(filePath)
	}

	// Download file
	url := fmt.Sprintf("%s/%s/resolve/%s/%s", ModelScopeDownloadBase, modelName, config.ModelScopeRevision, file.Path)

	client := &http.Client{} // No timeout for model file downloads
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Write to file with hash verification
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	hash := sha256.New()
	writer := io.MultiWriter(out, hash)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Verify hash
	downloadedHash := hex.EncodeToString(hash.Sum(nil))
	if downloadedHash != file.Digest {
		os.Remove(filePath)
		return fmt.Errorf("hash mismatch for %s: expected %s, got %s", file.Name, file.Digest, downloadedHash)
	}

	return nil
}

func (p *OvmsProvider) verifyFileHash(filePath, expectedHash string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false
	}

	return hex.EncodeToString(hash.Sum(nil)) == expectedHash
}

// Configuration management methods

// getConfigPath returns the path to config.json
func (p *OvmsProvider) getConfigPath() string {
	return filepath.Join(p.config.EngineDir, "models", "config.json")
}

// loadConfig loads OVMS configuration
func (p *OvmsProvider) loadConfig() (*OVMSConfig, error) {
	configPath := p.getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config doesn't exist, return empty config
			return &OVMSConfig{
				MediapipeConfigList: []ModelConfig{},
				ModelConfigList:     []interface{}{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config OVMSConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// saveConfig saves OVMS configuration
func (p *OvmsProvider) saveConfig(config *OVMSConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := p.getConfigPath()
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (p *OvmsProvider) generateGraphPBTxt(modelName, modelType string) error {
	// Construct model directory (matching built-in OpenVINO)
	modelDir := fmt.Sprintf("%s/models/%s", p.config.EngineDir, modelName)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		p.LogError("Failed to create model directory", err)
		return err
	}

	// Replace backslashes with forward slashes for Windows compatibility
	enginePath := strings.Replace(p.config.EngineDir, "\\", "/", -1)

	var template string
	switch modelType {
	case "text-to-image":
		template = config.GraphPBTxtTextToImage
	case "speech-to-text", "speech-to-text-ws":
		template = fmt.Sprintf(config.GraphPBTxtSpeechToText, modelName, enginePath)
	case "text-to-speech":
		template = fmt.Sprintf(config.GraphPBTxtTextToSpeech, modelName, enginePath)
	case "chat":
		toolParser := config.InferToolParser(modelName)
		reasoningParser := "qwen3"
		template = fmt.Sprintf(config.GraphPBTxtChat, enginePath, modelName, toolParser, reasoningParser)
	case "generate":
		template = fmt.Sprintf(config.GraphPBTxtGenerate, enginePath, modelName)
	case "embed":
		template = fmt.Sprintf(config.GraphPBTxtEmbed, enginePath, modelName)
	case "rerank":
		template = fmt.Sprintf(config.GraphPBTxtRerank, enginePath, modelName)
	default:
		p.LogError("Unsupported model type: "+modelType, nil)
		return fmt.Errorf("unsupported model type: %s", modelType)
	}

	// Write graph.pbtxt
	graphPath := fmt.Sprintf("%s/graph.pbtxt", modelDir)
	if err := os.WriteFile(graphPath, []byte(template), 0o644); err != nil {
		return err
	}

	// For Chat service, also generate chat_template.jinja (matching built-in OpenVINO)
	if modelType == "chat" {
		chatTemplatePath := fmt.Sprintf("%s/chat_template.jinja", modelDir)
		if err := os.WriteFile(chatTemplatePath, []byte(config.ChatTemplateJinja), 0o644); err != nil {
			p.LogError("Failed to create chat_template.jinja", err)
			return err
		}
		p.LogInfo(fmt.Sprintf("Generated chat_template.jinja for model: %s", modelName))
	}

	return nil
}

func (p *OvmsProvider) inferModelType(modelName string) string {
	modelNameLower := strings.ToLower(modelName)

	// Infer model type from name
	if strings.Contains(modelNameLower, "embed") {
		return "embed"
	}
	if strings.Contains(modelNameLower, "rerank") {
		return "rerank"
	}
	if strings.Contains(modelNameLower, "t2i") || strings.Contains(modelNameLower, "text2image") {
		return "text-to-image"
	}
	if strings.Contains(modelNameLower, "s2t") || strings.Contains(modelNameLower, "speech2text") || strings.Contains(modelNameLower, "whisper") {
		return "speech-to-text"
	}
	if strings.Contains(modelNameLower, "tts") || strings.Contains(modelNameLower, "text2speech") {
		return "text-to-speech"
	}
	if strings.Contains(modelNameLower, "generate") {
		return "generate"
	}

	// Default to chat
	return "chat"
}

func (p *OvmsProvider) addModelToConfig(modelName, modelPath string) error {
	// Load config using new method
	config, err := p.loadConfig()
	if err != nil {
		return err
	}

	// Check if model already exists
	for _, m := range config.MediapipeConfigList {
		if m.Name == modelName {
			return nil
		}
	}

	// Add new model - matching built-in OpenVINO format
	// BasePath is not set (commented out in built-in)
	// GraphPath is relative path only
	newModel := ModelConfig{
		Name:      modelName,
		GraphPath: "graph.pbtxt",
	}
	config.MediapipeConfigList = append(config.MediapipeConfigList, newModel)

	// Save config using new method
	if err := p.saveConfig(config); err != nil {
		return err
	}

	return nil
}

func (p *OvmsProvider) removeModelFromConfig(modelName string) error {
	// Load config using new method
	config, err := p.loadConfig()
	if err != nil {
		return err
	}

	// Remove model
	newList := []ModelConfig{}
	for _, m := range config.MediapipeConfigList {
		if m.Name != modelName {
			newList = append(newList, m)
		}
	}
	config.MediapipeConfigList = newList

	// Save config using new method
	if err := p.saveConfig(config); err != nil {
		return err
	}

	return nil
}

func (p *OvmsProvider) isModelLoaded(modelName string) bool {
	// Check if model is in config and OVMS is running
	configPath := filepath.Join(p.config.EngineDir, "config.json")

	var config OVMSConfig
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	for _, m := range config.MediapipeConfigList {
		if m.Name == modelName {
			return true
		}
	}

	return false
}

// validateModelName validates the model name format
func validateModelName(modelName string) error {
	if modelName == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// Model name should not contain path separators or special characters
	invalidChars := []string{"/", "\\", "..", "~", "$", "`"}
	for _, char := range invalidChars {
		if strings.Contains(modelName, char) {
			return fmt.Errorf("model name contains invalid character: %s", char)
		}
	}

	// Model name should be reasonable length
	if len(modelName) > 255 {
		return fmt.Errorf("model name too long (max 255 characters)")
	}

	return nil
}

// checkModelMetadata checks if a model is loaded in OVMS by calling gRPC ModelMetadata
func (p *OvmsProvider) checkModelMetadata(modelName string) error {
	// Use config from provider instance (not the constant)
	grpcAddr := fmt.Sprintf("%s:%d", p.config.OVMSHost, p.config.OVMSGRPCPort)

	// Create gRPC connection
	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to OVMS gRPC at %s: %w", grpcAddr, err)
	}
	defer conn.Close()

	// Create gRPC client
	gClient := grpc_client.NewGRPCInferenceServiceClient(conn)

	// Call ModelMetadata to check if model is loaded
	req := &grpc_client.ModelMetadataRequest{
		Name: modelName,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = gClient.ModelMetadata(ctx, req)
	if err != nil {
		return fmt.Errorf("model not loaded: %w", err)
	}

	return nil
}
