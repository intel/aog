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

// Plugin types
const (
	// PluginTypeLocal represents local engine plugins (manage local AI engines).
	PluginTypeLocal = "local"

	// PluginTypeRemote represents remote API plugins (integrate with cloud AI services).
	PluginTypeRemote = "remote"
)

// Protocol types
const (
	ProtocolHTTP      = "http"
	ProtocolHTTPS     = "https"
	ProtocolGRPC      = "grpc"
	ProtocolGRPCWeb   = "grpc-web"
	ProtocolWebSocket = "websocket"
	ProtocolWSS       = "wss"
)

// Service types
const (
	ServiceTypeChat         = "chat"
	ServiceTypeEmbed        = "embed"
	ServiceTypeGenerate     = "generate"
	ServiceTypeTextToImage  = "text-to-image"
	ServiceTypeImageToImage = "image-to-image"
	ServiceTypeImageToVideo = "image-to-video"
	ServiceTypeTextToVideo  = "text-to-video"
	ServiceTypeTextToSpeech = "text-to-speech"
	ServiceTypeSpeechToText = "speech-to-text"
	ServiceTypeVision       = "vision"
)

// Authentication types
const (
	AuthTypeNone   = "none"
	AuthTypeAPIKey = "apikey"
	AuthTypeToken  = "token"
	AuthTypeOAuth  = "oauth"
	AuthTypeSign   = "sign"
)

// GPU types
const (
	GPUTypeNone     = "none"
	GPUTypeNvidia   = "nvidia"
	GPUTypeAmd      = "amd"
	GPUTypeIntelArc = "intel_arc"
)

// Error codes
const (
	// General error codes
	ErrCodeSuccess            = 0
	ErrCodeUnknown            = 1
	ErrCodeInvalidArgument    = 2
	ErrCodeNotFound           = 3
	ErrCodeAlreadyExists      = 4
	ErrCodePermissionDenied   = 5
	ErrCodeResourceExhausted  = 6
	ErrCodeFailedPrecondition = 7
	ErrCodeAborted            = 8
	ErrCodeOutOfRange         = 9
	ErrCodeUnimplemented      = 10
	ErrCodeInternal           = 11
	ErrCodeUnavailable        = 12
	ErrCodeDataLoss           = 13
	ErrCodeUnauthenticated    = 14

	// Plugin-specific error codes (1000-1999)
	ErrCodePluginNotFound          = 1000
	ErrCodePluginLoadFailed        = 1001
	ErrCodePluginInvalidConfig     = 1002
	ErrCodePluginStartFailed       = 1003
	ErrCodePluginStopFailed        = 1004
	ErrCodePluginHealthCheckFailed = 1005

	// Engine-specific error codes (2000-2999)
	ErrCodeEngineNotFound          = 2000
	ErrCodeEngineNotInstalled      = 2001
	ErrCodeEngineStartFailed       = 2002
	ErrCodeEngineStopFailed        = 2003
	ErrCodeEngineInstallFailed     = 2004
	ErrCodeEngineUpgradeFailed     = 2005
	ErrCodeEngineHealthCheckFailed = 2006

	// Model-specific error codes (3000-3999)
	ErrCodeModelNotFound      = 3000
	ErrCodeModelPullFailed    = 3001
	ErrCodeModelDeleteFailed  = 3002
	ErrCodeModelLoadFailed    = 3003
	ErrCodeModelUnloadFailed  = 3004
	ErrCodeModelInvalidFormat = 3005

	// Service invocation error codes (4000-4999)
	ErrCodeServiceNotFound                  = 4000
	ErrCodeServiceUnavailable               = 4001
	ErrCodeServiceTimeout                   = 4002
	ErrCodeServiceInvalidRequest            = 4003
	ErrCodeServiceInvalidResponse           = 4004
	ErrCodeServiceStreamingNotSupported     = 4005
	ErrCodeServiceBidirectionalNotSupported = 4006
)

// PluginError represents a plugin error.
type PluginError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *PluginError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

// NewPluginError creates a new plugin error.
func NewPluginError(code int, message string, details string) *PluginError {
	return &PluginError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// PluginStatus represents the status of a plugin.
type PluginStatus struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"` // running, stopped, error
	Message string `json:"message,omitempty"`
}

// EngineStatus represents the status of an engine.
type EngineStatus struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"` // running, stopped, installing, error
	Message string `json:"message,omitempty"`
}

// Operation status constants
const (
	StatusRunning    = "running"
	StatusStopped    = "stopped"
	StatusInstalling = "installing"
	StatusError      = "error"
	StatusUnknown    = "unknown"
)

// StreamChunk represents a chunk in streaming response (e.g., SSE, NDJSON).
type StreamChunk struct {
	Data     []byte            // chunk data
	Error    error             // error information
	IsFinal  bool              // whether this is the final chunk
	Metadata map[string]string // metadata (e.g., content-type)
}
