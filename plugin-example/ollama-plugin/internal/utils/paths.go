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

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExpandPath expands environment variables in path
// Supported variables:
//
//	${PLUGIN_DIR} - Plugin executable directory
//	${DATA_DIR} - Data root directory
//	${HOME} - User home directory
func ExpandPath(path string, vars map[string]string) (string, error) {
	// If empty path, return error
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Expand all variables
	expanded := path
	for key, value := range vars {
		placeholder := fmt.Sprintf("${%s}", key)
		expanded = strings.ReplaceAll(expanded, placeholder, value)
	}

	// Expand user home directory (~)
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home dir: %w", err)
		}
		expanded = filepath.Join(home, expanded[2:])
	}

	// Convert to absolute path
	if !filepath.IsAbs(expanded) {
		absPath, err := filepath.Abs(expanded)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		expanded = absPath
	}

	// Clean path
	expanded = filepath.Clean(expanded)

	return expanded, nil
}

// GetPluginDir gets plugin executable directory
func GetPluginDir() (string, error) {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Search upward from executable directory for plugin.yaml
	// Executable might be in subdirectories like bin/darwin-arm64/
	currentDir := filepath.Dir(execPath)

	for i := 0; i < 5; i++ { // Search up to 5 levels
		manifestPath := filepath.Join(currentDir, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			// Found plugin.yaml, this is the plugin root directory
			return currentDir, nil
		}

		// Go up one level
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Already reached root directory
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("plugin.yaml not found in any parent directory of %s", execPath)
}

// GetOllamaExecutableName gets ollama executable name (cross-platform)
func GetOllamaExecutableName() string {
	if runtime.GOOS == "windows" {
		return "ollama.exe"
	}
	return "ollama"
}

// EnsureDir ensures directory exists
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

// EnsureDirs ensures multiple directories exist
func EnsureDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := EnsureDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// PathExists checks if path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsExecutable checks if file is executable
func IsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Windows only check file exists
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}

	// On Unix systems check execute permission
	return !info.IsDir() && (info.Mode()&0o111 != 0)
}
