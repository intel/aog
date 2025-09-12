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
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	excleFile "github.com/xuri/excelize/v2"
)

type MemoryInfo struct {
	Size       int
	MemoryType string
}

func GetUserConfigDir() (string, error) {
	var dir string
	switch sys := runtime.GOOS; sys {
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Preferences")
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"))
	case "linux":
		// Use XDG_CONFIG_HOME for configuration files, fallback to ~/.config
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome != "" {
			dir = xdgConfigHome
		} else {
			dir = filepath.Join(os.Getenv("HOME"), ".config")
		}
	default:
		return "", fmt.Errorf("unsupported operating system")
	}
	return dir, nil
}

// GetUserCacheDir returns the user cache directory following XDG spec
func GetUserCacheDir() (string, error) {
	var dir string
	switch sys := runtime.GOOS; sys {
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Caches")
	case "windows":
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"))
	case "linux":
		// Use XDG_CACHE_HOME for cache files, fallback to ~/.cache
		xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
		if xdgCacheHome != "" {
			dir = xdgCacheHome
		} else {
			dir = filepath.Join(os.Getenv("HOME"), ".cache")
		}
	default:
		return "", fmt.Errorf("unsupported operating system")
	}
	return dir, nil
}

// CheckServiceIsExistInEnv checks if service exists in environment variables
func CheckServiceIsExistInEnv(serviceName string) bool {
	_, err := exec.LookPath(serviceName)
	return err != nil
}

func GetTmpDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// Windows: %TEMP% or %TMP%
		tempDir := os.Getenv("TEMP")
		if tempDir == "" {
			tempDir = os.Getenv("TMP")
		}
		if tempDir == "" {
			return "", fmt.Errorf("unable to get temp directory on Windows")
		}
		return tempDir, nil
	case "darwin":
		// macOS: /tmp or $TMPDIR
		tempDir := os.Getenv("TMPDIR")
		if tempDir == "" {
			tempDir = "/tmp"
		}
		return tempDir, nil
	case "linux":
		// Linux: $TMPDIR, $TMP, $TEMP, or /tmp
		tempDir := os.Getenv("TMPDIR")
		if tempDir == "" {
			tempDir = os.Getenv("TMP")
		}
		if tempDir == "" {
			tempDir = os.Getenv("TEMP")
		}
		if tempDir == "" {
			tempDir = "/tmp"
		}
		return tempDir, nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func GetGpuInfo() (int, error) {
	gpuInfo := "0"
	isIntelEngine := IpexOllamaSupportGPUStatus()
	if isIntelEngine {
		cmd := exec.Command("xpu-smi", "stats", "-d", "0")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		result := ParseTableOutput(string(output))
		gpuInfo = result["GPU Utilization (%)"]
	} else {
		cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}
		gpuInfo = string(output)
	}
	gpuUtilization, err := strconv.Atoi(gpuInfo)
	if err != nil {
		return 0, err
	}
	return gpuUtilization, nil
}

func IsServerRunning() bool {
	serverUrl := "http://127.0.0.1:16688" + "/health"
	resp, err := http.Get(serverUrl)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Min 辅助函数，返回两个整数中的较小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NowUnixMilli 获取当前Unix时间戳（毫秒）
func NowUnixMilli() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// 尝试从PDF文件提取文本
func ExtractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("PDF文件打开失败: %w", err)
	}
	defer f.Close()
	var sb strings.Builder
	b, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("PDF文本提取失败: %w", err)
	}
	_, err = io.Copy(&sb, b)
	if err != nil {
		return "", fmt.Errorf("PDF文本读取失败: %w", err)
	}
	return sb.String(), nil
}

// 尝试从Word文件提取文本
func ExtractTextFromDocx(filePath string) (string, error) {
	r, err := docx.ReadDocxFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Word文件打开失败")
	}
	if r == nil {
		return "", fmt.Errorf("Word文件打开失败")
	}
	defer r.Close()
	doc := r.Editable()
	content := doc.GetContent()
	// 用正则去除所有 <...> 标签，确保只返回纯文本
	reTag := regexp.MustCompile(`<[^>]+>`)
	cleanText := reTag.ReplaceAllString(content, "")
	return cleanText, nil
}

// 尝试从Excel文件提取文本
func ExtractTextFromXlsx(filePath string) (string, error) {
	f, err := excleFile.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("Excel文件打开失败")
	}
	if f == nil {
		return "", fmt.Errorf("excel文件打开失败")
	}
	defer f.Close()
	var sb strings.Builder
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// 按chunkSize对纯文本内容分块
func ChunkTextContent(text string, chunkSize int) []string {
	reTag := regexp.MustCompile(`<[^>]+>`)
	cleanText := reTag.ReplaceAllString(text, "")
	reKeep := regexp.MustCompile(`[\p{Han}\p{L}\p{N}\p{P}\p{Zs}，。！？；：“”‘’、·…—\-\(\)\[\]{}<>《》\n\r\t]+`)
	filtered := reKeep.FindAllString(cleanText, -1)
	finalText := strings.Join(filtered, "")
	var chunks []string
	runes := []rune(finalText)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
