// Package tools defines MCP tool schemas and handlers
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetServicesSchema get services list tool
var GetServicesSchema = &mcp.Tool{
	Name:        "aog_get_services",
	Description: "Get all available AI service lists in AOG, including chat, text-to-image, speech-to-text, embed, etc.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"service_name": {
				Type:        "string",
				Description: "Optional: Specify service name, such as chat, text-to-image, speech-to-text, embed",
			},
		},
	},
}

// GetServiceProvidersSchema get service providers tool
var GetServiceProvidersSchema = &mcp.Tool{
	Name:        "aog_get_service_providers",
	Description: "Get service provider information for specified services, including local and remote providers",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"service_name": {
				Type:        "string",
				Description: "Service name, such as chat, text-to-image, speech-to-text, embed",
			},
			"provider_name": {
				Type:        "string",
				Description: "Optional: Specify provider name",
			},
			"service_source": {
				Type:        "string",
				Enum:        []interface{}{"local", "remote"},
				Description: "Optional: Local or remote service provider",
			},
		},
	},
}

// GetModelsSchema get models list tool
var GetModelsSchema = &mcp.Tool{
	Name:        "aog_get_models",
	Description: "Get list of installed models, can be filtered by service type or provider",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"provider_name": {
				Type:        "string",
				Description: "Optional: Specify service provider name",
			},
			"model_name": {
				Type:        "string",
				Description: "Optional: Specify model name",
			},
			"service_name": {
				Type:        "string",
				Description: "Optional: Specify service type, such as chat, text-to-image, etc.",
			},
		},
	},
}

// GetRecommendedModelsSchema get recommended models tool
var GetRecommendedModelsSchema = &mcp.Tool{
	Name:        "aog_get_recommended_models",
	Description: "Get AOG recommended model list, these models are optimized and tested",
	InputSchema: &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	},
}

// GetSupportedModelsSchema get supported models tool
var GetSupportedModelsSchema = &mcp.Tool{
	Name:        "aog_get_supported_models",
	Description: "Get the list of models supported by the specified service provider.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"service_source": {
				Type:        "string",
				Enum:        []interface{}{"local", "remote"},
				Description: "Optional: Local or remote service provider",
			},
			"flavor": {
				Type:        "string",
				Description: "Optional: API flavor, such as ollama, openai, aliyun, deepseek, etc.",
			},
			"service_name": {
				Type:        "string",
				Description: "Optional: Specify service name",
			},
			"search_name": {
				Type:        "string",
				Description: "Optional: Search model name",
			},
			"page_size": {
				Type:        "integer",
				Description: "Optional: Number of items per page",
			},
			"page": {
				Type:        "integer",
				Description: "Optional: Page number",
			},
			"mine": {
				Type:        "boolean",
				Description: "Optional: Whether to show only my models",
			},
		},
		Required: []string{"service_source", "flavor"},
	},
}

// InstallModelSchema install model tool
var InstallModelSchema = &mcp.Tool{
	Name:        "aog_install_model",
	Description: "Install specified AI model into AOG system",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"model_name": {
				Type:        "string",
				Description: "Required: Model name, such as deepseek-r1:7b, qwen2.5:0.5b, etc.",
			},
			"service_name": {
				Type:        "string",
				Description: "Required: Service name, such as chat, text-to-image, speech-to-text, embed",
			},
			"service_source": {
				Type:        "string",
				Enum:        []interface{}{"local", "remote"},
				Description: "Required: Local or remote service",
			},
			"provider_name": {
				Type:        "string",
				Description: "Optional: Service provider name, use default provider if not specified",
			},
			"size": {
				Type:        "string",
				Description: "Optional: Model size information",
			},
		},
		Required: []string{"model_name", "service_name", "service_source"},
	},
}

// ChatSchema chat tool
var ChatSchema = &mcp.Tool{
	Name:        "aog_chat",
	Description: "Use AOG's chat service for conversations, supports multi-turn dialogue and streaming output. IMPORTANT: The model parameter must exactly match an existing model name from the installed models list (obtainable via aog_get_models). Using a non-existent model name will cause the service to fail.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"messages": {
				Type: "array",
				Items: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"role": {
							Type:        "string",
							Enum:        []interface{}{"user", "assistant", "system"},
							Description: "Message role",
						},
						"content": {
							Type:        "string",
							Description: "Message content",
						},
					},
					Required: []string{"role", "content"},
				},
				Description: "List of conversation messages",
			},
			"model": {
				Type:        "string",
				Description: "Optional: Specify model name. Must exactly match model_name from the installed models list (obtainable via aog_get_models). If not specified or empty, the model parameter will not be passed and the service default model will be used. Passing a non-existent model name will cause service error",
			},
			"temperature": {
				Type:        "number",
				Minimum:     func() *float64 { v := 0.0; return &v }(),
				Maximum:     func() *float64 { v := 2.0; return &v }(),
				Description: "Optional: Temperature parameter, controls output randomness",
			},
			"max_tokens": {
				Type:        "integer",
				Minimum:     func() *float64 { v := 1.0; return &v }(),
				Description: "Optional: Maximum number of tokens",
			},
			"stream": {
				Type:        "boolean",
				Description: "Optional: Whether to use streaming output, default false",
			},
		},
		Required: []string{"messages"},
	},
}

// TextToImageSchema text-to-image tool
var TextToImageSchema = &mcp.Tool{
	Name:        "aog_text_to_image",
	Description: "Use AOG's text-to-image service to generate images from text descriptions. IMPORTANT: The model parameter must exactly match an existing model name from the installed models list (obtainable via aog_get_models). Using a non-existent model name will cause the service to fail.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"prompt": {
				Type:        "string",
				Description: "Required: Image description prompt, describing the content of the image to be generated",
			},
			"model": {
				Type:        "string",
				Description: "Optional: Specify model name. Must exactly match model_name from the installed models list (obtainable via aog_get_models). If not specified or empty, the model parameter will not be passed and the service default model will be used. Passing a non-existent model name will cause service error",
			},
			"n": {
				Type:        "integer",
				Minimum:     func() *float64 { v := 1.0; return &v }(),
				Maximum:     func() *float64 { v := 4.0; return &v }(),
				Description: "Optional: Number of images to generate, default 1, maximum 4",
			},
			"size": {
				Type:        "string",
				Enum:        []interface{}{"512*512", "1024*1024", "2048*2048"},
				Description: "Optional: Image size, default 512*512",
			},
			"seed": {
				Type:        "integer",
				Description: "Optional: Random seed for deterministic results",
			},
		},
		Required: []string{"prompt"},
	},
}

// SpeechToTextSchema speech-to-text tool
var SpeechToTextSchema = &mcp.Tool{
	Name:        "aog_speech_to_text",
	Description: "Use AOG's speech-to-text service to convert audio to text. IMPORTANT: The model parameter must exactly match an existing model name from the installed models list (obtainable via aog_get_models). Using a non-existent model name will cause the service to fail.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"audio": {
				Type:        "string",
				Description: "Required: Audio file path or base64 encoded audio data",
			},
			"model": {
				Type:        "string",
				Description: "Optional: Specify model name. Must exactly match model_name from the installed models list (obtainable via aog_get_models). If not specified or empty, the model parameter will not be passed and the service default model will be used. Passing a non-existent model name will cause service error",
			},
			"language": {
				Type:        "string",
				Description: "Optional: Audio language, such as zh, en, etc.",
			},
		},
		Required: []string{"audio"},
	},
}

// EmbedSchema text embedding tool
var EmbedSchema = &mcp.Tool{
	Name:        "aog_embed",
	Description: "Use AOG's embedding service to generate text vector representations. Returns complete data structure containing embedding vector arrays, can be used for semantic search, similarity calculation and other tasks. IMPORTANT: The model parameter must exactly match an existing model name from the installed models list (obtainable via aog_get_models). Using a non-existent model name will cause the service to fail.",
	InputSchema: &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"input": {
				Type:        "string",
				Description: "Required: Text content to be embedded",
			},
			"model": {
				Type:        "string",
				Description: "Optional: Specify model name. Must exactly match model_name from the installed models list (obtainable via aog_get_models). If not specified or empty, the model parameter will not be passed and the service default model will be used. Passing a non-existent model name will cause service error",
			},
		},
		Required: []string{"input"},
	},
}

// HealthCheckSchema health check tool
var HealthCheckSchema = &mcp.Tool{
	Name:        "aog_health_check",
	Description: "Check AOG service health status, confirm whether the service is running normally",
	InputSchema: &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	},
}

// GetVersionSchema get version tool
var GetVersionSchema = &mcp.Tool{
	Name:        "aog_get_version",
	Description: "Get AOG version information",
	InputSchema: &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	},
}

// AllToolSchemas all tool schemas
var AllToolSchemas = []*mcp.Tool{
	GetServicesSchema,
	GetServiceProvidersSchema,
	GetModelsSchema,
	GetRecommendedModelsSchema,
	GetSupportedModelsSchema,
	InstallModelSchema,
	ChatSchema,
	TextToImageSchema,
	SpeechToTextSchema,
	EmbedSchema,
	HealthCheckSchema,
	GetVersionSchema,
}
