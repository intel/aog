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

package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginManifest defines the complete metadata for a plugin.
// This is the core SDK type that defines the structure of plugin.yaml.
type PluginManifest struct {
	Version   string                    `json:"version" yaml:"version"`
	Provider  ProviderInfo              `json:"provider" yaml:"provider"`
	Services  []ServiceDef              `json:"services" yaml:"services"`
	Platforms map[string]PlatformConfig `json:"platforms,omitempty" yaml:"platforms,omitempty"`
	Resources *ResourcesConfig          `json:"resources,omitempty" yaml:"resources,omitempty"`

	// PluginDir is the plugin directory path (injected at runtime, not serialized).
	PluginDir string `json:"-" yaml:"-"`
}

// ProviderInfo defines the basic information about a plugin provider.
type ProviderInfo struct {
	Name        string `json:"name" yaml:"name"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Version     string `json:"version" yaml:"version"`
	Type        string `json:"type" yaml:"type"` // local/remote
	Author      string `json:"author,omitempty" yaml:"author,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	EngineHost  string `json:"engine_host,omitempty" yaml:"engine_host,omitempty"` // Engine base URL (required for local type)
}

// ServiceDef defines a service supported by the plugin.
type ServiceDef struct {
	ServiceName    string   `json:"service_name" yaml:"service_name"`
	TaskType       string   `json:"task_type" yaml:"task_type"`
	Protocol       string   `json:"protocol" yaml:"protocol"`
	ExposeProtocol string   `json:"expose_protocol" yaml:"expose_protocol"`
	Endpoint       string   `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	AuthType       string   `json:"auth_type,omitempty" yaml:"auth_type,omitempty"`
	DefaultModel   string   `json:"default_model,omitempty" yaml:"default_model,omitempty"`
	SupportModels  []string `json:"support_models,omitempty" yaml:"support_models,omitempty"`

	// ConfigRef references an existing template configuration, format: "flavor:service"
	ConfigRef string `json:"config_ref,omitempty" yaml:"config_ref,omitempty"`

	// Timeout is the service invocation timeout in seconds, 0 means use default.
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`

	Middlewares  []string            `json:"middlewares,omitempty" yaml:"middlewares,omitempty"`
	Capabilities ServiceCapabilities `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

// ServiceCapabilities declares the capabilities of a service.
type ServiceCapabilities struct {
	// SupportStreaming indicates if HTTP protocol supports streaming response.
	SupportStreaming bool `json:"support_streaming,omitempty" yaml:"support_streaming,omitempty"`

	// SupportBidirectional indicates if bidirectional streaming is supported.
	SupportBidirectional bool `json:"support_bidirectional,omitempty" yaml:"support_bidirectional,omitempty"`
}

// PlatformConfig defines platform-specific configuration.
type PlatformConfig struct {
	Executable   string   `json:"executable" yaml:"executable"`
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// ResourcesConfig defines plugin-managed resource configuration.
type ResourcesConfig struct {
	// DataDir is the data root directory, supports environment variable expansion.
	DataDir string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`

	// Ollama contains Ollama-specific resource configuration (optional, depends on plugin type).
	Ollama *OllamaResources `json:"ollama,omitempty" yaml:"ollama,omitempty"`
}

// OllamaResources defines Ollama resource configuration.
type OllamaResources struct {
	Executable  string `json:"executable,omitempty" yaml:"executable,omitempty"`
	ModelsDir   string `json:"models_dir,omitempty" yaml:"models_dir,omitempty"`
	DownloadDir string `json:"download_dir,omitempty" yaml:"download_dir,omitempty"`
}

// LoadManifest loads plugin metadata from the specified directory.
func LoadManifest(pluginDir string) (*PluginManifest, error) {
	yamlPath := filepath.Join(pluginDir, "plugin.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return loadManifestFromYAML(yamlPath)
	}

	jsonPath := filepath.Join(pluginDir, "plugin.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return loadManifestFromJSON(jsonPath)
	}

	return nil, fmt.Errorf("plugin manifest not found in %s", pluginDir)
}

func loadManifestFromYAML(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	return &manifest, nil
}

func loadManifestFromJSON(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	return &manifest, nil
}

// SaveManifest saves the plugin metadata to a file.
func (m *PluginManifest) SaveManifest(pluginDir string, format string) error {
	switch format {
	case "yaml", "yml":
		return m.saveAsYAML(pluginDir)
	case "json":
		return m.saveAsJSON(pluginDir)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func (m *PluginManifest) saveAsYAML(pluginDir string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	path := filepath.Join(pluginDir, "plugin.yaml")
	return os.WriteFile(path, data, 0o644)
}

func (m *PluginManifest) saveAsJSON(pluginDir string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	path := filepath.Join(pluginDir, "plugin.json")
	return os.WriteFile(path, data, 0o644)
}

// GetPlatformConfig returns the configuration for the specified platform.
func (m *PluginManifest) GetPlatformConfig(goos, goarch string) (*PlatformConfig, error) {
	platformKey := fmt.Sprintf("%s_%s", goos, goarch)

	config, exists := m.Platforms[platformKey]
	if !exists {
		return nil, fmt.Errorf("no configuration for platform %s", platformKey)
	}

	return &config, nil
}

// GetServiceByName returns the service definition by name.
func (m *PluginManifest) GetServiceByName(serviceName string) (*ServiceDef, error) {
	for i := range m.Services {
		if m.Services[i].ServiceName == serviceName {
			return &m.Services[i], nil
		}
	}
	return nil, fmt.Errorf("service %s not found", serviceName)
}

// ListServiceNames returns all service names.
func (m *PluginManifest) ListServiceNames() []string {
	names := make([]string, 0, len(m.Services))
	for _, svc := range m.Services {
		names = append(names, svc.ServiceName)
	}
	return names
}

// SupportsStreaming checks if the service supports streaming response.
func (s *ServiceDef) SupportsStreaming() bool {
	protocol := strings.ToUpper(s.Protocol)
	if protocol == "WEBSOCKET" || protocol == "WSS" {
		return true
	}
	return s.Capabilities.SupportStreaming
}

// SupportsBidirectional checks if the service supports bidirectional streaming.
func (s *ServiceDef) SupportsBidirectional() bool {
	protocol := strings.ToUpper(s.ExposeProtocol)
	if protocol == "WEBSOCKET" || protocol == "WSS" {
		return true
	}
	return s.Capabilities.SupportBidirectional
}

// GetInvokeMode returns the recommended invocation mode.
func (s *ServiceDef) GetInvokeMode() string {
	protocol := strings.ToUpper(s.Protocol)
	if protocol == "WEBSOCKET" || protocol == "WSS" {
		return "bidirectional"
	}
	if s.Capabilities.SupportStreaming {
		return "streaming"
	}
	return "unary"
}
