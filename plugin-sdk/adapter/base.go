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
	"log"
	"sync/atomic"

	"github.com/intel/aog/plugin-sdk/types"
)

// BasePluginProvider provides common base implementation for all plugins.
//
// It includes metadata management, status management, error handling, and logging.
// Plugin developers can embed this adapter to reduce boilerplate code.
type BasePluginProvider struct {
	manifest  *types.PluginManifest
	status    atomic.Int32 // 0: stopped, 1: running, 2: error
	logPrefix string
}

// NewBasePluginProvider creates a new base plugin provider.
func NewBasePluginProvider(manifest *types.PluginManifest) *BasePluginProvider {
	prefix := "Plugin"
	if manifest != nil && manifest.Provider.Name != "" {
		prefix = fmt.Sprintf("[%s]", manifest.Provider.Name)
	}

	return &BasePluginProvider{
		manifest:  manifest,
		logPrefix: prefix,
	}
}

// GetManifest returns the plugin metadata.
func (b *BasePluginProvider) GetManifest() *types.PluginManifest {
	return b.manifest
}

// GetOperateStatus returns the plugin operational status.
func (b *BasePluginProvider) GetOperateStatus() int {
	return int(b.status.Load())
}

// SetOperateStatus sets the plugin operational status.
func (b *BasePluginProvider) SetOperateStatus(status int) {
	oldStatus := b.status.Swap(int32(status))

	if oldStatus != int32(status) {
		b.LogDebug(fmt.Sprintf("Status changed: %s -> %s",
			b.statusToString(int(oldStatus)),
			b.statusToString(status)))
	}
}

// HealthCheck performs a default health check.
//
// The default implementation only checks plugin status. Subclasses should override this method
// to provide more specific health check logic.
func (b *BasePluginProvider) HealthCheck(ctx context.Context) error {
	status := b.GetOperateStatus()
	if status == 2 {
		return b.WrapError("health_check", fmt.Errorf("plugin is in error state"))
	}

	b.LogDebug(fmt.Sprintf("Health check passed, status: %s", b.statusToString(status)))
	return nil
}

// InvokeService Service invocation (must be implemented by a subclass)
//
// This is the core approach of a plug-in and must be implemented by a specific plug-in.
func (b *BasePluginProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	return nil, b.WrapError("invoke_service",
		fmt.Errorf("InvokeService must be implemented by plugin"))
}

// WrapError wraps an error with plugin context information.
func (b *BasePluginProvider) WrapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &types.PluginError{
		Code:    types.ErrCodeInternal,
		Message: fmt.Sprintf("[%s] %s failed", b.getPluginName(), operation),
		Details: err.Error(),
	}
}

func (b *BasePluginProvider) LogInfo(message string) {
	log.Printf("%s [INFO] %s\n", b.logPrefix, message)
}

func (b *BasePluginProvider) LogError(message string, err error) {
	log.Printf("%s [ERROR] %s: %v\n", b.logPrefix, message, err)
}

func (b *BasePluginProvider) LogDebug(message string) {
	log.Printf("%s [DEBUG] %s\n", b.logPrefix, message)
}

func (b *BasePluginProvider) getPluginName() string {
	if b.manifest != nil && b.manifest.Provider.Name != "" {
		return b.manifest.Provider.Name
	}
	return "unknown"
}

func (b *BasePluginProvider) statusToString(status int) string {
	switch status {
	case 0:
		return "stopped"
	case 1:
		return "running"
	case 2:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", status)
	}
}

// ValidateManifest validates the plugin metadata.
func (b *BasePluginProvider) ValidateManifest() error {
	if b.manifest == nil {
		return &types.PluginError{
			Code:    types.ErrCodePluginInvalidConfig,
			Message: "manifest is nil",
		}
	}

	if b.manifest.Provider.Name == "" {
		return &types.PluginError{
			Code:    types.ErrCodePluginInvalidConfig,
			Message: "provider name is empty",
		}
	}

	if b.manifest.Provider.Version == "" {
		return &types.PluginError{
			Code:    types.ErrCodePluginInvalidConfig,
			Message: "provider version is empty",
		}
	}

	if len(b.manifest.Services) == 0 {
		return &types.PluginError{
			Code:    types.ErrCodePluginInvalidConfig,
			Message: "no services defined",
		}
	}

	return nil
}

// GetServiceByName returns the service definition by name.
func (b *BasePluginProvider) GetServiceByName(serviceName string) (*types.ServiceDef, error) {
	if b.manifest == nil {
		return nil, b.WrapError("get_service", fmt.Errorf("manifest is nil"))
	}

	return b.manifest.GetServiceByName(serviceName)
}

// IsServiceSupported checks if the plugin supports the specified service.
func (b *BasePluginProvider) IsServiceSupported(serviceName string) bool {
	_, err := b.GetServiceByName(serviceName)
	return err == nil
}
