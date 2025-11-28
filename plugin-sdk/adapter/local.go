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

package adapter

import (
	"context"
	"fmt"

	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/types"
)

// Compile-time interface checks to ensure LocalPluginAdapter implements all SDK interfaces.
var (
	_ client.PluginProvider      = (*LocalPluginAdapter)(nil)
	_ client.LocalPluginProvider = (*LocalPluginAdapter)(nil)
)

// LocalPluginAdapter is an adapter for local engine plugins.
//
// Designed specifically for Local-type plugins, it provides engine lifecycle management interfaces.
// Local plugins manage local engines, including lifecycle, installation, environment initialization,
// and model management. They don't require authentication management.
type LocalPluginAdapter struct {
	*BasePluginProvider
	EngineHost string
}

// NewLocalPluginAdapter creates a new local plugin adapter.
func NewLocalPluginAdapter(manifest *types.PluginManifest) *LocalPluginAdapter {
	return &LocalPluginAdapter{
		BasePluginProvider: NewBasePluginProvider(manifest),
	}
}

// StartEngine starts the local engine. Plugins must implement this method.
func (l *LocalPluginAdapter) StartEngine(mode string) error {
	return l.WrapError("start_engine", fmt.Errorf("StartEngine must be implemented by plugin"))
}

// StopEngine stops the local engine. Plugins must implement this method.
func (l *LocalPluginAdapter) StopEngine() error {
	return l.WrapError("stop_engine", fmt.Errorf("StopEngine must be implemented by plugin"))
}

// GetConfig returns the engine configuration. Plugins must implement this method.
func (l *LocalPluginAdapter) GetConfig(ctx context.Context) (*types.EngineRecommendConfig, error) {
	return nil, l.WrapError("get_config", fmt.Errorf("GetConfig must be implemented by plugin"))
}

// CheckEngine checks if the engine is installed.
func (l *LocalPluginAdapter) CheckEngine() (bool, error) {
	return false, nil
}

// InstallEngine installs the engine. Plugins must implement this method.
func (l *LocalPluginAdapter) InstallEngine(ctx context.Context) error {
	return l.WrapError("install_engine", fmt.Errorf("InstallEngine must be implemented by plugin"))
}

// InitEnv initializes environment variables. Plugins must implement this method.
func (l *LocalPluginAdapter) InitEnv() error {
	return l.WrapError("init_env", fmt.Errorf("InitEnv must be implemented by plugin"))
}

// UpgradeEngine upgrades the engine. Plugins must implement this method.
func (l *LocalPluginAdapter) UpgradeEngine(ctx context.Context) error {
	return l.WrapError("upgrade_engine", fmt.Errorf("UpgradeEngine must be implemented by plugin"))
}

// PullModel pulls a model from a remote repository. Plugins must implement this method.
func (l *LocalPluginAdapter) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
	return nil, l.WrapError("pull_model", fmt.Errorf("PullModel must be implemented by plugin"))
}

// PullModelStream pulls a model with streaming progress. Plugins must implement this method.
func (l *LocalPluginAdapter) PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error) {
	dataChan := make(chan []byte)
	errChan := make(chan error, 1)

	close(dataChan)
	errChan <- l.WrapError("pull_model_stream", fmt.Errorf("PullModelStream must be implemented by plugin"))

	return dataChan, errChan
}

// DeleteModel deletes a local model. Plugins must implement this method.
func (l *LocalPluginAdapter) DeleteModel(ctx context.Context, req *types.DeleteRequest) error {
	return l.WrapError("delete_model", fmt.Errorf("DeleteModel must be implemented by plugin"))
}

// ListModels lists all local models. Plugins must implement this method.
func (l *LocalPluginAdapter) ListModels(ctx context.Context) (*types.ListResponse, error) {
	return nil, l.WrapError("list_models", fmt.Errorf("ListModels must be implemented by plugin"))
}

// LoadModel loads a model into memory. Plugins must implement this method.
func (l *LocalPluginAdapter) LoadModel(ctx context.Context, req *types.LoadRequest) error {
	return l.WrapError("load_model", fmt.Errorf("LoadModel must be implemented by plugin"))
}

// UnloadModel unloads models from memory. Plugins must implement this method.
func (l *LocalPluginAdapter) UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error {
	return l.WrapError("unload_model", fmt.Errorf("UnloadModel must be implemented by plugin"))
}

// GetRunningModels returns currently loaded models. Plugins must implement this method.
func (l *LocalPluginAdapter) GetRunningModels(ctx context.Context) (*types.ListResponse, error) {
	return nil, l.WrapError("get_running_models", fmt.Errorf("GetRunningModels must be implemented by plugin"))
}

// GetVersion returns the engine version. Plugins must implement this method.
func (l *LocalPluginAdapter) GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error) {
	return nil, l.WrapError("get_version", fmt.Errorf("GetVersion must be implemented by plugin"))
}
