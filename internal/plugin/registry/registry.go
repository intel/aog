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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	sdkclient "github.com/intel/aog/plugin-sdk/client"
	sdkserver "github.com/intel/aog/plugin-sdk/server"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/version"
)

// FlavorRegistrar is a callback function type for registering APIFlavors.
// Used for dependency injection to avoid circular dependencies.
type FlavorRegistrar func(*sdktypes.PluginManifest) error

// PluginRegistry manages plugin discovery, loading, caching, and lifecycle.
type PluginRegistry struct {
	mu              sync.RWMutex
	pluginDir       string
	manifests       map[string]*sdktypes.PluginManifest
	plugins         map[string]*pluginHandle
	flavorRegistrar FlavorRegistrar
	datastore       datastore.Datastore
}

// pluginHandle contains plugin metadata, provider instance, and loading state.
//
// Supports both legacy and SDK interface types:
//   - Legacy: provider.ModelServiceProvider (backward compatibility)
//   - SDK: sdkclient.PluginProvider / RemotePluginProvider / LocalPluginProvider
type pluginHandle struct {
	manifest *sdktypes.PluginManifest

	loadOnce sync.Once
	loadErr  error

	providerRaw interface{}

	provider provider.ModelServiceProvider

	basePlugin   sdkclient.PluginProvider
	remotePlugin sdkclient.RemotePluginProvider
	localPlugin  sdkclient.LocalPluginProvider

	client *plugin.Client
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry(pluginDir string, ds datastore.Datastore) *PluginRegistry {
	return &PluginRegistry{
		pluginDir: pluginDir,
		manifests: make(map[string]*sdktypes.PluginManifest),
		plugins:   make(map[string]*pluginHandle),
		datastore: ds,
	}
}

// DiscoverPlugins discovers all plugins in the plugin directory.
// Scans the pluginDir directory and loads plugin.yaml from each subdirectory.
func (r *PluginRegistry) DiscoverPlugins() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.EngineLogger.Info("Starting plugin discovery", "directory", r.pluginDir)

	if _, err := os.Stat(r.pluginDir); os.IsNotExist(err) {
		logger.EngineLogger.Info("Plugin directory does not exist, skipping plugin discovery", "dir", r.pluginDir)
		return nil
	}

	entries, err := os.ReadDir(r.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	logger.EngineLogger.Info("Scanning plugin directory",
		"directory", r.pluginDir,
		"entries", len(entries))

	discoveredCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(r.pluginDir, entry.Name())
		if err := r.discoverPlugin(pluginPath); err != nil {
			logger.EngineLogger.Warn("Failed to discover plugin",
				"path", pluginPath,
				"error", err)
			continue
		}
		discoveredCount++
	}

	logger.EngineLogger.Info("Plugin discovery completed",
		"directory", r.pluginDir,
		"discovered", discoveredCount)

	if r.flavorRegistrar != nil {
		if err := r.registerAllPluginFlavors(); err != nil {
			logger.EngineLogger.Error("Failed to register plugin flavors",
				"error", err)
			return fmt.Errorf("failed to register plugin flavors: %w", err)
		}
	}

	if r.datastore != nil {
		if err := r.registerAllPluginServices(); err != nil {
			logger.EngineLogger.Error("Failed to register plugin services", "error", err)
		}
	}

	return nil
}

// SetFlavorRegistrar sets the Flavor registration callback function.
// Uses dependency injection to avoid circular dependencies.
func (r *PluginRegistry) SetFlavorRegistrar(registrar FlavorRegistrar) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flavorRegistrar = registrar
}

func (r *PluginRegistry) registerAllPluginFlavors() error {
	if r.flavorRegistrar == nil {
		logger.EngineLogger.Warn("FlavorRegistrar not set, skipping plugin flavor registration")
		return nil
	}

	registeredCount := 0
	for name, manifest := range r.manifests {
		if err := r.flavorRegistrar(manifest); err != nil {
			logger.EngineLogger.Error("Failed to register plugin flavor", "plugin", name, "error", err)
			continue
		}
		registeredCount++
		logger.EngineLogger.Debug("Plugin registered as APIFlavor",
			"plugin", name,
			"services", len(manifest.Services))
	}

	logger.EngineLogger.Info("Plugin flavor registration completed",
		"total", len(r.manifests),
		"registered", registeredCount)

	return nil
}

func (r *PluginRegistry) registerAllPluginServices() error {
	if r.datastore == nil {
		logger.EngineLogger.Debug("DataStore not available, skipping plugin service registration")
		return nil
	}

	successCount := 0
	for name, manifest := range r.manifests {
		if err := r.registerPluginServices(manifest); err != nil {
			logger.EngineLogger.Error("Failed to register services for plugin", "plugin", name, "error", err)
			continue
		}
		successCount++
	}

	logger.EngineLogger.Info("Plugin service registration summary",
		"total", len(r.manifests),
		"success", successCount,
		"failed", len(r.manifests)-successCount)

	return nil
}

func (r *PluginRegistry) discoverPlugin(pluginPath string) error {
	manifest, err := sdktypes.LoadManifest(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	manifest.PluginDir = pluginPath

	pluginName := manifest.Provider.Name

	if _, exists := r.manifests[pluginName]; exists {
		return fmt.Errorf("plugin name conflict: %s already exists", pluginName)
	}

	r.manifests[pluginName] = manifest
	r.plugins[pluginName] = &pluginHandle{
		manifest: manifest,
	}

	logger.EngineLogger.Info("Plugin discovered",
		"name", pluginName,
		"version", manifest.Provider.Version,
		"path", pluginPath)

	return nil
}

// GetProvider returns the plugin Provider (implements ProviderFactory interface).
// Uses lazy loading with singleton pattern. Kept for backward compatibility; use GetPluginProvider() instead.
func (r *PluginRegistry) GetProvider(name string) (provider.ModelServiceProvider, error) {
	r.mu.RLock()
	handle, exists := r.plugins[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	// Lazy loading with sync.Once ensures single initialization
	handle.loadOnce.Do(func() {
		r.loadPluginAndIdentifyType(handle)
	})

	if handle.loadErr != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", name, handle.loadErr)
	}

	if handle.provider != nil {
		return handle.provider, nil
	}

	if handle.localPlugin != nil {
		adapter := NewLocalPluginAdapter(handle.localPlugin)
		logger.EngineLogger.Debug("Adapting LocalPluginProvider to ModelServiceProvider",
			"plugin", name,
			"localPlugin_type", fmt.Sprintf("%T", handle.localPlugin),
			"adapter_type", fmt.Sprintf("%T", adapter),
			"adapter_value", fmt.Sprintf("%#v", adapter))
		return adapter, nil
	}

	// TODO: Support RemotePluginProvider adapter
	if handle.remotePlugin != nil {
		adapter := NewRemotePluginAdapter(handle.remotePlugin)
		logger.EngineLogger.Debug("Adapting LocalPluginProvider to ModelServiceProvider",
			"plugin", name,
			"localPlugin_type", fmt.Sprintf("%T", handle.localPlugin),
			"adapter_type", fmt.Sprintf("%T", adapter),
			"adapter_value", fmt.Sprintf("%#v", adapter))
		return adapter, nil
	}

	return nil, fmt.Errorf("plugin %s does not implement ModelServiceProvider-compatible interface", name)
}

// GetPluginProvider returns the plugin's base interface.
// All plugins implement the PluginProvider interface.
func (r *PluginRegistry) GetPluginProvider(name string) (sdkclient.PluginProvider, error) {
	r.mu.RLock()
	handle, exists := r.plugins[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	handle.loadOnce.Do(func() {
		r.loadPluginAndIdentifyType(handle)
	})

	if handle.loadErr != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", name, handle.loadErr)
	}

	if handle.basePlugin == nil {
		return nil, fmt.Errorf("plugin %s does not implement PluginProvider interface", name)
	}

	return handle.basePlugin, nil
}

// GetRemotePluginProvider returns the Remote plugin interface.
func (r *PluginRegistry) GetRemotePluginProvider(name string) (sdkclient.RemotePluginProvider, error) {
	r.mu.RLock()
	handle, exists := r.plugins[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	handle.loadOnce.Do(func() {
		r.loadPluginAndIdentifyType(handle)
	})

	if handle.loadErr != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", name, handle.loadErr)
	}

	if handle.remotePlugin == nil {
		return nil, fmt.Errorf("plugin %s does not implement RemotePluginProvider interface", name)
	}

	return handle.remotePlugin, nil
}

// GetLocalPluginProvider returns the Local plugin interface.
func (r *PluginRegistry) GetLocalPluginProvider(name string) (sdkclient.LocalPluginProvider, error) {
	r.mu.RLock()
	handle, exists := r.plugins[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	handle.loadOnce.Do(func() {
		r.loadPluginAndIdentifyType(handle)
	})

	if handle.loadErr != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", name, handle.loadErr)
	}

	if handle.localPlugin == nil {
		return nil, fmt.Errorf("plugin %s does not implement LocalPluginProvider interface", name)
	}

	return handle.localPlugin, nil
}

// ListAvailableProviders lists all available plugins.
func (r *PluginRegistry) ListAvailableProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.manifests))
	for name := range r.manifests {
		names = append(names, name)
	}
	return names
}

// loadPluginAndIdentifyType loads the plugin and identifies its interface types.
func (r *PluginRegistry) loadPluginAndIdentifyType(handle *pluginHandle) {
	manifest := handle.manifest
	logger.EngineLogger.Info("Loading plugin", "name", manifest.Provider.Name)

	executable, err := r.getExecutableForPlatform(manifest)
	if err != nil {
		handle.loadErr = fmt.Errorf("failed to get executable: %w", err)
		return
	}

	// CRITICAL: Use SDK's PluginHandshake and PluginMap to ensure Host and Plugin use the same definitions
	clientConfig := &plugin.ClientConfig{
		HandshakeConfig:  sdkserver.PluginHandshake,
		Plugins:          sdkserver.PluginMap,
		Cmd:              r.buildCommand(executable),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.NewNullLogger(), // Disable plugin framework logs to avoid interfering with AOG logs
		// Do not set SyncStdout/SyncStderr, let go-plugin use default pipe handling
		// This avoids pipe blocking and file I/O issues
	}

	logger.EngineLogger.Debug("Creating plugin client", "name", manifest.Provider.Name, "executable", executable)
	logger.EngineLogger.Debug("Config details", "name", manifest.Provider.Name,
		"handshake_version", clientConfig.HandshakeConfig.ProtocolVersion,
		"magic_cookie_key", clientConfig.HandshakeConfig.MagicCookieKey)

	// This will start the plugin process
	client := plugin.NewClient(clientConfig)
	logger.EngineLogger.Debug("plugin.NewClient() returned", "name", manifest.Provider.Name)

	logger.EngineLogger.Debug("Establishing RPC connection", "name", manifest.Provider.Name)

	// Use goroutine + channel to implement timeout mechanism
	type rpcResult struct {
		client plugin.ClientProtocol
		err    error
	}
	rpcChan := make(chan rpcResult, 1)
	go func() {
		logger.EngineLogger.Debug("RPC connection goroutine started", "name", manifest.Provider.Name)
		c, e := client.Client()
		logger.EngineLogger.Debug("client.Client() returned", "name", manifest.Provider.Name, "error", e)
		rpcChan <- rpcResult{client: c, err: e}
	}()

	// Wait for RPC connection or timeout (10 seconds)
	var rpcClient plugin.ClientProtocol
	select {
	case result := <-rpcChan:
		rpcClient = result.client
		err = result.err
		if err != nil {
			client.Kill()
			handle.loadErr = fmt.Errorf("failed to get RPC client: %w", err)
			logger.EngineLogger.Error("RPC connection failed", "name", manifest.Provider.Name, "error", err)
			return
		}
	case <-time.After(10 * time.Second):
		client.Kill()
		handle.loadErr = fmt.Errorf("RPC connection timeout after 10s")
		logger.EngineLogger.Error("RPC connection timeout", "name", manifest.Provider.Name)
		return
	}
	logger.EngineLogger.Debug("RPC connection established", "name", manifest.Provider.Name)

	// Use SDK-defined plugin type identifier
	logger.EngineLogger.Debug("Dispensing plugin", "name", manifest.Provider.Name, "type", sdkserver.PluginTypeProvider)
	raw, err := rpcClient.Dispense(sdkserver.PluginTypeProvider)
	if err != nil {
		client.Kill()
		handle.loadErr = fmt.Errorf("failed to dispense plugin: %w", err)
		logger.EngineLogger.Error("Dispense failed", "name", manifest.Provider.Name, "error", err)
		return
	}
	logger.EngineLogger.Debug("Plugin dispensed successfully", "name", manifest.Provider.Name)

	handle.providerRaw = raw
	handle.client = client

	logger.EngineLogger.Debug("Plugin raw type",
		"name", manifest.Provider.Name,
		"type", fmt.Sprintf("%T", raw))

	// SDK's GRPCClient now returns *sdkclient.GRPCProviderClient which implements all SDK interfaces
	if basePlugin, ok := raw.(sdkclient.PluginProvider); ok {
		handle.basePlugin = basePlugin
		logger.EngineLogger.Debug("✅ Plugin implements SDK PluginProvider interface",
			"name", manifest.Provider.Name)
	} else {
		logger.EngineLogger.Debug("❌ Plugin does NOT implement SDK PluginProvider interface",
			"name", manifest.Provider.Name)
	}

	if remotePlugin, ok := raw.(sdkclient.RemotePluginProvider); ok {
		handle.remotePlugin = remotePlugin
		logger.EngineLogger.Debug("✅ Plugin implements SDK RemotePluginProvider interface",
			"name", manifest.Provider.Name)
	}

	if localPlugin, ok := raw.(sdkclient.LocalPluginProvider); ok {
		handle.localPlugin = localPlugin
		logger.EngineLogger.Debug("✅ Plugin implements SDK LocalPluginProvider interface",
			"name", manifest.Provider.Name)
	} else {
		logger.EngineLogger.Debug("ℹ️  Plugin does NOT implement SDK LocalPluginProvider interface (may be remote plugin)",
			"name", manifest.Provider.Name)
	}

	// Backward compatibility: try to identify legacy interface
	if oldProvider, ok := raw.(provider.ModelServiceProvider); ok {
		handle.provider = oldProvider
		logger.EngineLogger.Debug("Plugin implements legacy ModelServiceProvider interface",
			"name", manifest.Provider.Name)
	}

	logger.EngineLogger.Info("Plugin loaded successfully",
		"name", manifest.Provider.Name,
		"version", manifest.Provider.Version,
		"interfaces", r.describePluginInterfaces(handle))
}

// describePluginInterfaces describes the interfaces implemented by the plugin.
func (r *PluginRegistry) describePluginInterfaces(handle *pluginHandle) string {
	interfaces := []string{}
	if handle.basePlugin != nil {
		interfaces = append(interfaces, "PluginProvider")
	}
	if handle.remotePlugin != nil {
		interfaces = append(interfaces, "RemotePluginProvider")
	}
	if handle.localPlugin != nil {
		interfaces = append(interfaces, "LocalPluginProvider")
	}
	if handle.provider != nil {
		interfaces = append(interfaces, "ModelServiceProvider(legacy)")
	}

	if len(interfaces) == 0 {
		return "none"
	}

	result := ""
	for i, iface := range interfaces {
		if i > 0 {
			result += ", "
		}
		result += iface
	}
	return result
}

// =============== Plugin Service Registration ===============

// registerPluginServices registers plugin services to the service_provider table.
func (r *PluginRegistry) registerPluginServices(manifest *sdktypes.PluginManifest) error {
	if r.datastore == nil {
		logger.EngineLogger.Warn("DataStore not available, skipping plugin service registration",
			"plugin", manifest.Provider.Name)
		return nil
	}

	if err := r.validateProviderType(manifest); err != nil {
		return fmt.Errorf("plugin type validation failed: %w", err)
	}

	ctx := context.Background()
	registeredServices := make([]string, 0)

	for _, service := range manifest.Services {
		serviceProvider := r.createServiceProvider(manifest, &service)

		query := &types.ServiceProvider{ProviderName: serviceProvider.ProviderName}
		listOpts := &datastore.ListOptions{}
		existingList, err := r.datastore.List(ctx, query, listOpts)
		if err != nil {
			return fmt.Errorf("failed to check existing service provider %s: %w",
				serviceProvider.ProviderName, err)
		}

		if len(existingList) > 0 {
			if existing, ok := existingList[0].(*types.ServiceProvider); ok {
				serviceProvider.ID = existing.ID
				if err := r.datastore.Put(ctx, serviceProvider); err != nil {
					logger.EngineLogger.Error("Failed to update plugin service provider",
						"provider", serviceProvider.ProviderName,
						"error", err)
					continue
				}
				logger.EngineLogger.Info("Updated plugin service provider",
					"provider", serviceProvider.ProviderName,
					"service", serviceProvider.ServiceName)
			}
		} else {
			if err := r.datastore.Put(ctx, serviceProvider); err != nil {
				logger.EngineLogger.Error("Failed to create plugin service provider",
					"provider", serviceProvider.ProviderName,
					"error", err)
				continue
			}
			logger.EngineLogger.Info("Registered plugin service provider",
				"provider", serviceProvider.ProviderName,
				"service", serviceProvider.ServiceName)
		}

		registeredServices = append(registeredServices, service.ServiceName)
	}

	logger.EngineLogger.Info("Plugin service registration completed",
		"plugin", manifest.Provider.Name,
		"services", registeredServices)

	return nil
}

// createServiceProvider creates a ServiceProvider record from plugin metadata.
func (r *PluginRegistry) createServiceProvider(manifest *sdktypes.PluginManifest, service *sdktypes.ServiceDef) *types.ServiceProvider {
	fullURL := service.Endpoint
	if manifest.Provider.EngineHost != "" {
		fullURL = manifest.Provider.EngineHost + service.Endpoint
	}

	logger.EngineLogger.Debug("Creating service provider",
		"plugin", manifest.Provider.Name,
		"service", service.ServiceName,
		"engine_host", manifest.Provider.EngineHost,
		"endpoint", service.Endpoint,
		"fullURL", fullURL)

	return &types.ServiceProvider{
		ProviderName:  fmt.Sprintf("%s_%s_%s", manifest.Provider.Type, manifest.Provider.Name, service.ServiceName),
		ServiceName:   service.ServiceName, // chat, embed, etc.
		ServiceSource: manifest.Provider.Type,
		Desc:          fmt.Sprintf("%s Plugin - %s Service", manifest.Provider.Name, strings.Title(service.ServiceName)),
		Method:        r.protocolToMethod(service.Protocol),
		AuthType:      service.AuthType,
		AuthKey:       "",
		Flavor:        manifest.Provider.Name,
		URL:           fullURL,
		Scope:         "plugin",
	}
}

// validateProviderType validates the plugin type field.
func (r *PluginRegistry) validateProviderType(manifest *sdktypes.PluginManifest) error {
	providerType := manifest.Provider.Type
	if providerType != "local" && providerType != "remote" {
		return fmt.Errorf("invalid provider type '%s', must be 'local' or 'remote'", providerType)
	}
	return nil
}

// protocolToMethod converts protocol to HTTP method.
func (r *PluginRegistry) protocolToMethod(protocol string) string {
	switch strings.ToUpper(protocol) {
	case "HTTP", "HTTPS":
		return "POST"
	case "WEBSOCKET", "WSS":
		return "WS"
	default:
		return "POST"
	}
}

// IsEntityNotFound checks if the error indicates entity not found.
func IsEntityNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "no rows") ||
		strings.Contains(err.Error(), "record not found")
}

// Shutdown closes all plugins.
func (r *PluginRegistry) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, handle := range r.plugins {
		if handle.client != nil {
			logger.EngineLogger.Info("Shutting down plugin", "name", name)
			handle.client.Kill()
		}
	}
}

// GetPluginManifest returns the plugin's manifest.
func (r *PluginRegistry) GetPluginManifest(name string) (*sdktypes.PluginManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifest, exists := r.manifests[name]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	return manifest, nil
}

// getExecutableForPlatform returns the executable path for the current platform.
func (r *PluginRegistry) getExecutableForPlatform(manifest *sdktypes.PluginManifest) (string, error) {
	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH

	platformConfig, err := manifest.GetPlatformConfig(currentOS, currentArch)
	if err != nil {
		return "", fmt.Errorf("no configuration for platform %s/%s", currentOS, currentArch)
	}

	pluginPath := filepath.Join(r.pluginDir, manifest.Provider.Name)
	execPath := filepath.Join(pluginPath, platformConfig.Executable)

	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		return "", fmt.Errorf("executable not found: %s", execPath)
	}

	return execPath, nil
}

// buildCommand builds the plugin startup command and passes AOG runtime information.
func (r *PluginRegistry) buildCommand(executable string) *exec.Cmd {
	cmd := exec.Command(executable)

	// Pass AOG version to plugin for selecting correct engine version
	cmd.Env = append(os.Environ(), fmt.Sprintf("AOG_VERSION=%s", version.AOGVersion))

	return cmd
}

// Note: getHandshakeConfig and getPluginMap methods have been removed.
// Now directly using PluginHandshake and PluginMap exported from plugin-sdk/server.
// This ensures Host and Plugin use exactly the same definitions, which is a core requirement of go-plugin.

// PluginManifestWrapper wraps plugin manifest with path information.
type PluginManifestWrapper struct {
	Manifest *sdktypes.PluginManifest
	Path     string
}

// GetAllManifests returns all discovered plugin manifests (for CLI display).
func (r *PluginRegistry) GetAllManifests() map[string]*PluginManifestWrapper {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*PluginManifestWrapper)
	for name, manifest := range r.manifests {
		pluginPath := filepath.Join(r.pluginDir, manifest.Provider.Name)
		result[name] = &PluginManifestWrapper{
			Manifest: manifest,
			Path:     pluginPath,
		}
	}
	return result
}

// GetManifest returns the specified plugin's manifest.
func (r *PluginRegistry) GetManifest(name string) *sdktypes.PluginManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.manifests[name]
}

// ========== Global PluginRegistry Access ===========

var (
	globalPluginRegistry *PluginRegistry
	registryOnce         sync.Once
)

// SetGlobalPluginRegistry sets the global plugin registry (should be called once at startup).
func SetGlobalPluginRegistry(registry *PluginRegistry) {
	registryOnce.Do(func() {
		globalPluginRegistry = registry
	})
}

// GetGlobalPluginRegistry returns the global plugin registry.
// Returns nil if not initialized.
func GetGlobalPluginRegistry() *PluginRegistry {
	return globalPluginRegistry
}
