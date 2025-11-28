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
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadFile downloads a file (ported from built-in utils.DownloadFile logic)
func DownloadFile(url, destDir string, cover bool) (string, error) {
	// Ensure target directory exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Parse filename from URL
	filename := filepath.Base(url)
	destPath := filepath.Join(destDir, filename)

	// Check if file already exists
	if !cover {
		if _, err := os.Stat(destPath); err == nil {
			// File already exists and no overwrite
			return destPath, nil
		}
	}

	// Make HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temporary file
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	// Write file content
	_, err = io.Copy(out, resp.Body)
	out.Close()

	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Rename to final file
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to rename file: %w", err)
	}

	return destPath, nil
}
