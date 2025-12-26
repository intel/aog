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
	"errors"
	"log/slog"
	"net/url"
	"strings"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/server/checker"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

type ServiceProvider interface {
	CreateServiceProvider(ctx context.Context, request *dto.CreateServiceProviderRequest) (*dto.CreateServiceProviderResponse, error)
	DeleteServiceProvider(ctx context.Context, request *dto.DeleteServiceProviderRequest) (*dto.DeleteServiceProviderResponse, error)
	UpdateServiceProvider(ctx context.Context, request *dto.UpdateServiceProviderRequest) (*dto.UpdateServiceProviderResponse, error)
	GetServiceProvider(ctx context.Context, request *dto.GetServiceProviderRequest) (*dto.GetServiceProviderResponse, error)
	GetServiceProviders(ctx context.Context, request *dto.GetServiceProvidersRequest) (*dto.GetServiceProvidersResponse, error)
}

type ServiceProviderImpl struct {
	Ds datastore.Datastore
}

func NewServiceProvider() ServiceProvider {
	return &ServiceProviderImpl{
		Ds: datastore.GetDefaultDatastore(),
	}
}

func (s *ServiceProviderImpl) CreateServiceProvider(ctx context.Context, request *dto.CreateServiceProviderRequest) (*dto.CreateServiceProviderResponse, error) {
	ds := datastore.GetDefaultDatastore()

	sp := &types.ServiceProvider{}
	sp.ProviderName = request.ProviderName

	isExist, err := ds.IsExist(ctx, sp)
	if err != nil {
		return nil, err
	}
	if isExist {
		return nil, bcode.ErrAIGCServiceProviderIsExist
	}
	providerServiceInfo := schedule.GetProviderServiceDefaultInfo(request.ApiFlavor, request.ServiceName)

	sp.ServiceName = request.ServiceName
	sp.ServiceSource = request.ServiceSource
	sp.Flavor = request.ApiFlavor
	sp.AuthType = request.AuthType
	if request.AuthType != types.AuthTypeNone && request.AuthKey == "" {
		return nil, bcode.ErrProviderAuthInfoLost
	}
	sp.AuthKey = request.AuthKey
	sp.Desc = request.Desc
	sp.Method = request.Method
	sp.URL = request.Url
	sp.Status = 0
	if request.Url == "" {
		sp.URL = providerServiceInfo.RequestUrl
	}
	if request.Method == "" {
		sp.Method = "POST"
	}
	sp.ExtraHeaders = request.ExtraHeaders
	if request.ExtraHeaders == "" {
		sp.ExtraHeaders = providerServiceInfo.ExtraHeaders
	}
	if request.ExtraJsonBody == "" {
		sp.ExtraJSONBody = "{}"
	}
	if request.Properties == "" {
		sp.Properties = "{}"
	}
	sp.CreatedAt = types.Now()
	sp.UpdatedAt = types.Now()

	modelIsExist := make(map[string]bool)

	if request.ServiceSource == types.ServiceSourceLocal {
		engineProvider, err := provider.GetModelEngine(request.ApiFlavor)
		if err != nil {
			logger.EngineLogger.Error("Failed to get engine", "flavor", request.ApiFlavor, "error", err)
			return nil, bcode.ErrProviderNotExist
		}
		engineConfig, err := engineProvider.GetConfig(ctx)
		if err != nil || engineConfig == nil {
			return nil, bcode.ErrEngineNotAvailable
		}
		if strings.Contains(request.Url, engineConfig.Host) {
			parseUrl, err := url.Parse(request.Url)
			if err != nil {
				return nil, bcode.ErrProviderServiceUrlNotFormat
			}
			host := parseUrl.Host
			engineConfig.Host = host
		}
		healthErr := engineProvider.HealthCheck(ctx)
		if healthErr != nil {
			return nil, healthErr
		}

		modelList, listErr := engineProvider.ListModels(ctx)
		if listErr != nil {
			return nil, listErr
		}

		for _, v := range modelList.Models {
			for _, mName := range request.Models {
				if v.Name == mName {
					modelIsExist[mName] = true
				} else if _, ok := modelIsExist[mName]; !ok {
					modelIsExist[mName] = false
				}
			}
		}

		for _, mName := range request.Models {
			if !modelIsExist[mName] {
				slog.Info("The model " + mName + " does not exist, ready to start pulling the model.")
				stream := false
				pullReq := &sdktypes.PullModelRequest{
					Model:  mName,
					Stream: &stream,
				}
				m := new(types.Model)
				m.ModelName = mName
				m.ProviderName = request.ProviderName
				err = s.Ds.Get(ctx, m)
				if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
					// todo debug log output
					return nil, bcode.ErrServer
				} else if errors.Is(err, datastore.ErrEntityInvalid) {
					m.Status = "downloading"
					err = s.Ds.Put(ctx, m)
					if err != nil {
						return nil, bcode.ErrAddModel
					}
				}
				if m.Status == "failed" {
					m.Status = "downloading"
				}
				if err != nil {
				}
				go AsyncPullModel(sp, m, pullReq)
			}
		}
	} else if request.ServiceSource == types.ServiceSourceRemote {
		checkSpStatus := 0
		for _, mName := range request.Models {
			server := checker.CreateChecker(*sp, mName)
			if server == nil {
				continue
			}
			checkRes := server.CheckServer()
			if !checkRes {
				continue
			}

			model := &types.Model{
				ModelName:    mName,
				ProviderName: request.ProviderName,
				Status:       "downloaded",
				CreatedAt:    types.Now(),
				UpdatedAt:    types.Now(),
			}

			err := ds.Put(ctx, model)
			if err != nil {
				return nil, err
			}
			err = createRelatedDBData(ctx, s.Ds, nil, model, nil)
			if err != nil {
				return nil, err
			}
			checkSpStatus = 1
		}
		sp.Status = checkSpStatus
	}

	err = ds.Put(ctx, sp)
	if err != nil {
		return nil, err
	}
	err = createRelatedDBData(ctx, s.Ds, sp, nil, nil)
	return &dto.CreateServiceProviderResponse{
		Bcode: *bcode.ServiceProviderCode,
	}, nil
}

func (s *ServiceProviderImpl) DeleteServiceProvider(ctx context.Context, request *dto.DeleteServiceProviderRequest) (*dto.DeleteServiceProviderResponse, error) {
	sp := new(types.ServiceProvider)
	sp.ProviderName = request.ProviderName

	ds := datastore.GetDefaultDatastore()
	err := ds.Get(ctx, sp)
	if err != nil {
		return nil, err
	}

	if sp.Scope == constants.ProviderScopeSystem {
		return nil, bcode.ErrSystemProviderCannotDelete
	}

	m := new(types.Model)
	m.ProviderName = request.ProviderName
	list, err := ds.List(ctx, m, &datastore.ListOptions{
		Page:     0,
		PageSize: 100,
	})
	if err != nil {
		return nil, err
	}

	if sp.ServiceSource == types.ServiceSourceLocal {
		// Delete the locally downloaded model.
		// It is necessary to check whether the local model is jointly referenced by other service providers.
		// If so, do not delete the local model but only delete the record.
		engine, err := provider.GetModelEngine(sp.Flavor)
		if err != nil {
			logger.EngineLogger.Warn("Failed to get engine", "flavor", sp.Flavor, "error", err)
			// Continue without deleting models
		} else {
			for _, m := range list {
				dsModel := m.(*types.Model)
				tmpModel := &types.Model{
					ModelName: dsModel.ModelName,
				}
				count, err := ds.Count(ctx, tmpModel, &datastore.FilterOptions{})
				if err != nil || count > 1 {
					continue
				}
				if dsModel.Status == "downloaded" {
					delReq := &sdktypes.DeleteRequest{Model: dsModel.ModelName}

					err = engine.DeleteModel(ctx, delReq)
					if err != nil {
						return nil, err
					}
				}

			}
		}
	}

	err = ds.Delete(ctx, m)
	if err != nil {
		return nil, err
	}

	err = ds.Delete(ctx, sp)
	if err != nil {
		return nil, err
	}

	// Check the currently set local and remote service providers. If so, set them to empty.
	service := &types.Service{Name: sp.ServiceName}
	err = ds.Get(ctx, service)
	if err != nil {
		return nil, err
	}
	if sp.ServiceSource == types.ServiceSourceRemote && sp.ProviderName == service.RemoteProvider {
		service.RemoteProvider = ""
		if service.LocalProvider == "" {
			service.Status = 0
		}
	} else if sp.ServiceSource == types.ServiceSourceLocal && sp.ProviderName == service.LocalProvider {
		service.LocalProvider = ""
		if service.RemoteProvider == "" {
			service.Status = 0
		}
	}

	err = ds.Put(ctx, service)
	if err != nil {
		return nil, err
	}

	return &dto.DeleteServiceProviderResponse{
		Bcode: *bcode.ServiceProviderCode,
	}, nil
}

func (s *ServiceProviderImpl) UpdateServiceProvider(ctx context.Context, request *dto.UpdateServiceProviderRequest) (*dto.UpdateServiceProviderResponse, error) {
	ds := datastore.GetDefaultDatastore()
	sp := &types.ServiceProvider{}
	sp.ProviderName = request.ProviderName

	err := ds.Get(ctx, sp)
	if err != nil {
		return nil, err
	}
	providerDefaultInfo := schedule.GetProviderServiceDefaultInfo(sp.Flavor, sp.ServiceName)
	if request.ServiceName != "" {
		sp.ServiceName = request.ServiceName
	}
	if request.ServiceSource != "" {
		sp.ServiceSource = request.ServiceSource
	}
	if request.ApiFlavor != "" {
		sp.Flavor = request.ApiFlavor
	}
	if request.AuthType != "" {
		sp.AuthType = request.AuthType
	}
	if request.AuthKey != "" {
		sp.AuthKey = request.AuthKey
	}
	if request.Desc != "" {
		sp.Desc = request.Desc
	}
	if request.Method != "" {
		sp.Method = request.Method
	}
	if request.Url != "" {
		sp.URL = request.Url
	} else {
		sp.URL = providerDefaultInfo.RequestUrl
	}
	if request.ExtraHeaders != "" {
		sp.ExtraHeaders = request.ExtraHeaders
	} else {
		sp.ExtraHeaders = providerDefaultInfo.ExtraHeaders
	}
	if request.ExtraJsonBody != "" {
		sp.ExtraJSONBody = request.ExtraJsonBody
	}
	if request.Properties != "" {
		sp.Properties = request.Properties
	}
	sp.UpdatedAt = types.Now()

	for _, modelName := range request.Models {
		model := types.Model{ProviderName: sp.ProviderName, ModelName: modelName, ServiceSource: sp.ServiceSource, ServiceName: sp.ServiceName}
		if request.ServiceSource == types.ServiceSourceLocal && request.ProviderName == types.FlavorOllama {
			model = types.Model{ProviderName: sp.ProviderName, ModelName: modelName}
		}

		err = ds.Get(ctx, &model)
		if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
			return nil, err
		}
		server := checker.CreateChecker(*sp, model.ModelName)
		if server == nil {
			model.Status = "failed"
			err = ds.Put(ctx, sp)
			if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
				return nil, err
			} else if errors.Is(err, datastore.ErrEntityInvalid) {
				err = ds.Add(ctx, &model)
				if err != nil {
					return nil, err
				}
			}
			err = ds.Put(ctx, &model)
			if err != nil {
				return nil, err
			}
			return nil, bcode.ErrProviderIsUnavailable
		}
		checkRes := server.CheckServer()
		if !checkRes {
			model.Status = "failed"
			err = ds.Put(ctx, sp)
			if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
				return nil, err
			} else if errors.Is(err, datastore.ErrEntityInvalid) {
				err = ds.Add(ctx, &model)
				if err != nil {
					return nil, err
				}
			}
			err = ds.Put(ctx, &model)
			if err != nil {
				return nil, err
			}
			return nil, bcode.ErrProviderIsUnavailable
		}
		err = ds.Get(ctx, &model)
		model.Status = "downloaded"
		if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
			return nil, err
		} else if errors.Is(err, datastore.ErrEntityInvalid) {
			err = ds.Add(ctx, &model)
			if err != nil {
				return nil, err
			}
		}
		err = ds.Put(ctx, &model)
		if err != nil {
			return nil, err
		}
	}

	err = ds.Put(ctx, sp)
	if err != nil {
		return nil, err
	}

	return &dto.UpdateServiceProviderResponse{
		Bcode: *bcode.ServiceProviderCode,
	}, nil
}

func (s *ServiceProviderImpl) GetServiceProvider(ctx context.Context, request *dto.GetServiceProviderRequest) (*dto.GetServiceProviderResponse, error) {
	sp := new(types.ServiceProvider)
	sp.ProviderName = request.ProviderName
	err := s.Ds.Get(ctx, sp)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, err
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, bcode.ErrProviderNotFound
	}
	data := new(dto.ServiceProvider)
	data.ProviderName = sp.ProviderName
	data.ServiceName = sp.ServiceName
	data.ServiceSource = sp.ServiceSource
	data.Flavor = sp.Flavor
	data.Status = sp.Status

	return &dto.GetServiceProviderResponse{
		Bcode: *bcode.ServiceProviderCode,
		Data:  *data,
	}, nil
}

func (s *ServiceProviderImpl) GetServiceProviders(ctx context.Context, request *dto.GetServiceProvidersRequest) (*dto.GetServiceProvidersResponse, error) {
	sp := new(types.ServiceProvider)
	sp.ServiceName = request.ServiceName
	sp.ProviderName = request.ProviderName
	sp.Flavor = request.ApiFlavor
	sp.ServiceSource = request.ServiceSource

	ds := datastore.GetDefaultDatastore()
	list, err := ds.List(ctx, sp, &datastore.ListOptions{Page: 0, PageSize: 100})
	if err != nil {
		return nil, err
	}
	var spNames []string
	for _, v := range list {
		dsProvider := v.(*types.ServiceProvider)
		spNames = append(spNames, dsProvider.ProviderName)
	}

	inOptions := make([]datastore.InQueryOption, 0)
	inOptions = append(inOptions, datastore.InQueryOption{
		Key:    "provider_name",
		Values: spNames,
	})
	m := new(types.Model)
	mList, err := ds.List(ctx, m, &datastore.ListOptions{
		FilterOptions: datastore.FilterOptions{
			In: inOptions,
		},
		Page:     0,
		PageSize: 10,
	})
	if err != nil {
		return nil, err
	}

	spModels := make(map[string][]string)
	for _, v := range mList {
		dsModel := v.(*types.Model)
		spModels[dsModel.ProviderName] = append(spModels[dsModel.ProviderName], dsModel.ModelName)
	}

	respData := make([]dto.ServiceProvider, 0)
	for _, v := range list {
		dsProvider := v.(*types.ServiceProvider)
		serviceProviderStatus := 0
		if dsProvider.ServiceSource == types.ServiceSourceRemote {
			model := types.Model{
				ProviderName: dsProvider.ProviderName,
			}
			err = ds.Get(ctx, &model)
			// checkServerObj := ChooseCheckServer(*dsProvider, model.ModelName)
			// status := checkServerObj.CheckServer()
			// if status {
			// 	serviceProviderStatus = 1
			// }
			serviceProviderStatus = 1
		} else {
			providerEngine, err := provider.GetModelEngine(dsProvider.Flavor)
			if err != nil {
				logger.EngineLogger.Warn("Failed to get engine", "flavor", dsProvider.Flavor, "error", err)
				serviceProviderStatus = 2
			} else {
				err = providerEngine.HealthCheck(ctx)
				if err == nil {
					serviceProviderStatus = 1
				}
			}
		}

		tmp := &dto.ServiceProvider{
			ProviderName:  dsProvider.ProviderName,
			ServiceName:   dsProvider.ServiceName,
			ServiceSource: dsProvider.ServiceSource,
			Desc:          dsProvider.Desc,
			AuthType:      dsProvider.AuthType,
			AuthKey:       dsProvider.AuthKey,
			Flavor:        dsProvider.Flavor,
			Properties:    dsProvider.Properties,
			Status:        serviceProviderStatus,
			CreatedAt:     dsProvider.CreatedAt,
			UpdatedAt:     dsProvider.UpdatedAt,
		}

		if models, ok := spModels[dsProvider.ProviderName]; ok {
			tmp.Models = models
		}

		respData = append(respData, *tmp)
	}

	return &dto.GetServiceProvidersResponse{
		Bcode: *bcode.ServiceProviderCode,
		Data:  respData,
	}, nil
}

// ChooseCheckServer creates and returns an appropriate service checker
// This function maintains backward compatibility with the existing API
func ChooseCheckServer(sp types.ServiceProvider, modelName string) checker.ServiceChecker {
	return checker.CreateChecker(sp, modelName)
}
