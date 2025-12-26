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

package registry

import (
	"context"
	"fmt"

	"github.com/intel/aog/internal/provider"
	sdkclient "github.com/intel/aog/plugin-sdk/client"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

// localPluginAdapter adapts SDK LocalPluginProvider to ModelServiceProvider.
// Enables plugins to integrate with the existing ProviderFactory system.
type localPluginAdapter struct {
	local sdkclient.LocalPluginProvider
}

// Compile-time check: ensure localPluginAdapter implements all required interfaces
var (
	_ sdkclient.PluginProvider      = (*localPluginAdapter)(nil)
	_ sdkclient.StreamablePlugin    = (*localPluginAdapter)(nil)
	_ sdkclient.BidirectionalPlugin = (*localPluginAdapter)(nil)
	_ provider.ModelServiceProvider = (*localPluginAdapter)(nil)
)

// NewLocalPluginAdapter creates a local plugin adapter.
func NewLocalPluginAdapter(local sdkclient.LocalPluginProvider) provider.ModelServiceProvider {
	return &localPluginAdapter{local: local}
}

// ==================== EngineLifecycleManager ====================

func (a *localPluginAdapter) StartEngine(mode string) error {
	return a.local.StartEngine(mode)
}

func (a *localPluginAdapter) StopEngine() error {
	return a.local.StopEngine()
}

func (a *localPluginAdapter) HealthCheck(ctx context.Context) error {
	return a.local.HealthCheck(ctx)
}

func (a *localPluginAdapter) GetConfig(ctx context.Context) (*sdktypes.EngineRecommendConfig, error) {
	return a.local.GetConfig(ctx)
}

// ==================== EngineInstaller ====================

func (a *localPluginAdapter) CheckEngine() (bool, error) {
	return a.local.CheckEngine()
}

func (a *localPluginAdapter) InstallEngine(ctx context.Context) error {
	return a.local.InstallEngine(ctx)
}

func (a *localPluginAdapter) InitEnv() error {
	return a.local.InitEnv()
}

func (a *localPluginAdapter) UpgradeEngine(ctx context.Context) error {
	return a.local.UpgradeEngine(ctx)
}

// ==================== ModelManager ====================

func (a *localPluginAdapter) PullModel(ctx context.Context, req *sdktypes.PullModelRequest, fn sdktypes.PullProgressFunc) (*sdktypes.ProgressResponse, error) {
	return a.local.PullModel(ctx, req, fn)
}

func (a *localPluginAdapter) PullModelStream(ctx context.Context, req *sdktypes.PullModelRequest) (chan []byte, chan error) {
	dataChan := make(chan []byte)
	errChan := make(chan error, 1)
	close(dataChan)
	return dataChan, errChan
}

func (a *localPluginAdapter) DeleteModel(ctx context.Context, req *sdktypes.DeleteRequest) error {
	return a.local.DeleteModel(ctx, req)
}

func (a *localPluginAdapter) ListModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	return a.local.ListModels(ctx)
}

func (a *localPluginAdapter) LoadModel(ctx context.Context, req *sdktypes.LoadRequest) error {
	return a.local.LoadModel(ctx, req)
}

func (a *localPluginAdapter) UnloadModel(ctx context.Context, req *sdktypes.UnloadModelRequest) error {
	return a.local.UnloadModel(ctx, req)
}

func (a *localPluginAdapter) GetRunningModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	return a.local.GetRunningModels(ctx)
}

func (a *localPluginAdapter) GetSupportModelList(ctx context.Context) ([]sdktypes.RecommendModelData, error) {
	return a.local.GetSupportModelList(ctx)
}

// ==================== EngineInfoProvider ====================

func (a *localPluginAdapter) GetVersion(ctx context.Context, resp *sdktypes.EngineVersionResponse) (*sdktypes.EngineVersionResponse, error) {
	return a.local.GetVersion(ctx, resp)
}

func (a *localPluginAdapter) GetOperateStatus() int {
	return a.local.GetOperateStatus()
}

func (a *localPluginAdapter) SetOperateStatus(status int) {
	a.local.SetOperateStatus(status)
}

// ==================== PluginProvider ====================

func (a *localPluginAdapter) GetManifest() *sdktypes.PluginManifest {
	return a.local.GetManifest()
}

// ==================== ServiceInvoker ====================

func (a *localPluginAdapter) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	return a.local.InvokeService(ctx, serviceName, authInfo, request)
}

// ==================== StreamablePlugin ====================

func (a *localPluginAdapter) InvokeServiceStream(ctx context.Context, serviceName string, authInfo string, request []byte) (<-chan sdkclient.StreamChunk, error) {
	// Check whether the underlying plugin implements StreamablePlugin
	if streamable, ok := a.local.(sdkclient.StreamablePlugin); ok {
		return streamable.InvokeServiceStream(ctx, serviceName, authInfo, request)
	}
	return nil, fmt.Errorf("plugin does not implement streaming interface")
}

// ==================== BidirectionalPlugin ====================

func (a *localPluginAdapter) InvokeServiceBidirectional(ctx context.Context, serviceName string, wsConnID string, authInfo string, inStream <-chan sdkclient.BidiMessage, outStream chan<- sdkclient.BidiMessage) error {
	// Check whether the underlying plugin implements BidirectionalPlugin
	if bidirectional, ok := a.local.(sdkclient.BidirectionalPlugin); ok {
		return bidirectional.InvokeServiceBidirectional(ctx, serviceName, wsConnID, authInfo, inStream, outStream)
	}
	return fmt.Errorf("plugin does not implement bidirectional streaming interface")
}

// remotePluginAdapter adapts SDK RemotePluginProvider to ModelServiceProvider
// Enables remote plugins to integrate with the existing ProviderFactory system
var (
	_ sdkclient.PluginProvider      = (*remotePluginAdapter)(nil)
	_ sdkclient.StreamablePlugin    = (*remotePluginAdapter)(nil)
	_ sdkclient.BidirectionalPlugin = (*remotePluginAdapter)(nil)
	_ provider.ModelServiceProvider = (*remotePluginAdapter)(nil)
)

type remotePluginAdapter struct {
	remote sdkclient.RemotePluginProvider
}

// NewRemotePluginAdapter creates a remote plugin adapter
func NewRemotePluginAdapter(remote sdkclient.RemotePluginProvider) provider.ModelServiceProvider {
	return &remotePluginAdapter{remote: remote}
}

// ==================== EngineLifecycleManager ====================
// Remote plugins do not manage engine lifecycle, return errors or no-ops

func (a *remotePluginAdapter) StartEngine(mode string) error {
	return fmt.Errorf("remote plugin does not support engine lifecycle management")
}

func (a *remotePluginAdapter) StopEngine() error {
	return fmt.Errorf("remote plugin does not support engine lifecycle management")
}

func (a *remotePluginAdapter) HealthCheck(ctx context.Context) error {
	return a.remote.HealthCheck(ctx)
}

func (a *remotePluginAdapter) GetConfig(ctx context.Context) (*sdktypes.EngineRecommendConfig, error) {
	return nil, fmt.Errorf("remote plugin does not support engine configuration")
}

// ==================== EngineInstaller ====================
// Remote plugins do not handle engine installation, return errors or no-ops

func (a *remotePluginAdapter) CheckEngine() (bool, error) {
	return false, fmt.Errorf("remote plugin does not support engine installation")
}

func (a *remotePluginAdapter) InstallEngine(ctx context.Context) error {
	return fmt.Errorf("remote plugin does not support engine installation")
}

func (a *remotePluginAdapter) InitEnv() error {
	return fmt.Errorf("remote plugin does not support environment initialization")
}

func (a *remotePluginAdapter) UpgradeEngine(ctx context.Context) error {
	return fmt.Errorf("remote plugin does not support engine upgrade")
}

// ==================== ModelManager ====================
// Remote plugins do not handle model management, return errors or no-ops

func (a *remotePluginAdapter) PullModel(ctx context.Context, req *sdktypes.PullModelRequest, fn sdktypes.PullProgressFunc) (*sdktypes.ProgressResponse, error) {
	return nil, fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) PullModelStream(ctx context.Context, req *sdktypes.PullModelRequest) (chan []byte, chan error) {
	dataChan := make(chan []byte)
	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("remote plugin does not support model management")
	close(dataChan)
	close(errChan)
	return dataChan, errChan
}

func (a *remotePluginAdapter) DeleteModel(ctx context.Context, req *sdktypes.DeleteRequest) error {
	return fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) ListModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	return nil, fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) LoadModel(ctx context.Context, req *sdktypes.LoadRequest) error {
	return fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) UnloadModel(ctx context.Context, req *sdktypes.UnloadModelRequest) error {
	return fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) GetRunningModels(ctx context.Context) (*sdktypes.ListResponse, error) {
	return nil, fmt.Errorf("remote plugin does not support model management")
}

func (a *remotePluginAdapter) GetSupportModelList(ctx context.Context) ([]sdktypes.RecommendModelData, error) {
	return a.remote.GetSupportModelList(ctx)
}

// ==================== EngineInfoProvider ====================

func (a *remotePluginAdapter) GetVersion(ctx context.Context, resp *sdktypes.EngineVersionResponse) (*sdktypes.EngineVersionResponse, error) {
	return nil, fmt.Errorf("remote plugin does not support version information")
}

func (a *remotePluginAdapter) GetOperateStatus() int {
	return a.remote.GetOperateStatus()
}

func (a *remotePluginAdapter) SetOperateStatus(status int) {
	a.remote.SetOperateStatus(status)
}

// ==================== PluginProvider ====================

func (a *remotePluginAdapter) GetManifest() *sdktypes.PluginManifest {
	return a.remote.GetManifest()
}

// ==================== ServiceInvoker ====================

func (a *remotePluginAdapter) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	return a.remote.InvokeService(ctx, serviceName, authInfo, request)
}

// ==================== StreamablePlugin Support ====================
// If the underlying remote plugin implements StreamablePlugin, adapt it as well

func (a *remotePluginAdapter) InvokeServiceStream(ctx context.Context, serviceName string, authInfo string, request []byte) (<-chan sdkclient.StreamChunk, error) {
	if streamable, ok := a.remote.(sdkclient.StreamablePlugin); ok {
		return streamable.InvokeServiceStream(ctx, serviceName, authInfo, request)
	}
	return nil, fmt.Errorf("remote plugin does not support streaming")
}

// ==================== BidirectionalPlugin Support ====================
// If the underlying remote plugin implements BidirectionalPlugin, adapt it as well

func (a *remotePluginAdapter) InvokeServiceBidirectional(ctx context.Context, serviceName string, wsconID string, authInfo string, inStream <-chan sdkclient.BidiMessage, outStream chan<- sdkclient.BidiMessage) error {
	if bidi, ok := a.remote.(sdkclient.BidirectionalPlugin); ok {
		return bidi.InvokeServiceBidirectional(ctx, serviceName, wsconID, authInfo, inStream, outStream)
	}
	return fmt.Errorf("remote plugin does not support bidirectional streaming")
}
