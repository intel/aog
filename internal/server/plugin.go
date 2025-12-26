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

package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/plugin/registry"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

type Plugin interface {
	PluginList(ctx context.Context) (*dto.GetPluginListResponse, error)
	PluginInfo(ctx context.Context, request *dto.GetPluginInfoRequest) (*dto.GetPluginInfoResponse, error)
	PluginStop(ctx context.Context, request *dto.PluginStopRequest) (*dto.PluginStopResponse, error)
	PluginDelete(ctx context.Context, request *dto.PluginDeleteRequest) (*dto.PluginDeleteResponse, error)
	PluginLoad(ctx context.Context, request *dto.PluginLoadRequest) (*dto.PluginLoadResponse, error)
	PluginDownload(ctx context.Context, request *dto.PluginLoadRequest) (*dto.PluginLoadResponse, error)
}

type PluginImpl struct {
	Ds       datastore.Datastore
	JDs      datastore.JsonDatastore
	Registry *registry.PluginRegistry
}

func NewPlugin() *PluginImpl {
	return &PluginImpl{
		Ds:       datastore.GetDefaultDatastore(),
		JDs:      datastore.GetDefaultJsonDatastore(),
		Registry: registry.GetGlobalPluginRegistry(),
	}
}

func (s *PluginImpl) PluginList(ctx context.Context) (*dto.GetPluginListResponse, error) {
	manifests := s.Registry.GetAllManifests()
	pluginList := []dto.GetPluginInfoResponseData{}
	for name, manifest := range manifests {
		var services []string
		for _, service := range manifest.Manifest.Services {
			services = append(services, service.ServiceName)
		}
		status := s.Registry.GetPluginStatus(name)
		pluginInfo := dto.GetPluginInfoResponseData{
			Name:         name,
			ProviderName: manifest.Manifest.Provider.Name,
			Version:      manifest.Manifest.Version,
			Services:     services,
			Description:  manifest.Manifest.Provider.Description,
			Status:       status,
		}
		pluginList = append(pluginList, pluginInfo)
	}
	pluginDir := s.Registry.GetPluginDir()
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		logger.LogicLogger.Info("Plugin directory does not exist", "dir", pluginDir)
		return nil, err
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(pluginDir, entry.Name())
		manifest, err := sdktypes.LoadManifest(pluginPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load plugin manifest: %w", err)
		}
		pluginName := manifest.Provider.Name
		if _, exists := manifests[pluginName]; exists {
			continue
		}
		var unLoadPluginServices []string
		for _, service := range manifest.Services {
			unLoadPluginServices = append(unLoadPluginServices, service.ServiceName)
		}
		pluginInfo := dto.GetPluginInfoResponseData{
			Name:         pluginName,
			ProviderName: manifest.Provider.Name,
			Version:      manifest.Version,
			Services:     unLoadPluginServices,
			Description:  manifest.Provider.Description,
			Status:       constants.PluginStatusUnload,
		}
		pluginList = append(pluginList, pluginInfo)

	}

	return &dto.GetPluginListResponse{
		Bcode: *bcode.PluginCode,
		Data:  pluginList,
	}, nil
}

func (s *PluginImpl) PluginInfo(ctx context.Context, request *dto.GetPluginInfoRequest) (*dto.GetPluginInfoResponse, error) {
	sp := new(types.ServiceProvider)
	sp.ProviderName = request.Name
	pluginManifest, err := s.Registry.GetPluginManifest(request.Name)
	if err != nil {
		return nil, bcode.ErrPluginNotFound
	}
	var services []string
	for _, service := range pluginManifest.Services {
		services = append(services, service.ServiceName)
	}
	status := s.Registry.GetPluginStatus(request.Name)
	pluginInfo := dto.GetPluginInfoResponseData{
		Name:         request.Name,
		ProviderName: pluginManifest.Provider.Name,
		Version:      pluginManifest.Version,
		Services:     services,
		Description:  pluginManifest.Provider.Description,
		Status:       status,
	}

	return &dto.GetPluginInfoResponse{
		Bcode: *bcode.PluginCode,
		Data:  pluginInfo,
	}, nil
}

func (s *PluginImpl) PluginStop(ctx context.Context, request *dto.PluginStopRequest) (*dto.PluginStopResponse, error) {
	err := s.Registry.StopPluginProcess(request.Name)
	if err != nil {
		return nil, err
	}

	return &dto.PluginStopResponse{
		Bcode: *bcode.PluginCode,
	}, nil
}

func (s *PluginImpl) PluginDelete(ctx context.Context, request *dto.PluginDeleteRequest) (*dto.PluginDeleteResponse, error) {
	// stop plugin process
	err := s.Registry.StopPluginProcess(request.Name)
	if err != nil {
		return nil, bcode.ErrPluginNotFound
	}

	// delete db
	sp := new(types.ServiceProvider)
	sp.Flavor = request.Name
	err = s.Ds.Delete(ctx, sp)
	if err != nil {
		return nil, err
	}

	// delete plugin file
	manifest, err := s.Registry.GetPluginManifest(request.Name)
	if err == nil {
		if _, err = os.Stat(manifest.PluginDir); !os.IsNotExist(err) {
			err = os.RemoveAll(manifest.PluginDir)
			if err != nil {
				return nil, err
			}
		}
	}

	// remove glob variable
	err = s.Registry.UninstallPlugin(request.Name)
	if err != nil {
		return nil, err
	}
	return &dto.PluginDeleteResponse{
		Bcode: *bcode.PluginCode,
	}, nil
}

func (s *PluginImpl) PluginLoad(ctx context.Context, request *dto.PluginLoadRequest) (*dto.PluginLoadResponse, error) {
	manifest, err := s.Registry.GetPluginManifest(request.Name)
	if manifest != nil {
		return nil, bcode.ErrPluginDuplicateRegistration
	}

	pluginDir := filepath.Join(config.GlobalEnvironment.RootDir, "plugins")
	pluginPath := filepath.Join(pluginDir, request.Name)
	if _, err = os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, bcode.ErrPluginNotFound
	}

	// start Load plugin
	err = s.Registry.RegisterPlugin(request.Name, pluginPath)
	if err != nil {
		return nil, err
	}

	return &dto.PluginLoadResponse{
		Bcode: *bcode.PluginCode,
	}, nil
}

func (s *PluginImpl) PluginDownload(ctx context.Context, request *dto.PluginLoadRequest) (*dto.PluginLoadResponse, error) {
	manifest, err := s.Registry.GetPluginManifest(request.Name)
	if manifest != nil {
		return nil, bcode.ErrPluginDuplicateRegistration
	}

	pluginDir := filepath.Join(config.GlobalEnvironment.RootDir, "plugins")
	pluginPath := filepath.Join(pluginDir, request.Name)
	if _, err = os.Stat(pluginPath); os.IsNotExist(err) {
		return nil, bcode.ErrPluginNotFound
	}

	// start Load plugin
	err = s.Registry.RegisterPlugin(request.Name, pluginPath)
	if err != nil {
		return nil, err
	}

	return &dto.PluginLoadResponse{
		Bcode: *bcode.PluginCode,
	}, nil
}
