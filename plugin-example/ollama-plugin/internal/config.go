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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/intel/aog/plugin/examples/ollama-plugin/internal/utils"
	"gopkg.in/yaml.v3"
)

// Config holds the Ollama plugin configuration
type Config struct {
	// Basic configuration
	Host   string
	Scheme string
	Port   string

	// Default model
	DefaultModel string

	// Request timeout
	Timeout time.Duration

	// Plugin resource paths (read from plugin.yaml)
	EngineDir   string // Root data directory
	DownloadDir string // Download directory
	ModelsDir   string // Models directory
	ExecPath    string // Ollama executable path

	// GPU and platform information
	DeviceType string // GPU type
	Platform   string // Platform information

	// Download URL (automatically selected based on platform and GPU)
	DownloadURL string
}

// ResourcesConfig resource configuration (read from plugin.yaml)
type ResourcesConfig struct {
	DataDir string          `yaml:"data_dir"`
	Ollama  OllamaResources `yaml:"ollama"`
}

// OllamaResources Ollama resource configuration
type OllamaResources struct {
	Executable  string `yaml:"executable"`
	ModelsDir   string `yaml:"models_dir"`
	DownloadDir string `yaml:"download_dir"`
}

// PluginManifest plugin manifest (simplified version, only reads resources section)
type PluginManifest struct {
	Resources ResourcesConfig `yaml:"resources"`
}

// LoadConfig loads configuration and initializes plugin-managed paths
func LoadConfig() (*Config, error) {
	config := &Config{
		Scheme:       getEnv("OLLAMA_SCHEME", "http"),
		Port:         getEnv("OLLAMA_PORT", "16677"),
		DefaultModel: getEnv("OLLAMA_DEFAULT_MODEL", "qwen3:0.6b"),
		Timeout:      30 * time.Second,
	}

	// Initialize plugin-managed resource paths
	if err := initPluginResources(config); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin resources: %w", err)
	}

	// Detect GPU type
	config.DeviceType = utils.DetectGPUType()
	config.Platform = fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	// Select download URL
	config.DownloadURL = selectDownloadURL(config.DeviceType)

	// Set default Host
	if config.Host == "" {
		config.Host = "127.0.0.1:" + config.Port
	}

	// Validate scheme
	if config.Scheme != "http" && config.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme: %s (must be http or https)", config.Scheme)
	}

	return config, nil
}

// initPluginResources initializes plugin-managed resource paths
func initPluginResources(config *Config) error {
	// 1. Read resources configuration from plugin.yaml
	resources, err := loadResourcesFromManifest()
	if err != nil {
		return fmt.Errorf("failed to load resources config: %w", err)
	}

	// 2. Get plugin directory
	pluginDir, err := utils.GetPluginDir()
	if err != nil {
		return fmt.Errorf("failed to get plugin dir: %w", err)
	}

	// 3. Get AOG unified data directory (consistent with built-in engine)
	aogDataDir, err := getAOGDataDir()
	if err != nil {
		return fmt.Errorf("failed to get AOG data dir: %w", err)
	}

	// 4. Build environment variable mapping
	vars := map[string]string{
		"PLUGIN_DIR":   pluginDir,
		"HOME":         os.Getenv("HOME"),
		"AOG_DATA_DIR": aogDataDir, // New: AOG unified data directory
	}

	// 5. Expand data_dir
	dataDir, err := utils.ExpandPath(resources.DataDir, vars)
	if err != nil {
		return fmt.Errorf("failed to expand data_dir: %w", err)
	}
	config.EngineDir = dataDir

	// 6. Add DATA_DIR to variable mapping
	vars["DATA_DIR"] = dataDir

	// 7. Expand ollama paths
	execPath, err := utils.ExpandPath(resources.Ollama.Executable, vars)
	if err != nil {
		return fmt.Errorf("failed to expand ollama executable path: %w", err)
	}
	config.ExecPath = execPath

	modelsDir, err := utils.ExpandPath(resources.Ollama.ModelsDir, vars)
	if err != nil {
		return fmt.Errorf("failed to expand models_dir: %w", err)
	}
	config.ModelsDir = modelsDir

	downloadDir, err := utils.ExpandPath(resources.Ollama.DownloadDir, vars)
	if err != nil {
		return fmt.Errorf("failed to expand download_dir: %w", err)
	}
	config.DownloadDir = downloadDir

	// 8. Ensure directories exist
	if err := utils.EnsureDirs(dataDir, modelsDir, downloadDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	return nil
}

// loadResourcesFromManifest loads resources configuration from plugin.yaml
func loadResourcesFromManifest() (*ResourcesConfig, error) {
	// Get plugin.yaml path
	pluginDir, err := utils.GetPluginDir()
	if err != nil {
		return nil, err
	}

	manifestPath := filepath.Join(pluginDir, "plugin.yaml")

	// Read file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// If file doesn't exist, use default configuration
		if os.IsNotExist(err) {
			return getDefaultResources(), nil
		}
		return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
	}

	// Parse YAML
	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.yaml: %w", err)
	}

	// If resources not configured, use default values
	if manifest.Resources.DataDir == "" {
		return getDefaultResources(), nil
	}

	return &manifest.Resources, nil
}

// getDefaultResources gets default resource configuration (consistent with built-in engine)
func getDefaultResources() *ResourcesConfig {
	return &ResourcesConfig{
		DataDir: "${AOG_DATA_DIR}/engine/ollama",
		Ollama: OllamaResources{
			Executable:  "${DATA_DIR}/bin/ollama",
			ModelsDir:   "${DATA_DIR}/models",
			DownloadDir: "${HOME}/Downloads",
		},
	}
}

// selectDownloadURL selects download URL based on platform and GPU type
// Complete port of URL construction logic from built-in engine
func selectDownloadURL(gpuType string) string {
	// AOG download base URL (ported from built-in engine)
	const baseDownloadURL = "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog"
	const urlDirPathWindows = "/windows"
	const urlDirPathLinux = "/linux"
	const urlDirPathMacOS = "/mac"

	version := getEnv("AOG_VERSION", "0.6.0")
	arch := runtime.GOARCH

	switch runtime.GOOS {
	case "windows":
		switch gpuType {
		case utils.GPUTypeNvidia + "," + utils.GPUTypeAmd:
			return fmt.Sprintf("%s%s/%s/ollama-windows-amd64-all.zip", baseDownloadURL, urlDirPathWindows, version)
		case utils.GPUTypeNvidia:
			return fmt.Sprintf("%s%s/%s/ollama-windows-amd64.zip", baseDownloadURL, urlDirPathWindows, version)
		case utils.GPUTypeAmd:
			return fmt.Sprintf("%s%s/%s/ollama-windows-amd64-rocm.zip", baseDownloadURL, urlDirPathWindows, version)
		case utils.GPUTypeIntelArc:
			return fmt.Sprintf("%s%s/%s/ipex-llm-ollama.zip", baseDownloadURL, urlDirPathWindows, version)
		default:
			return fmt.Sprintf("%s%s/%s/ollama-windows-amd64-base.zip", baseDownloadURL, urlDirPathWindows, version)
		}

	case "linux":
		switch gpuType {
		case utils.GPUTypeNvidia:
			if arch == "arm64" {
				return fmt.Sprintf("%s%s/%s/ollama-cuda-linux-arm64.tgz", baseDownloadURL, urlDirPathLinux, version)
			}
			return fmt.Sprintf("%s%s/%s/ollama-cuda-linux-amd64.tgz", baseDownloadURL, urlDirPathLinux, version)
		case utils.GPUTypeAmd:
			if arch == "arm64" {
				return fmt.Sprintf("%s%s/%s/ollama-linux-arm64.tgz", baseDownloadURL, urlDirPathLinux, version)
			}
			return fmt.Sprintf("%s%s/%s/ollama-rocm-linux-amd64.tgz", baseDownloadURL, urlDirPathLinux, version)
		case utils.GPUTypeNvidia + "," + utils.GPUTypeAmd:
			if arch == "arm64" {
				return fmt.Sprintf("%s%s/%s/ollama-cuda-linux-arm64.tgz", baseDownloadURL, urlDirPathLinux, version)
			}
			return fmt.Sprintf("%s%s/%s/ollama-cuda-linux-amd64.tgz", baseDownloadURL, urlDirPathLinux, version)
		case utils.GPUTypeIntelArc:
			if arch == "arm64" {
				return fmt.Sprintf("%s%s/%s/ollama-linux-arm64.tgz", baseDownloadURL, urlDirPathLinux, version)
			}
			return fmt.Sprintf("%s%s/%s/ollama-linux-amd64.tgz", baseDownloadURL, urlDirPathLinux, version)
		default:
			if arch == "arm64" {
				return fmt.Sprintf("%s%s/%s/ollama-linux-arm64.tgz", baseDownloadURL, urlDirPathLinux, version)
			}
			return fmt.Sprintf("%s%s/%s/ollama-linux-amd64.tgz", baseDownloadURL, urlDirPathLinux, version)
		}

	case "darwin":
		return fmt.Sprintf("%s%s/Ollama-darwin.zip", baseDownloadURL, urlDirPathMacOS)

	default:
		return ""
	}
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getAOGDataDir gets AOG unified data directory (consistent with built-in engine)
// This allows sharing storage space with built-in ollama engine
func getAOGDataDir() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "linux":
		// Linux: /var/lib/aog
		dir = "/var/lib/aog"
	case "darwin":
		// macOS: ~/Library/Application Support/AOG
		home := os.Getenv("HOME")
		if home == "" {
			return "", fmt.Errorf("HOME environment variable not set")
		}
		dir = filepath.Join(home, "Library", "Application Support", "AOG")
	case "windows":
		// Windows: %LOCALAPPDATA%/AOG
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = os.Getenv("APPDATA")
		}
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA/APPDATA environment variable not set")
		}
		dir = filepath.Join(localAppData, "AOG")
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	return dir, nil
}
