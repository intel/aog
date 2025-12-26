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
	"time"
)

type LocalTime time.Time

// ModelInfo represents model information.
type ModelInfo struct {
	Name       string        `json:"name"`
	Model      string        `json:"model"`
	ModifiedAt time.Time     `json:"modified_at"`
	Size       int64         `json:"size"`
	Digest     string        `json:"digest,omitempty"`
	Details    *ModelDetails `json:"details,omitempty"`
}

// ModelDetails contains detailed model information.
type ModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ListModelsResponse is the response for listing models.
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// PullModelRequest is the request for pulling a model.
type PullModelRequest struct {
	Model     string `json:"model"`
	Insecure  bool   `json:"insecure,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Stream    *bool  `json:"stream,omitempty"`
	Name      string `json:"name,omitempty"`       // alias (for backward compatibility)
	ModelType string `json:"model_type,omitempty"` // model type (OpenVINO-specific)
}

// DeleteModelRequest is the request for deleting a model.
type DeleteModelRequest struct {
	Model string `json:"model"`
}

// LoadModelRequest is the request for loading a model.
type LoadModelRequest struct {
	Model string `json:"model"`
}

// UnloadModelRequest is the request for unloading models.
type UnloadModelRequest struct {
	Models []string `json:"models"`
}

// ProgressResponse represents progress for operations like model downloads.
type ProgressResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

// EngineConfig represents basic engine configuration.
type EngineConfig struct {
	Host         string `json:"host"`
	Scheme       string `json:"scheme"` // http/https
	EnginePath   string `json:"engine_path"`
	ExecFile     string `json:"exec_file"`
	ExecPath     string `json:"exec_path"`
	DownloadURL  string `json:"download_url"`
	DownloadPath string `json:"download_path"`
	DeviceType   string `json:"device_type,omitempty"` // GPU type
}

// EngineRecommendConfig represents extended engine configuration.
type EngineRecommendConfig struct {
	Host           string `json:"host"`
	Origin         string `json:"origin"`
	Scheme         string `json:"scheme"`
	RecommendModel string `json:"recommend_model"`
	DownloadUrl    string `json:"download_url"`
	DownloadPath   string `json:"download_path"`
	EnginePath     string `json:"engine_path"`
	ExecPath       string `json:"exec_path"`
	ExecFile       string `json:"exec_file"`
	DeviceType     string `json:"device_type"`
}

// EngineVersionResponse represents the engine version response.
type EngineVersionResponse struct {
	Version string `json:"version"`
}

// InvokeRequest represents a plugin service invocation request.
type InvokeRequest struct {
	ServiceName string            `json:"service_name"`
	RequestBody []byte            `json:"request_body"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// InvokeResponse represents a plugin service invocation response.
type InvokeResponse struct {
	ResponseBody []byte            `json:"response_body"`
	Headers      map[string]string `json:"headers,omitempty"`
	StatusCode   int               `json:"status_code,omitempty"`
}

// HealthStatus represents the health status.
type HealthStatus struct {
	Status  string            `json:"status"` // UP, DOWN, UNKNOWN
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// Credentials represents authentication credentials.
type Credentials struct {
	Type   string            `json:"type"`   // apikey, token, oauth, etc.
	Values map[string]string `json:"values"` // key-value pairs
}

type RecommendModelData struct {
	Id              string    `json:"id"`
	Service         string    `json:"service_name"`
	ApiFlavor       string    `json:"api_flavor"`
	Flavor          string    `json:"flavor"`
	Method          string    `json:"method" default:"POST"`
	Desc            string    `json:"desc"`
	Url             string    `json:"url"`
	AuthType        string    `json:"auth_type"`
	AuthApplyUrl    string    `json:"auth_apply_url"`
	AuthFields      []string  `json:"auth_fields"`
	Name            string    `json:"name"`
	ServiceProvider string    `json:"service_provider_name"`
	Size            string    `json:"size"`
	IsRecommended   bool      `json:"is_recommended" default:"false"`
	Status          string    `json:"status"`
	Avatar          string    `json:"avatar"`
	CanSelect       bool      `json:"can_select" default:"false"`
	Class           []string  `json:"class"`
	OllamaId        string    `json:"ollama_id"`
	ParamsSize      float32   `json:"params_size"`
	InputLength     int       `json:"input_length"`
	OutputLength    int       `json:"output_length"`
	Source          string    `json:"source"`
	IsDefault       string    `json:"is_default" default:"false"`
	Think           bool      `json:"think"`
	ThinkSwitch     bool      `json:"think_switch"`
	Tools           bool      `json:"tools"` // 是否支持工具调用
	Context         float32   `json:"context"`
	CreatedAt       LocalTime `json:"created_at"`
}

// ListResponse is an alias of ListModelsResponse for backward compatibility.
type ListResponse = ListModelsResponse

// DeleteRequest is an alias of DeleteModelRequest for backward compatibility.
type DeleteRequest = DeleteModelRequest

// LoadRequest is an alias of LoadModelRequest for backward compatibility.
type LoadRequest = LoadModelRequest

// PullProgressFunc is a progress callback function for LocalPluginProvider.PullModel
// to provide streaming progress notifications.
type PullProgressFunc func(ProgressResponse) error
