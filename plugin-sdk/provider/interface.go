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

package provider

import (
	"context"

	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

// EngineLifecycleManager defines core engine lifecycle operations
type EngineLifecycleManager interface {
	// StartEngine initializes and starts the engine process
	StartEngine(mode string) error

	// StopEngine gracefully stops the engine process
	StopEngine() error

	// HealthCheck verifies the engine is operational
	HealthCheck(ctx context.Context) error

	// GetConfig returns the engine's recommended runtime configuration
	GetConfig(ctx context.Context) (*sdktypes.EngineRecommendConfig, error)
}

// EngineInstaller defines engine installation and setup operations
type EngineInstaller interface {
	// CheckEngine verifies if the engine binary is installed
	CheckEngine() (bool, error)

	// InstallEngine downloads and installs the engine binary
	InstallEngine(ctx context.Context) error

	// InitEnv sets up the environment variables required by the engine
	InitEnv() error

	// UpgradeEngine updates the engine to the latest compatible version
	UpgradeEngine(ctx context.Context) error
}

// ModelManager defines model management operations
type ModelManager interface {
	// PullModel downloads a model from the repository with progress tracking
	PullModel(ctx context.Context, req *sdktypes.PullModelRequest, fn sdktypes.PullProgressFunc) (*sdktypes.ProgressResponse, error)

	// PullModelStream downloads a model with streaming progress updates
	PullModelStream(ctx context.Context, req *sdktypes.PullModelRequest) (chan []byte, chan error)

	// DeleteModel removes a model from local storage
	DeleteModel(ctx context.Context, req *sdktypes.DeleteRequest) error

	// ListModels returns all available models in the local repository
	ListModels(ctx context.Context) (*sdktypes.ListResponse, error)

	// LoadModel preloads a model into memory for faster inference
	LoadModel(ctx context.Context, req *sdktypes.LoadRequest) error

	// UnloadModel removes a model from memory to free resources
	UnloadModel(ctx context.Context, req *sdktypes.UnloadModelRequest) error

	// GetRunningModels returns the list of currently loaded models
	GetRunningModels(ctx context.Context) (*sdktypes.ListResponse, error)
}

// EngineInfoProvider defines informational operations
type EngineInfoProvider interface {
	// GetVersion returns the engine's version information
	GetVersion(ctx context.Context, resp *sdktypes.EngineVersionResponse) (*sdktypes.EngineVersionResponse, error)

	// GetOperateStatus returns the current operational status (0: stopped, 1: running, 2: error)
	GetOperateStatus() int

	// SetOperateStatus updates the operational status
	SetOperateStatus(status int)
}

// ServiceInvoker defines service invocation operations
type ServiceInvoker interface {
	// InvokeService executes a service request and returns the response
	// serviceName: "chat", "embed", "text-to-image" etc.
	// request: JSON formatted request data
	// returns: JSON formatted response data
	InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error)
}

// ModelServiceProvider defines the complete interface for AI model service providers
// This unified interface is implemented by both built-in engines and plugins
type ModelServiceProvider interface {
	EngineLifecycleManager
	EngineInstaller
	ModelManager
	EngineInfoProvider
	ServiceInvoker
}
