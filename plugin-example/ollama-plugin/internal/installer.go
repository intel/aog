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
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/intel/aog/plugin/examples/ollama-plugin/internal/utils"
)

// =============== EngineInstaller Implementation ===============

// CheckEngine checks if ollama is installed (checks plugin-managed directory)
func (p *OllamaProvider) CheckEngine() (bool, error) {
	log.Printf("[ollama-plugin] [INFO] Checking engine installation...")
	config, err := p.getConfig()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get config: %v", err)
		return false, err
	}

	log.Printf("[ollama-plugin] [DEBUG] Checking exec path: %s", config.ExecPath)

	// Check if file exists
	if !utils.PathExists(config.ExecPath) {
		log.Printf("[ollama-plugin] [INFO] Engine not found at: %s", config.ExecPath)
		return false, nil
	}

	// Verify executability
	isExec := utils.IsExecutable(config.ExecPath)
	log.Printf("[ollama-plugin] [INFO] Engine check result: installed=%v, executable=%v", true, isExec)
	return isExec, nil
}

// InstallEngine installs ollama engine (complete port of built-in logic)
func (p *OllamaProvider) InstallEngine(ctx context.Context) error {
	log.Printf("[ollama-plugin] [INFO] Starting engine installation...")
	config, err := p.getConfig()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get config: %v", err)
		return fmt.Errorf("failed to get config: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] Download URL: %s", config.DownloadURL)
	log.Printf("[ollama-plugin] [INFO] Download directory: %s", config.DownloadDir)
	log.Printf("[ollama-plugin] [INFO] Target exec path: %s", config.ExecPath)

	// Download ollama package (default no overwrite)
	cover := false
	log.Printf("[ollama-plugin] [INFO] Downloading ollama package...")
	file, err := utils.DownloadFile(config.DownloadURL, config.DownloadDir, cover)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Download failed: %v", err)
		return fmt.Errorf("failed to download ollama: %w", err)
	}
	log.Printf("[ollama-plugin] [INFO] Download completed: %s", file)

	// Call corresponding installation method based on platform
	log.Printf("[ollama-plugin] [INFO] Installing for platform: %s", runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		return p.installEngineWindows(file, config, cover)
	case "linux":
		return p.installEngineLinux(file, config, cover)
	case "darwin":
		return p.installEngineMacOS(file, config, cover)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// installEngineWindows Windows platform installation (ported from built-in engine)
func (p *OllamaProvider) installEngineWindows(file string, config *Config, cover bool) error {
	targetDir := filepath.Dir(config.ExecPath)

	// Delete old directory when overwriting
	if cover {
		if _, err := os.Stat(targetDir); err == nil {
			os.RemoveAll(targetDir)
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Extract installation package
	if err := utils.UnzipFile(file, targetDir); err != nil {
		return fmt.Errorf("failed to extract ollama: %w", err)
	}

	return nil
}

// installEngineLinux Linux platform installation (ported from built-in engine)
func (p *OllamaProvider) installEngineLinux(file string, config *Config, cover bool) error {
	targetDir := config.EngineDir

	// Delete old directory when overwriting
	if cover {
		if _, err := os.Stat(targetDir); err == nil {
			os.RemoveAll(targetDir)
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Extract installation package
	if err := utils.UnzipFile(file, targetDir); err != nil {
		return fmt.Errorf("failed to extract ollama: %w", err)
	}

	// Set executable permission
	if err := os.Chmod(config.ExecPath, 0o755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	return nil
}

// installEngineMacOS macOS platform installation (ported from built-in engine)
func (p *OllamaProvider) installEngineMacOS(file string, config *Config, cover bool) error {
	log.Printf("[ollama-plugin] [INFO] Installing engine for macOS...")
	// macOS needs to extract ollama executable from .app bundle
	targetDir := config.EngineDir
	log.Printf("[ollama-plugin] [DEBUG] Target directory: %s", targetDir)

	// Delete old directory when overwriting
	if cover {
		if _, err := os.Stat(targetDir); err == nil {
			log.Printf("[ollama-plugin] [INFO] Removing old installation...")
			os.RemoveAll(targetDir)
		}
	}

	// Create target directory
	log.Printf("[ollama-plugin] [INFO] Creating target directory...")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to create directory: %v", err)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Extract to temporary directory
	tmpDir := filepath.Join(config.DownloadDir, "tmp")
	log.Printf("[ollama-plugin] [INFO] Creating temporary directory: %s", tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to create temp directory: %v", err)
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	log.Printf("[ollama-plugin] [INFO] Extracting archive: %s", file)
	if err := utils.UnzipFile(file, tmpDir); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to extract: %v", err)
		return fmt.Errorf("failed to extract ollama: %w", err)
	}
	log.Printf("[ollama-plugin] [INFO] Archive extracted successfully")

	// Find Ollama.app and extract ollama executable
	appPath := filepath.Join(tmpDir, "Ollama.app")
	log.Printf("[ollama-plugin] [DEBUG] Looking for Ollama.app at: %s", appPath)
	if _, err := os.Stat(appPath); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Ollama.app not found")
		return fmt.Errorf("ollama.app not found in archive")
	}

	// Extract Resources/ollama executable
	srcOllama := filepath.Join(appPath, "Contents", "Resources", "ollama")
	log.Printf("[ollama-plugin] [DEBUG] Looking for ollama executable at: %s", srcOllama)
	if _, err := os.Stat(srcOllama); err != nil {
		log.Printf("[ollama-plugin] [ERROR] ollama executable not found in app bundle")
		return fmt.Errorf("ollama executable not found in app bundle")
	}

	// Create bin directory
	binDir := filepath.Dir(config.ExecPath)
	log.Printf("[ollama-plugin] [INFO] Creating bin directory: %s", binDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to create bin directory: %v", err)
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Copy executable
	log.Printf("[ollama-plugin] [INFO] Copying ollama executable to: %s", config.ExecPath)
	input, err := os.ReadFile(srcOllama)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to read ollama executable: %v", err)
		return fmt.Errorf("failed to read ollama executable: %w", err)
	}

	if err := os.WriteFile(config.ExecPath, input, 0o755); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to write ollama executable: %v", err)
		return fmt.Errorf("failed to write ollama executable: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] ✅ Engine installed successfully at: %s", config.ExecPath)
	return nil
}

// InitEnv initializes environment variables (ported from built-in engine)
func (p *OllamaProvider) InitEnv() error {
	config, err := p.getConfig()
	if err != nil {
		return err
	}

	// Set OLLAMA_HOST
	if err := os.Setenv("OLLAMA_HOST", config.Host); err != nil {
		return fmt.Errorf("failed to set OLLAMA_HOST: %w", err)
	}

	// Set OLLAMA_MODELS
	if err := os.Setenv("OLLAMA_MODELS", config.ModelsDir); err != nil {
		return fmt.Errorf("failed to set OLLAMA_MODELS: %w", err)
	}

	return nil
}

// UpgradeEngine upgrades ollama engine (ported from built-in engine)
func (p *OllamaProvider) UpgradeEngine(ctx context.Context) error {
	// Get current version
	currentVersion, err := p.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Compare versions
	if utils.VersionCompare(currentVersion, utils.OllamaMinVersion) >= 0 {
		return nil // Already latest version
	}

	// Stop engine
	if err := p.StopEngine(); err != nil {
		return fmt.Errorf("failed to stop engine: %w", err)
	}

	// Install new version (using context)
	if err := p.InstallEngine(ctx); err != nil {
		return fmt.Errorf("failed to install new version: %w", err)
	}

	return nil
}

// getCurrentVersion gets current ollama version
func (p *OllamaProvider) getCurrentVersion() (string, error) {
	config, err := p.getConfig()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(config.ExecPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse version number (simplified implementation, extract version from output)
	version := strings.TrimSpace(string(output))
	// If output format is "ollama version is 0.x.x", extract version number
	if strings.Contains(version, "version is ") {
		parts := strings.Split(version, "version is ")
		if len(parts) > 1 {
			version = strings.TrimSpace(parts[1])
		}
	}

	return version, nil
}

// getConfig helper method to get configuration
func (p *OllamaProvider) getConfig() (*Config, error) {
	// Get configuration from provider
	if p.config != nil {
		return p.config, nil
	}
	return LoadConfig()
}
