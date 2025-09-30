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

package dto

import (
	"time"

	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
)

type GetProductInfoResponse struct {
	Icon        string `json:"icon"`
	ProductName string `json:"productname"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type CreateAIGCServiceRequest struct {
	ServiceName   string `json:"service_name" validate:"required,supported_service"`
	ServiceSource string `json:"service_source" validate:"omitempty,supported_service_source"`
	ApiFlavor     string `json:"api_flavor" validate:"omitempty,supported_flavor"`
	ProviderName  string `json:"provider_name" validate:"omitempty,max=200"`
	Desc          string `json:"desc" validate:"omitempty,max=500"`
	Method        string `json:"method" validate:"omitempty,supported_http_method"`
	Url           string `json:"url" validate:"omitempty,url"`
	AuthType      string `json:"auth_type" validate:"omitempty,supported_auth_type"`
	AuthKey       string `json:"auth_key" validate:"required_with_auth"`
	ExtraHeaders  string `json:"extra_headers" validate:"omitempty,json_format"`
	ExtraJsonBody string `json:"extra_json_body" validate:"omitempty,json_format"`
	Properties    string `json:"properties" validate:"omitempty,json_format"`
	SkipModelFlag bool   `json:"skip_model"`
	ModelName     string `json:"model_name" validate:"omitempty,min=1,max=200"`
}

// SetDefaults implements RequestDefaultSetter interface
func (r *CreateAIGCServiceRequest) SetDefaults() {
	if r.ServiceSource == "" {
		r.ServiceSource = types.ServiceSourceLocal
	}

	if utils.Contains(types.SupportOnlyRemoteService, r.ServiceName) {
		r.ServiceSource = types.ServiceSourceRemote
	}

	if r.ApiFlavor == "" {
		recommendConfig := r.getRecommendConfig()
		r.ApiFlavor = recommendConfig.ModelEngine
	}

	if r.AuthType == "" {
		r.AuthType = types.AuthTypeNone
	}
}

// getRecommendConfig gets recommended configuration
func (r *CreateAIGCServiceRequest) getRecommendConfig() types.RecommendConfig {
	switch r.ServiceName {
	case types.ServiceChat:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   constants.DefaultChatModelName,
		}
	case types.ServiceEmbed:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   constants.DefaultEmbedModelName,
		}
	case types.ServiceModels:
		return types.RecommendConfig{}
	case types.ServiceGenerate:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   constants.DefaultChatModelName,
		}
	case types.ServiceTextToImage:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultTextToImageModel,
		}
	case types.ServiceSpeechToText:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultSpeechToTextModel,
		}
	case types.ServiceSpeechToTextWS:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultSpeechToTextModel,
		}
	case types.ServiceTextToSpeech:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultTextToSpeechModel,
		}
	case types.ServiceImageToImage:
		return types.RecommendConfig{
			ModelEngine: types.FlavorAliYun,
			ModelName:   constants.DefaultImageToImageModel,
		}
	case types.ServiceImageToVideo:
		return types.RecommendConfig{
			ModelEngine: types.FlavorAliYun,
			ModelName:   constants.DefaultImageToVideoModel,
		}
	default:
		return types.RecommendConfig{}
	}
}

type UpdateAIGCServiceRequest struct {
	ServiceName  string `json:"service_name" validate:"required,supported_service"`
	HybridPolicy string `json:"hybrid_policy" validate:"omitempty,supported_hybrid_policy"`
}

// SetDefaults implements RequestDefaultSetter interface
func (r *UpdateAIGCServiceRequest) SetDefaults() {
	if r.HybridPolicy == "" {
		r.HybridPolicy = types.HybridPolicyDefault
	}
}

type DeleteAIGCServiceRequest struct{}

type GetAIGCServiceRequest struct{}

type ExportServiceRequest struct {
	ServiceName  string `json:"service_name" validate:"omitempty,supported_service"`
	ProviderName string `json:"provider_name" validate:"omitempty,max=200"`
	ModelName    string `json:"model_name" validate:"omitempty,max=200"`
}

type ExportServiceResponse struct {
	Version          string                          `json:"version"`
	Services         map[string]ServiceEntry         `json:"services"`
	ServiceProviders map[string]ServiceProviderEntry `json:"service_providers"`
}
type ServiceEntry struct {
	ServiceProviders ServiceProviderInfo `json:"service_providers"`
	HybridPolicy     string              `json:"hybrid_policy"`
}
type ServiceProviderInfo struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
}
type ServiceProviderEntry struct {
	ServiceName   string   `json:"service_name"`
	ServiceSource string   `json:"service_source"`
	Desc          string   `json:"desc"`
	APIFlavor     string   `json:"api_flavor"`
	Method        string   `json:"method"`
	URL           string   `json:"url"`
	AuthType      string   `json:"auth_type"`
	AuthKey       string   `json:"auth_key"`
	Models        []string `json:"models"`
}

type ImportServiceRequest struct {
	Version          string                          `json:"version"`
	Services         map[string]ServiceEntry         `json:"services"`
	ServiceProviders map[string]ServiceProviderEntry `json:"service_providers"`
}

// SetDefaults implements RequestDefaultSetter interface
func (r *ImportServiceRequest) SetDefaults() {
	for providerName, provider := range r.ServiceProviders {
		if provider.AuthType == "" {
			provider.AuthType = types.AuthTypeNone
		}

		if provider.Method == "" {
			provider.Method = "POST"
		}

		if utils.Contains(types.SupportOnlyRemoteService, provider.ServiceName) {
			provider.ServiceSource = types.ServiceSourceRemote
		} else if provider.ServiceSource == "" {
			provider.ServiceSource = types.ServiceSourceLocal
		}

		r.ServiceProviders[providerName] = provider
	}
}

type ImportServiceResponse struct {
	bcode.Bcode
}

type GetAIGCServicesRequest struct {
	ServiceName string `json:"service_name,omitempty" validate:"omitempty,supported_service"`
}

type CreateAIGCServiceResponse struct {
	bcode.Bcode
}

type UpdateAIGCServiceResponse struct {
	bcode.Bcode
}

type DeleteAIGCServiceResponse struct{}

type GetAIGCServiceResponse struct{}

type GetAIGCServicesResponse struct {
	bcode.Bcode
	Data []Service `json:"data"`
}

type Service struct {
	ServiceName  string    `json:"service_name"`
	HybridPolicy string    `json:"hybrid_policy"`
	Status       int       `json:"status"`
	Avatar       string    `json:"avatar"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateModelRequest struct {
	ProviderName  string `json:"provider_name" validate:"required"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name" validate:"required"`
	ServiceSource string `json:"service_source" validate:"required"`
	Size          string `json:"size"`
}

type CreateModelStreamRequest struct {
	ProviderName  string `json:"provider_name"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name"`
	ServiceSource string `json:"service_source"`
}

type SelectDefaultModelRequest struct {
	ProviderName  string `json:"provider_name"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name"`
	ServiceSource string `json:"service_source"`
}
type DeleteModelRequest struct {
	ProviderName  string `json:"provider_name"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name" validate:"required"`
	ServiceSource string `json:"service_source" validate:"required"`
}

type GetModelsRequest struct {
	ProviderName string `form:"provider_name,omitempty"`
	ModelName    string `form:"model_name,omitempty"`
	ServiceName  string `form:"service_name,omitempty"`
}

type GetModelListRequest struct {
	ServiceSource string `form:"service_source" validate:"required"`
	Flavor        string `form:"flavor" validate:"required"`
}

type ModelStreamCancelRequest struct {
	ModelName string `json:"model_name" validate:"required"`
}

type CreateModelResponse struct {
	bcode.Bcode
}

type DeleteModelResponse struct {
	bcode.Bcode
}

type GetModelsResponse struct {
	bcode.Bcode
	Data []Model `json:"data"`
}

type RecommendModelResponse struct {
	bcode.Bcode
	Data map[string][]RecommendModelData `json:"data"`
}

type ModelStreamCancelResponse struct {
	bcode.Bcode
}

type Model struct {
	ModelName     string    `json:"model_name"`
	Avatar        string    `json:"avatar"`
	ProviderName  string    `json:"provider_name"`
	Status        string    `json:"status"`
	ServiceName   string    `json:"service_name"`
	ServiceSource string    `json:"service_source"`
	IsDefault     bool      `json:"is_default"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SetDefaultModelRequest struct {
	ProviderName  string `json:"provider_name"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name" validate:"required"`
	ServiceSource string `json:"service_source" validate:"required"`
	IsDefault     bool   `json:"is_default"`
}

type LocalSupportModelData struct {
	OllamaId    string   `json:"id"`
	Name        string   `json:"name"`
	Avatar      string   `json:"avatar"`
	Description string   `json:"description"`
	Class       []string `json:"class"`
	Flavor      string   `json:"provider"`
	Size        string   `json:"size"`
	ParamsSize  float32  `json:"params_size"`
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
	CreatedAt       time.Time `json:"created_at"`
}

type CreateServiceProviderRequest struct {
	ServiceName   string   `json:"service_name" validate:"required"`
	ServiceSource string   `json:"service_source" validate:"required"`
	ApiFlavor     string   `json:"api_flavor" validate:"required"`
	ProviderName  string   `json:"provider_name" validate:"required"`
	Desc          string   `json:"desc"`
	Method        string   `json:"method"`
	Url           string   `json:"url"`
	AuthType      string   `json:"auth_type"`
	AuthKey       string   `json:"auth_key"`
	Models        []string `json:"models"`
	ExtraHeaders  string   `json:"extra_headers"`
	ExtraJsonBody string   `json:"extra_json_body"`
	Properties    string   `json:"properties"`
}

type UpdateServiceProviderRequest struct {
	ProviderName  string   `json:"provider_name" validate:"required"`
	ServiceName   string   `json:"service_name"`
	ServiceSource string   `json:"service_source"`
	ApiFlavor     string   `json:"api_flavor"`
	Desc          string   `json:"desc"`
	Method        string   `json:"method"`
	Url           string   `json:"url"`
	AuthType      string   `json:"auth_type"`
	AuthKey       string   `json:"auth_key"`
	Models        []string `json:"models"`
	ExtraHeaders  string   `json:"extra_headers"`
	ExtraJsonBody string   `json:"extra_json_body"`
	Properties    string   `json:"properties"`
}

type DeleteServiceProviderRequest struct {
	ProviderName string `json:"provider_name" validate:"required"`
}

type GetServiceProviderRequest struct{}

type GetServiceProvidersRequest struct {
	ServiceName   string `json:"service_name,omitempty"`
	ServiceSource string `json:"service_source,omitempty"`
	ProviderName  string `json:"provider_name,omitempty"`
	ApiFlavor     string `json:"api_flavor,omitempty"`
}

type CreateServiceProviderResponse struct {
	bcode.Bcode
}

type UpdateServiceProviderResponse struct {
	bcode.Bcode
}

type DeleteServiceProviderResponse struct {
	bcode.Bcode
}

type GetServiceProviderResponse struct{}

type GetServiceProvidersResponse struct {
	bcode.Bcode
	Data []ServiceProvider `json:"data"`
}

type ServiceProvider struct {
	ProviderName  string    `json:"provider_name"`
	ServiceName   string    `json:"service_name"`
	ServiceSource string    `json:"service_source"`
	Desc          string    `json:"desc"`
	AuthType      string    `json:"auth_type"`
	AuthKey       string    `json:"auth_key"`
	Flavor        string    `json:"flavor"`
	Properties    string    `json:"properties"`
	Models        []string  `json:"models"`
	Status        int       `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ServiceWithModels struct {
	Service Service `json:"service"`
	Models  []Model `json:"models"`
}

type DashboardResponse struct {
	Models   []Model   `json:"models"`
	Services []Service `json:"services"`
}

// control panel model list
type GetSupportModelRequest struct {
	Flavor        string `form:"flavor"`
	ServiceSource string `form:"service_source"`
	ServiceName   string `form:"service_name"`
	PageSize      int    `form:"page_size"`
	Page          int    `form:"page"`
	SearchName    string `form:"search_name"`
}

// control panel paginated model list
type GetSupportModelResponseData struct {
	Data      []RecommendModelData `json:"data"`
	Page      int                  `json:"page"`
	PageSize  int                  `json:"page_size"`
	Total     int                  `json:"total"`
	TotalPage int                  `json:"total_page"`
}
type GetSupportModelResponse struct {
	bcode.Bcode
	Data GetSupportModelResponseData `json:"data"`
}
type SupportModel struct {
	Id            string    `json:"id"`
	OllamaId      string    `json:"Ollama_id"`
	Name          string    `json:"name"`
	Avatar        string    `json:"avatar"`
	Description   string    `json:"description"`
	Class         []string  `json:"class"`
	Flavor        string    `json:"flavor"`
	ApiFlavor     string    `json:"api_flavor"`
	Size          string    `json:"size"`
	ParamSize     float32   `json:"params_size"`
	InputLength   int       `json:"input_length"`
	OutputLength  int       `json:"output_length"`
	ServiceSource string    `json:"service_source"`
	ServiceName   string    `json:"service_name"`
	CreatedAt     time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

type GetModelkeyRequest struct {
	ProviderName  string `json:"provider_name" validate:"required"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name"`
	ServiceSource string `json:"service_source"`
}

type GetModelkeyResponse struct {
	bcode.Bcode
	ModelKey string `json:"model_key"`
}

type RagGetFileRequest struct {
	FileId string `form:"file_id" validate:"required"`
}

type RagGetFileResponse struct {
	bcode.Bcode
	Data types.RagFile `json:"data"`
}
type RagGetFilesResponse struct {
	bcode.Bcode
	Data []types.RagFile `json:"files"`
}

type RagUploadFileResponseData struct {
	FileId string `json:"file_id"`
}

type RagUploadFileResponse struct {
	bcode.Bcode
	Data RagUploadFileResponseData `json:"data"`
}

type RagDeleteFileRequest struct {
	FileId string `form:"file_id" validate:"required"`
}

type RagDeleteFileResponse struct {
	bcode.Bcode
}

type RagRetrievalRequest struct {
	Model   string   `json:"model"`
	FileIDs []string `json:"file_ids"`
	Text    string   `json:"text"`
}

type RagRetrievalResponseData struct {
	Model   string `json:"model"`
	Content string `json:"content"`
}
type RagRetrievalResponse struct {
	bcode.Bcode
	Data RagRetrievalResponseData `json:"data"`
}

type GetVersionResponseData struct {
	Version     string `json:"version"`
	SpecVersion string `json:"spec_version"`
}

type GetVersionResponse struct {
	bcode.Bcode
	Data GetVersionResponseData `json:"data"`
}

type GetEngineVersionRequest struct {
	EngineName string `form:"engine_name"`
}

type GetEngineVersionResponse struct {
	bcode.Bcode
	Data map[string]string `json:"data"`
}

type UpdateAvailableResponseData struct {
	Message string `json:"message"`
	Status  bool   `json:"status"`
}

type UpdateAvailableResponse struct {
	bcode.Bcode
	Data UpdateAvailableResponseData `json:"data"`
}

type UpdateResponse struct {
	bcode.Bcode
	Data struct{} `json:"data"`
}

type GetSeverHealthResponse struct {
	bcode.Bcode
	Data map[string]string `json:"data"`
}

type GetEngineHealthRequest struct {
	EngineName string `form:"engine_name"`
}

type GetEngineHealthResponse struct {
	bcode.Bcode
	Data map[string]string `json:"data"`
}
