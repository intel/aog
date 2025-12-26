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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	sdktypes "github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/config"
)

// Engine lifecycle management

// GetConfig returns engine configuration based on platform
func (p *OvmsProvider) GetConfig(ctx context.Context) (*sdktypes.EngineRecommendConfig, error) {
	config := &sdktypes.EngineRecommendConfig{
		// Basic info
	}

	// Determine download URLs based on platform
	switch runtime.GOOS {
	case "windows":
		// config.EngineDownloadURL = config.GetOVMSDownloadURL("windows", "", "", "v0.8.1")
		// config.ScriptsDownloadURL = config.GetScriptsDownloadURL("windows", "v0.8.1")
	case "linux":
		// config.EngineDownloadURL = config.GetOVMSDownloadURL("linux", "", "", "v0.8.1")
		// config.ScriptsDownloadURL = config.GetScriptsDownloadURL("linux", "v0.8.1")
	case "darwin":
		return nil, fmt.Errorf("macOS is not supported for OVMS")
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return config, nil
}

// StartEngine starts the OVMS server process
func (p *OvmsProvider) StartEngine(mode string) error {
	// Create context with timeout for startup operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if engine is already running
	if p.cmd != nil && p.cmd.Process != nil {
		return nil
	}

	// Verify engine installation
	enginePath := p.config.ExecDir
	if _, err := os.Stat(enginePath); os.IsNotExist(err) {
		return fmt.Errorf("OVMS engine not installed at: %s", enginePath)
	}

	// Prepare config.json path (must be in models/ subdirectory)
	modelsDir := filepath.Join(p.config.EngineDir, "models")
	configPath := filepath.Join(modelsDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := p.initializeConfig(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
	}

	// Verify config.json exists and is readable
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config.json not found at: %s", configPath)
	}

	// Build command based on platform using startup scripts
	var cmd *exec.Cmd
	var scriptFile string

	if runtime.GOOS == "windows" {
		// Windows: Create batch script
		batchContent := p.generateWindowsStartScript()
		scriptFile = filepath.Join(p.config.ExecDir, "start_ovms.bat")

		if err := os.WriteFile(scriptFile, []byte(batchContent), 0o644); err != nil {
			return fmt.Errorf("failed to create batch file: %w", err)
		}

		cmd = exec.Command("cmd", "/C", scriptFile)
		cmd.Dir = p.config.EngineDir
	} else {
		// Linux: Create shell script
		shellContent := p.generateLinuxStartScript()
		scriptFile = filepath.Join(p.config.ExecDir, "start_ovms.sh")

		if err := os.WriteFile(scriptFile, []byte(shellContent), 0o755); err != nil {
			return fmt.Errorf("failed to create shell script: %w", err)
		}

		cmd = exec.Command("/bin/bash", scriptFile)
		cmd.Dir = p.config.EngineDir
	}

	// Set stdout/stderr to standard output (AOG will capture plugin logs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start OVMS: %w", err)
	}

	p.cmd = cmd

	// Wait for server to be ready
	if err := p.waitForReady(ctx, 30*time.Second); err != nil {
		p.StopEngine()
		return fmt.Errorf("OVMS failed to become ready: %w", err)
	}
	return nil
}

// StopEngine stops the OVMS server process
func (p *OvmsProvider) StopEngine() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Try graceful shutdown first (platform-specific)
	if runtime.GOOS == "windows" {
		// Windows: use taskkill to terminate process tree (including all child processes)
		pid := p.cmd.Process.Pid

		// Use taskkill with /F (force) and /T (tree) flags
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
		if err := killCmd.Run(); err != nil {
			// Fallback to direct Kill if taskkill fails
			if err := p.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		}

		// Wait for process to exit
		p.cmd.Wait()
	} else {
		// Unix-like systems: try graceful shutdown with SIGINT
		p.cmd.Process.Signal(os.Interrupt)

		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- p.cmd.Wait()
		}()

		select {
		case <-time.After(10 * time.Second):
			if err := p.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
			<-done
		case <-done:
		}
	}

	p.cmd = nil
	return nil
}

// HealthCheck checks if the OVMS server is healthy
func (p *OvmsProvider) HealthCheck(ctx context.Context) error {
	// Try HTTP health endpoint
	healthURL := fmt.Sprintf("http://127.0.0.1:%s/v1/config", config.OpenvinoHTTPPort)

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Don't log every failure, just return error
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
	}

	return nil
}

// GetVersion returns the OVMS version
func (p *OvmsProvider) GetVersion(ctx context.Context, req *sdktypes.EngineVersionResponse) (*sdktypes.EngineVersionResponse, error) {
	// Try to get version from running server
	versionURL := fmt.Sprintf("http://127.0.0.1:%s/v1/config", config.OpenvinoHTTPPort)

	client := &http.Client{Timeout: 5 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		// Fallback to default version
		return &sdktypes.EngineVersionResponse{
			Version: "2025.0",
		}, nil
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		// Fallback to default version
		return &sdktypes.EngineVersionResponse{
			Version: "2025.0",
		}, nil
	}
	defer resp.Body.Close()

	// Parse version from response if available
	// For now, return default version
	return &sdktypes.EngineVersionResponse{
		Version: "2025.0",
	}, nil
}

// CheckEngineCtx verifies if the engine is installed (with context)
func (p *OvmsProvider) CheckEngineCtx(ctx context.Context) (bool, error) {
	enginePath := p.config.ExecDir
	var ovmsExe string
	if runtime.GOOS == "windows" {
		ovmsExe = filepath.Join(enginePath, "ovms.exe")
	} else {
		ovmsExe = filepath.Join(enginePath, "ovms")
	}

	if _, err := os.Stat(ovmsExe); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// Helper functions

func (p *OvmsProvider) waitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			attempt++
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for OVMS to be ready after %d attempts", attempt)
			}

			err := p.HealthCheck(ctx)
			if err == nil {
				return nil
			}
		}
	}
}

func (p *OvmsProvider) initializeConfig() error {
	// IMPORTANT: config.json must be in models/ subdirectory (same as built-in OpenVINO)
	modelsDir := filepath.Join(p.config.EngineDir, "models")
	configPath := filepath.Join(modelsDir, "config.json")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	// Ensure models directory exists
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	// Create config with both fields (matching built-in OpenVINO structure)
	config := &OVMSConfig{
		MediapipeConfigList: []ModelConfig{},
		ModelConfigList:     []interface{}{},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// OVMSConfig represents OVMS configuration (matching built-in OpenVINO structure)
type OVMSConfig struct {
	MediapipeConfigList []ModelConfig `json:"mediapipe_config_list"`
	ModelConfigList     []interface{} `json:"model_config_list"`
}

// ModelConfig represents a model configuration
type ModelConfig struct {
	Name      string `json:"name"`
	BasePath  string `json:"base_path,omitempty"`
	GraphPath string `json:"graph_path,omitempty"`
}

// DetectLinuxDistribution detects the Linux distribution and version
func DetectLinuxDistribution() (string, string, error) {
	if runtime.GOOS != "linux" {
		return "", "", fmt.Errorf("not a Linux system")
	}

	// Try /etc/os-release first
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(data), "\n")
	var distro, version string

	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}

	if distro == "" {
		return "", "", fmt.Errorf("failed to detect distribution")
	}

	// Normalize distro name
	distro = normalizeDistroName(distro)

	return distro, version, nil
}

func normalizeDistroName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "ubuntu":
		return "ubuntu"
	case "deepin":
		return "deepin"
	case "rhel", "redhat":
		return "rhel"
	case "centos":
		return "centos"
	default:
		return name
	}
}

// generateWindowsStartScript generates Windows batch script for starting OVMS
func (p *OvmsProvider) generateWindowsStartScript() string {
	modelDir := filepath.Join(p.config.EngineDir, "models")
	ovmsExe := filepath.Join(p.config.ExecDir, "ovms.exe")

	// Convert to Windows path format
	execPath := strings.ReplaceAll(p.config.ExecDir, "/", "\\")
	enginePath := strings.ReplaceAll(p.config.EngineDir, "/", "\\")
	modelDir = strings.ReplaceAll(modelDir, "/", "\\")
	ovmsExe = strings.ReplaceAll(ovmsExe, "/", "\\")

	return fmt.Sprintf(`@echo on
call "%s\setupvars.bat"
set PATH=%s\python\Scripts;%%PATH%%
set HF_HOME=%s\.cache
set HF_ENDPOINT=https://hf-mirror.com
"%s" --port %s --rest_port %s --grpc_bind_address 127.0.0.1 --config_path "%s\config.json"`,
		execPath,
		execPath,
		enginePath,
		ovmsExe,
		config.OpenvinoGRPCPort,
		config.OpenvinoHTTPPort,
		modelDir)
}

// generateLinuxStartScript generates Linux shell script for starting OVMS
func (p *OvmsProvider) generateLinuxStartScript() string {
	modelDir := filepath.Join(p.config.EngineDir, "models")
	libPath := filepath.Join(p.config.EngineDir, "lib")
	binPath := filepath.Join(p.config.EngineDir, "bin")
	pythonPath := filepath.Join(libPath, "python")
	ovmsExe := filepath.Join(p.config.ExecDir, "ovms")

	return fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
"%s" --port %s --rest_port %s --grpc_bind_address 127.0.0.1 --config_path "%s/config.json"`,
		libPath,
		binPath,
		pythonPath,
		ovmsExe,
		config.OpenvinoGRPCPort,
		config.OpenvinoHTTPPort,
		modelDir)
}
