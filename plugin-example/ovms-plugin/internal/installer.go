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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/config"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/utils"
)

// InstallEngine downloads and installs OVMS engine and scripts
func (p *OvmsProvider) InstallEngine(ctx context.Context) error {
	// Linux distribution support check (matching built-in OpenVINO)
	if runtime.GOOS == "linux" {
		distro, version, err := DetectLinuxDistribution()
		if err != nil {
			return fmt.Errorf("failed to detect Linux distribution: %w", err)
		}

		// Only support Ubuntu 22.04/24.04 and Deepin
		supported := false
		if distro == "ubuntu" && (version == "22.04" || version == "24.04") {
			supported = true
		} else if distro == "deepin" {
			supported = true
		}

		if !supported {
			return fmt.Errorf("unsupported Linux distribution: %s %s. Only Ubuntu 22.04, Ubuntu 24.04, and Deepin are supported", distro, version)
		}
	}

	// Create necessary directories
	if err := p.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Determine download URLs based on platform
	var ovmsURL, scriptsURL string
	switch runtime.GOOS {
	case "windows":
		ovmsURL = config.GetOVMSDownloadURL("windows", "", "", "")
		scriptsURL = config.GetScriptsDownloadURL("windows", config.OVMSVersion)
	case "linux":
		distro, version, err := DetectLinuxDistribution()
		if err != nil {
			distro = "ubuntu"
			version = "22.04"
		}
		ovmsURL = config.GetOVMSDownloadURL("linux", distro, version, "")
		scriptsURL = config.GetScriptsDownloadURL("linux", config.OVMSVersion)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	if ovmsURL == "" {
		return fmt.Errorf("no download URL available for current platform")
	}

	// Download OVMS
	ovmsFile := filepath.Join(p.config.DownloadDir, filepath.Base(ovmsURL))
	if err := p.downloadFile(ctx, ovmsURL, ovmsFile); err != nil {
		return fmt.Errorf("failed to download OVMS: %w", err)
	}

	// Download scripts
	scriptsFile := filepath.Join(p.config.DownloadDir, "scripts.zip")
	if err := p.downloadFile(ctx, scriptsURL, scriptsFile); err != nil {
		return fmt.Errorf("failed to download scripts: %w", err)
	}

	// Extract OVMS to EngineDir (matches built-in OVMS behavior)
	// After extraction:
	//   Windows: EngineDir/ovms.exe, EngineDir/setupvars.bat, EngineDir/python/*
	//   Linux: EngineDir/ovms/bin/ovms, EngineDir/ovms/lib/*, etc.
	if err := extractArchive(ovmsFile, p.config.EngineDir); err != nil {
		return fmt.Errorf("failed to extract OVMS: %w", err)
	}

	// Extract scripts (zip already contains "scripts" directory structure, so extract to EngineDir)
	if err := extractArchive(scriptsFile, p.config.EngineDir); err != nil {
		return fmt.Errorf("failed to extract scripts: %w", err)
	}

	// Run installation script
	if err := p.runInstallScript(); err != nil {
		return fmt.Errorf("failed to run install script: %w", err)
	}

	// Create config.json file (required for OVMS to start)
	if err := p.initializeConfig(); err != nil {
		return fmt.Errorf("failed to create config.json: %w", err)
	}

	return nil
}

// InitEnv initializes the environment (placeholder for now)
func (p *OvmsProvider) InitEnv() error {
	return nil
}

// UpgradeEngine upgrades the OVMS engine
func (p *OvmsProvider) UpgradeEngine(ctx context.Context) error {
	// Stop engine before upgrade
	if err := p.StopEngine(); err != nil {
		return fmt.Errorf("failed to stop engine: %w", err)
	}

	// Re-install (download latest version)
	if err := p.InstallEngine(ctx); err != nil {
		return fmt.Errorf("failed to upgrade engine: %w", err)
	}

	return nil
}

// Helper functions

func (p *OvmsProvider) createDirectories() error {
	dirs := []string{
		p.config.DataDir,
		p.config.EngineDir,
		p.config.ExecDir,
		p.config.DownloadDir,
		filepath.Join(p.config.EngineDir, "models"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (p *OvmsProvider) downloadFile(ctx context.Context, url, destPath string) error {
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		return nil
	}

	// Create HTTP client (no timeout for large file downloads)
	client := &http.Client{}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy with progress tracking
	totalBytes := resp.ContentLength
	var downloadedBytes int64

	buffer := make([]byte, 64*1024) // 64KB buffer
	lastLog := time.Now()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloadedBytes += int64(n)

			// Update last log time for progress tracking
			if time.Since(lastLog) > 5*time.Second && totalBytes > 0 {
				lastLog = time.Now()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

func extractArchive(src, dest string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use Go standard library for extraction (cross-platform, no external dependencies)
	if err := utils.UnzipFile(src, dest); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

func (p *OvmsProvider) runInstallScript() error {
	var scriptPath string
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		scriptPath = filepath.Join(p.config.EngineDir, "install.bat")
		if _, err := os.Stat(scriptPath); err != nil {
			// Create install script if not exists
			if err := p.generateWindowsInstallScript(scriptPath); err != nil {
				return err
			}
		}
		cmd = exec.Command("cmd", "/c", scriptPath)

	case "linux":
		// Detect distribution to use appropriate script
		distro, version, err := DetectLinuxDistribution()
		if err != nil {
			distro = "ubuntu"
			version = "22.04"
		}

		scriptPath = filepath.Join(p.config.EngineDir, "install.sh")
		if err := p.generateLinuxInstallScript(scriptPath, distro, version); err != nil {
			return err
		}

		// Make script executable
		if err := os.Chmod(scriptPath, 0o755); err != nil {
			return fmt.Errorf("failed to make script executable: %w", err)
		}

		cmd = exec.Command("/bin/bash", scriptPath)

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Set working directory
	cmd.Dir = p.config.EngineDir

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install script failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (p *OvmsProvider) generateWindowsInstallScript(scriptPath string) error {
	engineDir := p.config.EngineDir
	execDir := p.config.ExecDir

	// Convert to Windows path format
	engineDir = strings.ReplaceAll(engineDir, "/", "\\")
	execDir = strings.ReplaceAll(execDir, "/", "\\")

	// Match built-in OVMS: setupvars from execDir, python from execDir, requirements from engineDir/scripts
	script := fmt.Sprintf(`@echo on
call "%s\setupvars.bat"
set PATH=%s\python\Scripts;%%PATH%%
python -m pip install -r "%s\scripts\requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/ --break-system-packages
`, execDir, execDir, engineDir)

	return os.WriteFile(scriptPath, []byte(script), 0o644)
}

func (p *OvmsProvider) generateLinuxInstallScript(scriptPath, distro, version string) error {
	// Match built-in OVMS path structure
	// Now ExecDir = EngineDir, OVMS extracted to EngineDir/ovms/*
	engineDir := p.config.EngineDir

	// Environment variables match built-in OVMS exactly
	// Built-in: libPath = execPath/ovms/lib, binPath = execPath/ovms/bin
	ldLibPath := fmt.Sprintf("%s/ovms/lib", engineDir) // EngineDir/ovms/lib
	binPath := fmt.Sprintf("%s/ovms/bin", engineDir)   // EngineDir/ovms/bin
	pythonPath := fmt.Sprintf("%s/python", ldLibPath)  // EngineDir/ovms/lib/python

	var script string

	if distro == "ubuntu" {
		if version == "24.04" {
			// Match InitShellLinuxUbuntu2404 exactly
			script = fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo apt -y install libpython3.12
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/ --break-system-packages
`, ldLibPath, binPath, pythonPath, engineDir)
		} else {
			// Default to 22.04, match InitShellLinuxUbuntu2204 exactly
			script = fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo apt -y install libpython3.10 python3-pip
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/ --break-system-packages
`, ldLibPath, binPath, pythonPath, engineDir)
		}
	} else if distro == "rhel" || distro == "centos" || distro == "rocky" {
		// Match InitShellLinuxREHL96 format
		script = fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
sudo yum install -y python39-libs
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/ --break-system-packages
`, ldLibPath, binPath, pythonPath, engineDir)
	} else {
		// Default fallback
		script = fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
python3 -m pip install "Jinja2==3.1.6" "MarkupSafe==3.0.2"
python3 -m pip install -r "%s/scripts/requirements.txt" -i https://mirrors.aliyun.com/pypi/simple/ --break-system-packages || true
`, ldLibPath, binPath, pythonPath, engineDir)
	}

	return os.WriteFile(scriptPath, []byte(script), 0o755)
}
