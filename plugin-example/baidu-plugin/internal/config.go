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
	"time"

	"gopkg.in/yaml.v3"

	"github.com/intel/aog/plugin/examples/baidu-plugin/internal/utils"
)

// Config holds the Baidu plugin configuration
type Config struct {
	YamlConfig
	// Request timeout duration
	Timeout time.Duration
}

type YamlConfig struct {
	Url      string              `json:"engine_host"`
	Services []YamlConfigService `json:"services"`
}

type YamlConfigService struct {
	ServiceName    string   `json:"service_name"`
	TaskType       string   `json:"task_type"`
	Protocol       string   `json:"protocol"`
	ExposeProtocol string   `json:"expose_protocol"`
	Endpoint       string   `json:"endpoint"`
	AuthType       string   `json:"auth_type"`
	AuthFields     []string `json:"auth_fields"`
	SpecialUrl     string   `json:"special_url"`
	ExtraUrl       string   `json:"extra_url"`
	ExtraHeaders   string   `json:"extra_headers"`
	DefaultModel   string   `json:"default_model"`
	SupportModels  []string `json:"support_models"`
}

// LoadConfig loads configuration and initializes plugin-managed paths
func LoadConfig() (*Config, error) {
	allConfig, err := loadPluginYamlAndConfig()
	if err != nil {
		return nil, err
	}
	config := &Config{
		YamlConfig: *allConfig,
		Timeout:    30 * time.Second,
	}
	return config, nil
}

func loadPluginYamlAndConfig() (*YamlConfig, error) {
	pluginDir, err := utils.GetPluginDir()
	if err != nil {
		return nil, err
	}
	yamlFile := filepath.Join(pluginDir, "plugin.yaml")
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		// If the file does not exist, fall back to defaults
		return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
	}
	var yamlConfigInfo YamlConfig
	if err := yaml.Unmarshal(data, &yamlConfigInfo); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.yaml: %w", err)
	}
	return &yamlConfigInfo, nil
}
