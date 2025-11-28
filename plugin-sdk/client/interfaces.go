//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

// Package client provides plugin client interface definitions and implementations.
//
// This package defines client interfaces used by AOG Host to communicate with plugins.
// These interfaces align with those in plugin-sdk/adapter package, but are located in SDK
// to allow Host-side usage without depending on AOG Core internal packages.
package client

import (
	"context"
	"net/http"

	"github.com/intel/aog/plugin-sdk/types"
)

// PluginProvider is the base plugin interface.
//
// All plugins must implement this interface.
// Provides plugin metadata, health checks, and service invocation functionality.
type PluginProvider interface {
	// GetManifest returns the plugin metadata.
	GetManifest() *types.PluginManifest

	// GetOperateStatus returns the operational status.
	GetOperateStatus() int

	// SetOperateStatus sets the operational status.
	SetOperateStatus(status int)

	// HealthCheck performs a health check.
	HealthCheck(ctx context.Context) error

	// InvokeService invokes a plugin service (core method).
	// serviceName: Service name (e.g., "chat", "embed")
	// request: Serialized request data
	// Returns: Serialized response data
	InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error)
}

// LocalPluginProvider is the local plugin interface.
//
// Local plugins require local installation and running of engines (e.g., Ollama, OpenVINO).
// Provides engine lifecycle management and model management functionality.
type LocalPluginProvider interface {
	PluginProvider

	// StartEngine starts the engine.
	StartEngine(mode string) error

	// StopEngine stops the engine.
	StopEngine() error

	// GetConfig returns the engine configuration.
	GetConfig(ctx context.Context) (*types.EngineRecommendConfig, error)

	// CheckEngine checks if the engine is installed.
	CheckEngine() (bool, error)

	// InstallEngine installs the engine.
	InstallEngine(ctx context.Context) error

	// InitEnv initializes the environment.
	InitEnv() error

	// UpgradeEngine upgrades the engine.
	UpgradeEngine(ctx context.Context) error

	// PullModel pulls a model (blocking).
	PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error)

	// PullModelStream pulls a model (streaming).
	PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error)

	// DeleteModel deletes a model.
	DeleteModel(ctx context.Context, req *types.DeleteRequest) error

	// ListModels lists all downloaded models.
	ListModels(ctx context.Context) (*types.ListResponse, error)

	// LoadModel loads a model into memory.
	LoadModel(ctx context.Context, req *types.LoadRequest) error

	// UnloadModel unloads models from memory.
	UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error

	// GetRunningModels returns currently running models.
	GetRunningModels(ctx context.Context) (*types.ListResponse, error)

	// GetVersion returns the engine version information.
	GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error)
}

// RemotePluginProvider is the remote plugin interface.
//
// Remote plugins don't require local engine installation (e.g., OpenAI, Anthropic cloud services).
// Primarily provides authentication management functionality.
type RemotePluginProvider interface {
	PluginProvider

	// SetAuth sets authentication information.
	SetAuth(req *http.Request, authInfo string, credentials map[string]string) error

	// ValidateAuth validates if authentication information is valid.
	ValidateAuth(ctx context.Context) error

	// RefreshAuth refreshes authentication information (for OAuth).
	RefreshAuth(ctx context.Context) error
}

// StreamChunk represents a streaming response data chunk.
//
// Used for data transfer in streaming service calls (e.g., streaming chat).
type StreamChunk struct {
	Data     []byte
	Error    error
	IsFinal  bool
	Metadata map[string]string
}

// BidiMessage represents a bidirectional streaming communication message.
//
// Used for message transfer in bidirectional streaming service calls.
type BidiMessage struct {
	Data        []byte
	MessageType string
	Metadata    map[string]string
	Error       error
}

// StreamablePlugin extends PluginProvider with server-side streaming support.
//
// Plugins implementing this interface can handle streaming service calls (e.g., SSE, NDJSON).
// This is optional - only implement if the plugin declares support_streaming = true in manifest.
type StreamablePlugin interface {
	PluginProvider

	// InvokeServiceStream executes a streaming service request.
	// Returns a channel that sends data chunks and closes when complete.
	// authInfo: Authentication information for the request (can be empty for local plugins)
	InvokeServiceStream(ctx context.Context, serviceName string, authInfo string, request []byte) (<-chan StreamChunk, error)
}

// BidirectionalPlugin extends PluginProvider with bidirectional streaming support.
//
// Plugins implementing this interface can handle WebSocket-like bidirectional communication.
// This is optional - only implement if the plugin declares support_bidirectional = true in manifest.
type BidirectionalPlugin interface {
	PluginProvider

	// InvokeServiceBidirectional handles bidirectional streaming communication.
	// Reads from inStream and writes to outStream until context is cancelled or streams are closed.
	InvokeServiceBidirectional(ctx context.Context, serviceName string, wsConnID string, authInfo string, inStream <-chan BidiMessage, outStream chan<- BidiMessage) error
}
