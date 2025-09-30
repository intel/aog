// Package types defines data types for interacting with AOG API
package types

import "time"

// AOGConfig AOG API basic configuration
type AOGConfig struct {
	BaseURL string `json:"base_url"`
	Version string `json:"version"`
	Timeout int    `json:"timeout"` // milliseconds
}

// ChatMessage chat message
type ChatMessage struct {
	Role    string `json:"role"` // user, assistant, system
	Content string `json:"content"`
}

// Bcode basic response code structure - strictly follows AOG dto definition
type Bcode struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Service discovery and management related types - strictly follows AOG dto definition
type GetAIGCServicesRequest struct {
	ServiceName string `json:"service_name,omitempty"`
}

type GetAIGCServicesResponse struct {
	Bcode
	Data []Service `json:"data"`
}

// Service service information - strictly follows AOG dto definition
type Service struct {
	ServiceName  string    `json:"service_name"`
	HybridPolicy string    `json:"hybrid_policy"`
	Status       int       `json:"status"`
	Avatar       string    `json:"avatar"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Service provider related types - strictly follows AOG dto definition
type GetServiceProvidersRequest struct {
	ServiceName   string `json:"service_name,omitempty"`
	ServiceSource string `json:"service_source,omitempty"`
	ProviderName  string `json:"provider_name,omitempty"`
	ApiFlavor     string `json:"api_flavor,omitempty"`
}

type GetServiceProvidersResponse struct {
	Bcode
	Data []ServiceProvider `json:"data"`
}

// ServiceProvider service provider - strictly follows AOG dto definition
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

// Model management related types - strictly follows AOG dto definition
type GetModelsRequest struct {
	ProviderName string `json:"provider_name,omitempty" form:"provider_name,omitempty"`
	ModelName    string `json:"model_name,omitempty" form:"model_name,omitempty"`
	ServiceName  string `json:"service_name,omitempty" form:"service_name,omitempty"`
}

type GetModelsResponse struct {
	Bcode
	Data []Model `json:"data"`
}

// Model model information - strictly follows AOG dto definition
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

// Recommended model related types - strictly follows AOG dto definition
type RecommendModelResponse struct {
	Bcode
	Data map[string][]RecommendModelData `json:"data"`
}

type RecommendModelData struct {
	Id              string   `json:"id"`
	Service         string   `json:"service_name"`
	ApiFlavor       string   `json:"api_flavor"`
	Flavor          string   `json:"flavor"`
	Method          string   `json:"method" default:"POST"`
	Desc            string   `json:"desc"`
	Url             string   `json:"url"`
	AuthType        string   `json:"auth_type"`
	AuthApplyUrl    string   `json:"auth_apply_url"`
	AuthFields      []string `json:"auth_fields"`
	Name            string   `json:"name"`
	ServiceProvider string   `json:"service_provider_name"`
	Size            string   `json:"size"`
	IsRecommended   bool     `json:"is_recommended" default:"false"`
	Status          string   `json:"status"`
	Avatar          string   `json:"avatar"`
	CanSelect       bool     `json:"can_select" default:"false"`
	Class           []string `json:"class"`
	OllamaId        string   `json:"ollama_id"`
	ParamsSize      float32  `json:"params_size"`
	InputLength     int      `json:"input_length"`
	OutputLength    int      `json:"output_length"`
	Source          string   `json:"source"`
	IsDefault       string   `json:"is_default" default:"false"`
}

type ModelBaseInfo struct {
	ModelName     string `json:"model_name"`
	Avatar        string `json:"avatar"`
	ProviderName  string `json:"provider_name"`
	Status        string `json:"status"`
	ServiceName   string `json:"service_name"`
	ServiceSource string `json:"service_source"`
	IsDefault     string `json:"is_default"`
}

// ChatRequest chat request
type ChatRequest struct {
	Messages    []ChatMessage `json:"messages"`
	Model       string        `json:"model,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      *bool         `json:"stream,omitempty"`
}

// ChatResponse chat response - matches actual AOG API response structure
type ChatResponse struct {
	BusinessCode int    `json:"business_code"`
	Message      string `json:"message"`
	Data         struct {
		CreatedAt    time.Time `json:"created_at"`
		FinishReason string    `json:"finish_reason"`
		Finished     bool      `json:"finished"`
		ID           string    `json:"id"`
		Message      struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
		Model string `json:"model"`
	} `json:"data"`
}

// TextToImageRequest text-to-image request
type TextToImageRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
	N      *int   `json:"n,omitempty"`
	Size   string `json:"size,omitempty"`
	Seed   *int   `json:"seed,omitempty"`
}

// TextToImageResponse text-to-image response - matches actual AOG API response structure
type TextToImageResponse struct {
	BusinessCode int    `json:"business_code,omitempty"`
	Message      string `json:"message,omitempty"`
	Data         struct {
		URL []string `json:"url"`
	} `json:"data"`
	ID string `json:"id,omitempty"`
}

// SpeechToTextRequest speech-to-text request
type SpeechToTextRequest struct {
	Audio    string `json:"audio"`
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
}

// SpeechToTextSegment speech-to-text segment
type SpeechToTextSegment struct {
	ID    int    `json:"id"`
	Start string `json:"start"`
	End   string `json:"end"`
	Text  string `json:"text"`
}

// SpeechToTextResponse speech-to-text response - matches actual AOG API response structure
type SpeechToTextResponse struct {
	BusinessCode int    `json:"business_code,omitempty"`
	Message      string `json:"message,omitempty"`
	Data         struct {
		Segments []SpeechToTextSegment `json:"segments"`
	} `json:"data"`
}

// EmbedRequest text embedding request
type EmbedRequest struct {
	Input string `json:"input"`
	Model string `json:"model,omitempty"`
}

// EmbedData single embedding data item
type EmbedData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbedResponse text embedding response - matches actual AOG API response structure
type EmbedResponse struct {
	BusinessCode int    `json:"business_code"`
	Message      string `json:"message"`
	Data         struct {
		Data  []EmbedData `json:"data"`
		Model string      `json:"model"`
	} `json:"data"`
}

// Model installation related types - strictly follows AOG dto definition
type CreateModelRequest struct {
	ProviderName  string `json:"provider_name"`
	ModelName     string `json:"model_name" validate:"required"`
	ServiceName   string `json:"service_name" validate:"required"`
	ServiceSource string `json:"service_source" validate:"required"`
	Size          string `json:"size"`
}

type CreateModelResponse struct {
	Bcode
}

// Supported model related types - strictly follows AOG dto definition
type GetSupportModelRequest struct {
	Flavor        string `json:"flavor" form:"flavor"`
	ServiceSource string `json:"service_source" form:"service_source" validate:"required"`
	ServiceName   string `json:"service_name" form:"service_name"`
	Mine          bool   `json:"mine" form:"mine" default:"false"`
	PageSize      int    `json:"page_size" form:"page_size"`
	Page          int    `json:"page" form:"page"`
	SearchName    string `json:"search_name" form:"search_name"`
}

type GetSupportModelResponseData struct {
	Data      []RecommendModelData `json:"data"`
	Page      int                  `json:"page"`
	PageSize  int                  `json:"page_size"`
	Total     int                  `json:"total"`
	TotalPage int                  `json:"total_page"`
}

type GetSupportModelResponse struct {
	Bcode
	Data GetSupportModelResponseData `json:"data"`
}

type SupportModelList struct {
	Data []ModelBaseInfo `json:"data"`
}

// AOGResponse generic response
type AOGResponse[T any] struct {
	Data    *T     `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
	Success bool   `json:"success,omitempty"`
}

// HealthResponse health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// VersionResponse version information response
type VersionResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time,omitempty"`
	GitCommit string `json:"git_commit,omitempty"`
}

// ToolResponse tool response
type ToolResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
