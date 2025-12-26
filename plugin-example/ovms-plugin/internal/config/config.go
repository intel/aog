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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	sdktypes "github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/utils"
)

// Config wraps plugin manifest and derived engine config.
type Config struct {
	Manifest     *sdktypes.PluginManifest
	EngineHost   string
	DataDir      string
	EngineDir    string
	ExecDir      string
	DownloadDir  string
	OVMSHost     string
	OVMSHTTPPort int // HTTP port for chat/generate/embed/rerank (16666)
	OVMSGRPCPort int // gRPC port for speech-to-text/text-to-speech (9000)
}

func LoadConfig() (*Config, error) {
	pluginDir, err := getPluginDir()
	if err != nil {
		return nil, fmt.Errorf("get plugin dir failed: %w", err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin.yaml failed: %w", err)
	}

	var manifest sdktypes.PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal plugin.yaml failed: %w", err)
	}

	cfg := &Config{Manifest: &manifest}

	cfg.EngineHost = manifest.Provider.EngineHost

	// Get data directory from manifest or use default AOG data directory
	dataDir := os.ExpandEnv(manifest.Resources.DataDir)
	if dataDir == "" || dataDir == "/engine/openvino" {
		// AOG_DATA_DIR not set, use platform-specific default
		aogDir, err := utils.GetAOGDataDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get AOG data directory: %w", err)
		}
		dataDir = filepath.Join(aogDir, "engine", "openvino")
	}
	cfg.DataDir = dataDir

	// Ensure data directory exists before validation
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Parse OVMS-specific resources from custom fields (if available)
	// For now, use defaults based on DataDir

	if cfg.EngineDir == "" {
		cfg.EngineDir = cfg.DataDir
	}
	if cfg.ExecDir == "" {
		// Match built-in OVMS: ExecDir = EngineDir on both Windows and Linux
		// After extraction, OVMS structure will be:
		//   Windows: EngineDir/ovms.exe, EngineDir/setupvars.bat, EngineDir/python/*
		//   Linux: EngineDir/ovms/bin/ovms, EngineDir/ovms/lib/*, etc.
		cfg.ExecDir = cfg.EngineDir
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = filepath.Join(os.TempDir(), "aog-openvino-downloads")
	}

	// Initialize OVMS connection settings
	cfg.OVMSHost = os.Getenv("OVMS_HOST")
	if cfg.OVMSHost == "" {
		cfg.OVMSHost = "localhost"
	}

	// HTTP port for chat/generate/embed/rerank services (matching built-in OpenVINO)
	ovmsHTTPPortStr := os.Getenv("OVMS_HTTP_PORT")
	if ovmsHTTPPortStr == "" {
		cfg.OVMSHTTPPort = 16666 // Default HTTP port
	} else {
		if port, err := strconv.Atoi(ovmsHTTPPortStr); err == nil {
			cfg.OVMSHTTPPort = port
		} else {
			cfg.OVMSHTTPPort = 16666
		}
	}

	// gRPC port for speech-to-text/text-to-speech services (matching built-in OpenVINO)
	ovmsGRPCPortStr := os.Getenv("OVMS_GRPC_PORT")
	if ovmsGRPCPortStr == "" {
		cfg.OVMSGRPCPort = 9000 // Default gRPC port
	} else {
		if port, err := strconv.Atoi(ovmsGRPCPortStr); err == nil {
			cfg.OVMSGRPCPort = port
		} else {
			cfg.OVMSGRPCPort = 9000
		}
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func getPluginDir() (string, error) {
	// First, try current working directory (set by AOG host)
	// AOG sets the working directory to plugin root when launching the plugin
	wd, err := os.Getwd()
	if err == nil {
		manifestPath := filepath.Join(wd, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			return wd, nil
		}
	}

	// Fallback: Search upward from executable directory
	// This handles cases where plugin is run standalone for testing
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	currentDir := filepath.Dir(exePath)
	for i := 0; i < 5; i++ { // Search up to 5 levels
		manifestPath := filepath.Join(currentDir, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // Reached filesystem root
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("plugin.yaml not found (checked working dir and parent directories of %s)", exePath)
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if cfg.Manifest == nil {
		return fmt.Errorf("manifest is required")
	}

	if cfg.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}

	// Check if data directory is writable
	testFile := filepath.Join(cfg.DataDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		return fmt.Errorf("data directory is not writable: %w", err)
	}
	os.Remove(testFile) // Clean up test file

	if cfg.OVMSHTTPPort <= 0 || cfg.OVMSHTTPPort > 65535 {
		return fmt.Errorf("invalid OVMS HTTP port: %d", cfg.OVMSHTTPPort)
	}

	if cfg.OVMSGRPCPort <= 0 || cfg.OVMSGRPCPort > 65535 {
		return fmt.Errorf("invalid OVMS gRPC port: %d", cfg.OVMSGRPCPort)
	}

	if cfg.OVMSHost == "" {
		return fmt.Errorf("OVMS host is required")
	}

	return nil
}
