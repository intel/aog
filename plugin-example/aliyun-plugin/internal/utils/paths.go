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

// ExpandPath expands environment variables in the path.
// Supported variables:
//
//	${PLUGIN_DIR} - directory containing the plugin executable
//	${DATA_DIR} - data root directory
//	${HOME} - user home directory
func ExpandPath(path string, vars map[string]string) (string, error) {
	// Return error when the path is empty
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Expand all provided placeholders
	expanded := path
	for key, value := range vars {
		placeholder := fmt.Sprintf("${%s}", key)
		expanded = strings.ReplaceAll(expanded, placeholder, value)
	}

	// Expand user home (~)
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
			return "", fmt.Errorf("failed to convert to absolute path: %w", err)
		}
		expanded = absPath
	}

	// Clean the resulting path
	expanded = filepath.Clean(expanded)

	return expanded, nil
}

// GetPluginDir returns the directory where the plugin executable resides
func GetPluginDir() (string, error) {
	// Determine executable path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Look upward from the executable directory for plugin.yaml
	// Executable may reside in nested directories like bin/darwin-arm64/
	currentDir := filepath.Dir(execPath)

	for i := 0; i < 5; i++ { // search up to 5 levels
		manifestPath := filepath.Join(currentDir, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			// Found plugin.yaml: this is the plugin root
			return currentDir, nil
		}

		// Move one directory up
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("plugin.yaml not found in any parent directory of %s", execPath)
}

// EnsureDir ensures the directory exists
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

// PathExists returns true if the path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsExecutable checks whether the file is executable
func IsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// On Windows just verify the file exists
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}

	// On Unix check executable bit
	return !info.IsDir() && (info.Mode()&0o111 != 0)
}
