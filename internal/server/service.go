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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/intel/aog/internal/provider"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
)

const (
	// Service status constants
	ServiceStatusError     = -1
	ServiceStatusReady     = 0
	ServiceStatusAvailable = 1
	ServiceStatusCreating  = 2

	// Model status constants
	ModelStatusDownloading = "downloading"
	ModelStatusDownloaded  = "downloaded"
	ModelStatusFailed      = "failed"

	// Service provider status
	ServiceProviderStatusReady     = 0
	ServiceProviderStatusAvailable = 1
)

// Default configuration
var (
	DefaultLocalServiceProperties = `{"max_input_tokens":2048,"supported_response_mode":["stream","sync"],"mode_is_changeable":true,"xpu":["GPU"]}`
)

type AIGCService interface {
	CreateAIGCService(ctx context.Context, request *dto.CreateAIGCServiceRequest) (*dto.CreateAIGCServiceResponse, error)
	UpdateAIGCService(ctx context.Context, request *dto.UpdateAIGCServiceRequest) (*dto.UpdateAIGCServiceResponse, error)
	GetAIGCService(ctx context.Context, request *dto.GetAIGCServiceRequest) (*dto.GetAIGCServiceResponse, error)
	GetAIGCServices(ctx context.Context, request *dto.GetAIGCServicesRequest) (*dto.GetAIGCServicesResponse, error)
	ExportService(ctx context.Context, request *dto.ExportServiceRequest) (*dto.ExportServiceResponse, error)
	ImportService(ctx context.Context, request *dto.ImportServiceRequest) (*dto.ImportServiceResponse, error)
}

type AIGCServiceImpl struct {
	Ds datastore.Datastore
}

func NewAIGCService() AIGCService {
	return &AIGCServiceImpl{
		Ds: datastore.GetDefaultDatastore(),
	}
}

// Helper function to update service status
func (s *AIGCServiceImpl) updateServiceStatus(ctx context.Context, service *types.Service, status int) {
	service.Status = status
	_ = s.Ds.Put(ctx, service) // Keep the original behavior of ignoring errors here
}

// Create remote service
func (s *AIGCServiceImpl) createRemoteService(ctx context.Context, request *dto.CreateAIGCServiceRequest, sp *types.ServiceProvider, m *types.Model, providerServiceInfo schedule.ServiceDefaultInfo, service *types.Service) error {
	// Set URL
	sp.URL = request.Url
	if request.Url == "" {
		sp.URL = providerServiceInfo.RequestUrl
	}

	// Set authentication type (API layer has set default values and validated authentication information completeness)
	sp.AuthType = request.AuthType
	sp.AuthKey = request.AuthKey
	sp.ExtraJSONBody = request.ExtraJsonBody
	sp.ExtraHeaders = request.ExtraHeaders
	if request.ExtraHeaders == "" {
		sp.ExtraHeaders = providerServiceInfo.ExtraHeaders
	}
	sp.Properties = request.Properties

	// Set model information
	m.ServiceSource = types.ServiceSourceRemote
	m.ServiceName = request.ServiceName
	m.ModelName = providerServiceInfo.DefaultModel
	m.ProviderName = sp.ProviderName

	// Runtime service availability check (non-static parameter validation, needs to be retained)
	if sp.AuthType != types.AuthTypeNone {
		checkSp := ChooseCheckServer(*sp, m.ModelName)
		if checkSp == nil {
			return bcode.ErrProviderIsUnavailable
		}
		if !checkSp.CheckServer() {
			return bcode.ErrProviderIsUnavailable
		}
		// Save model
		m.Status = ModelStatusDownloaded
		m.UpdatedAt = time.Now()
		if err := s.Ds.Put(ctx, m); err != nil {
			logger.LogicLogger.Error("Add model error: %s", err.Error())
			return bcode.ErrAddModel
		}
		if err := createRelatedDBData(ctx, s.Ds, sp, m, service); err != nil {
			return err
		}

		// Set service status to ready
		s.updateServiceStatus(ctx, service, ServiceStatusAvailable)

		// Save service provider
		sp.Status = ServiceProviderStatusAvailable
		if err := s.saveServiceProvider(ctx, sp); err != nil {
			return err
		}
	} else {
		s.updateServiceStatus(ctx, service, ServiceStatusReady)

		// Save service provider
		sp.Status = ServiceProviderStatusReady
		if err := s.saveServiceProvider(ctx, sp); err != nil {
			return err
		}
	}
	misExist, err := s.Ds.IsExist(ctx, m)
	if err != nil {
		return err
	}
	// Check for duplicates (remote services don't need to check mIsExist because it's always false)
	return s.checkDuplicateService(ctx, sp, m, misExist)
}

// Common logic for saving service providers
func (s *AIGCServiceImpl) saveServiceProvider(ctx context.Context, sp *types.ServiceProvider) error {
	err := s.Ds.Get(ctx, sp)
	if err == nil {
		err = s.Ds.Put(ctx, sp)
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		err = s.Ds.Add(ctx, sp)
	} else {
		logger.LogicLogger.Error("Check service provider error: %s", err.Error())
		return bcode.ErrAIGCServiceAddProvider
	}
	if err != nil {
		logger.LogicLogger.Error("Add service provider error: %s", err.Error())
		return bcode.ErrAIGCServiceAddProvider
	}
	return nil
}

// Create local service
func (s *AIGCServiceImpl) createLocalService(ctx context.Context, request *dto.CreateAIGCServiceRequest, sp *types.ServiceProvider, m *types.Model, providerServiceInfo schedule.ServiceDefaultInfo, service *types.Service) error {
	// Get recommended configuration (API layer has set default ApiFlavor)
	recommendConfig := getRecommendConfig(request.ServiceName)
	if request.ModelName != "" {
		recommendConfig.ModelName = request.ModelName
	}
	// API layer has ensured ApiFlavor has value, use it directly
	sp.Flavor = request.ApiFlavor

	// Set provider name
	if request.ProviderName == "" {
		sp.ProviderName = fmt.Sprintf("%s_%s_%s", request.ServiceSource, request.ApiFlavor, request.ServiceName)
	}

	// Ensure engine is ready
	if err := provider.EnsureEngineReady(recommendConfig.ModelEngine); err != nil {
		s.updateServiceStatus(ctx, service, ServiceStatusError)
		logger.LogicLogger.Error("Ensure engine ready error: ", err.Error())
		return err
	}

	// Set service status to ready
	s.updateServiceStatus(ctx, service, ServiceStatusReady)

	// Configure service provider
	sp.URL = providerServiceInfo.RequestUrl
	sp.ExtraJSONBody = ""
	sp.ExtraHeaders = ""
	sp.Properties = DefaultLocalServiceProperties
	sp.Status = ServiceProviderStatusReady

	// Handle model-related logic
	mIsExist := false
	if !request.SkipModelFlag {
		var err error
		mIsExist, err = s.handleLocalModelLogic(ctx, request, sp, m, recommendConfig, service)
		if err != nil {
			return err
		}
	}

	// Create related services
	if err := s.createRelatedServices(ctx, request, sp, service); err != nil {
		return err
	}

	// Check for duplicates
	return s.checkDuplicateService(ctx, sp, m, mIsExist)
}

// Handle local model logic
func (s *AIGCServiceImpl) handleLocalModelLogic(ctx context.Context, request *dto.CreateAIGCServiceRequest, sp *types.ServiceProvider, m *types.Model, recommendConfig types.RecommendConfig, service *types.Service) (bool, error) {
	engineProvider := provider.GetModelEngine(recommendConfig.ModelEngine)
	models, err := engineProvider.ListModels(ctx)
	if err != nil {
		logger.LogicLogger.Error("Get "+recommendConfig.ModelEngine+" model list error: ", err.Error())
		return false, bcode.ErrGetEngineModelList
	}

	// Check if model is already pulled
	isPulled := false
	for _, model := range models.Models {
		if model.Name == recommendConfig.ModelName {
			isPulled = true
			break
		}
	}

	// Set basic model information
	m.ProviderName = sp.ProviderName
	m.ModelName = recommendConfig.ModelName
	m.ServiceName = request.ServiceName
	m.ServiceSource = types.ServiceSourceLocal

	// Check model records in database
	mIsExist := false
	err = s.Ds.Get(ctx, m)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		logger.LogicLogger.Error("Get model from db error:", err)
		return false, bcode.ErrServer
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		m.Status = ModelStatusDownloading
		err = s.Ds.Add(ctx, m)
		if err != nil {
			logger.LogicLogger.Error("Add model to db error:", err)
			return false, bcode.ErrAddModel
		}
	} else {
		mIsExist = true
	}

	// Handle model pulling
	if !isPulled {
		if m.Status == ModelStatusFailed {
			m.Status = ModelStatusDownloading
		}
		stream := false
		pullReq := &types.PullModelRequest{
			Model:     recommendConfig.ModelName,
			Stream:    &stream,
			ModelType: sp.ServiceName,
		}
		go AsyncPullModel(sp, m, pullReq)
	} else {
		m.Status = ModelStatusDownloaded
		if err := s.Ds.Put(ctx, m); err != nil {
			return false, bcode.ErrAddModel
		}
		// If model already exists locally, set status to available
		checkServer := ChooseCheckServer(*sp, m.ModelName)
		if checkServer != nil {
			s.updateServiceStatus(ctx, service, ServiceStatusAvailable)
		}
	}

	return mIsExist, nil
}

// Create related services
func (s *AIGCServiceImpl) createRelatedServices(ctx context.Context, request *dto.CreateAIGCServiceRequest, sp *types.ServiceProvider, service *types.Service) error {
	// Get the task type of the current service
	currentServiceInfo := schedule.GetProviderServiceDefaultInfo(request.ApiFlavor, request.ServiceName)
	providerServices := schedule.GetProviderServices(request.ApiFlavor)

	for serviceName, serviceInfo := range providerServices {
		if serviceInfo.TaskType == currentServiceInfo.TaskType && serviceName != request.ServiceName {
			if err := s.createRelatedService(ctx, request, sp, service, serviceName, serviceInfo); err != nil {
				return err
			}
		}
	}
	return nil
}

// Create single related service
func (s *AIGCServiceImpl) createRelatedService(ctx context.Context, request *dto.CreateAIGCServiceRequest, sp *types.ServiceProvider, service *types.Service, serviceName string, serviceInfo schedule.ServiceDefaultInfo) error {
	// Create related service record
	relatedService := &types.Service{Name: serviceName}
	err := s.Ds.Get(ctx, relatedService)
	if err == nil {
		relatedService.Status = service.Status
		_ = s.Ds.Put(ctx, relatedService)
	}

	// Create related service provider
	relatedSp := &types.ServiceProvider{}
	relatedSp.ProviderName = strings.Replace(request.ProviderName, request.ServiceName, serviceName, -1)
	relatedSp.ServiceSource = request.ServiceSource
	relatedSp.AuthType = request.AuthType
	relatedSp.Status = sp.Status
	relatedSp.Method = http.MethodPost
	relatedSp.ServiceName = serviceName
	relatedSp.Desc = strings.Replace(request.Desc, request.ServiceName, serviceName, -1)
	relatedSp.Flavor = request.ApiFlavor
	relatedSp.URL = serviceInfo.RequestUrl

	// Check if related service provider already exists
	// isExist, err := s.Ds.IsExist(ctx, relatedSp)
	// if err != nil {
	// 	isExist = false
	// }

	if err := s.Ds.Put(ctx, relatedSp); err != nil {
		logger.LogicLogger.Error("Service Provider model already exist")
		return bcode.ErrModelIsExist
	}
	return nil
}

// Check for duplicate services
func (s *AIGCServiceImpl) checkDuplicateService(ctx context.Context, sp *types.ServiceProvider, m *types.Model, mIsExist bool) error {
	// Check if service provider already exists
	spIsExist, err := s.Ds.IsExist(ctx, sp)
	if err != nil {
		spIsExist = false
	}

	// Check if the combination of service provider and model already exists
	if spIsExist && mIsExist {
		logger.LogicLogger.Error("Service Provider model already exist")
		return bcode.ErrModelIsExist
	}

	return nil
}

func (s *AIGCServiceImpl) CreateAIGCService(ctx context.Context, request *dto.CreateAIGCServiceRequest) (*dto.CreateAIGCServiceResponse, error) {
	// Initialize service record
	service := &types.Service{Name: request.ServiceName}
	err := s.Ds.Get(ctx, service)
	if err != nil {
		return nil, err
	}
	if service.Status != -1 {
		logger.LogicLogger.Error("Service already installed ", service.Name)
		return nil, bcode.ErrServiceIsInstalled
	}
	s.updateServiceStatus(ctx, service, ServiceStatusCreating)

	// Get provider service information (API layer has validated service type and API flavor validity)
	providerInfo := schedule.GetProviderServices(request.ApiFlavor)
	providerServiceInfo, ok := providerInfo[request.ServiceName]
	if !ok {
		// This is runtime configuration check, still needs to be retained
		logger.LogicLogger.Error("Service " + request.ServiceName + " not supported by " + request.ApiFlavor)
		return nil, bcode.ErrEngineUnSupportService
	}
	if request.ProviderName == "" {
		request.ProviderName = fmt.Sprintf("%s_%s_%s", request.ServiceSource, request.ApiFlavor, request.ServiceName)
	}
	sp := &types.ServiceProvider{
		ProviderName: request.ProviderName,
	}
	err = s.Ds.Get(ctx, sp)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, err
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		err = s.Ds.Add(ctx, sp)
		if err != nil {
			return nil, err
		}
	}
	// Initialize service provider and model objects
	sp.ServiceSource = request.ServiceSource
	sp.ServiceName = request.ServiceName
	sp.Method = request.Method
	sp.Desc = request.Desc
	sp.Flavor = request.ApiFlavor
	if request.Method == "" {
		endpointStr := providerServiceInfo.Endpoints[0]
		method := strings.Split(endpointStr, " ")[0]
		sp.Method = method
	}

	m := &types.Model{
		ProviderName: request.ProviderName,
	}

	// Dispatch processing based on service source type
	if request.ServiceSource == types.ServiceSourceRemote {
		err = s.createRemoteService(ctx, request, sp, m, providerServiceInfo, service)
	} else {
		err = s.createLocalService(ctx, request, sp, m, providerServiceInfo, service)
	}

	if err != nil {
		return nil, err
	}

	return &dto.CreateAIGCServiceResponse{
		Bcode: *bcode.AIGCServiceCode,
	}, nil
}

func (s *AIGCServiceImpl) UpdateAIGCService(ctx context.Context, request *dto.UpdateAIGCServiceRequest) (*dto.UpdateAIGCServiceResponse, error) {
	service := types.Service{
		Name: request.ServiceName,
	}

	err := s.Ds.Get(ctx, &service)
	if err != nil {
		return nil, bcode.ErrServiceRecordNotFound
	}
	service.HybridPolicy = request.HybridPolicy
	err = s.Ds.Put(ctx, &service)
	if err != nil {
		return nil, bcode.ErrServiceRecordNotFound
	}

	return &dto.UpdateAIGCServiceResponse{
		Bcode: *bcode.AIGCServiceCode,
	}, nil
}

func (s *AIGCServiceImpl) GetAIGCService(ctx context.Context, request *dto.GetAIGCServiceRequest) (*dto.GetAIGCServiceResponse, error) {
	return &dto.GetAIGCServiceResponse{}, nil
}

func (s *AIGCServiceImpl) ExportService(ctx context.Context, request *dto.ExportServiceRequest) (*dto.ExportServiceResponse, error) {
	dbService := &types.Service{}
	if request.ServiceName != "" {
		dbService.Name = request.ServiceName
	}
	dbProvider := &types.ServiceProvider{}
	if request.ProviderName != "" {
		dbProvider.ProviderName = request.ProviderName
	}
	model := &types.Model{}
	if request.ModelName != "" {
		model.ModelName = request.ModelName
	}
	dbServices, err := getAllServices(dbService, dbProvider, model)
	if err != nil {
		return nil, err
	}

	return &dto.ExportServiceResponse{
		Version:          version.AOGVersion,
		Services:         dbServices.Services,
		ServiceProviders: dbServices.ServiceProviders,
	}, nil
}

func (s *AIGCServiceImpl) ImportService(ctx context.Context, request *dto.ImportServiceRequest) (*dto.ImportServiceResponse, error) {
	if request.Version != version.AOGVersion {
		return nil, bcode.ErrAIGCServiceVersionNotMatch
	}

	dbService := &types.Service{}
	dbProvider := &types.ServiceProvider{}
	model := &types.Model{}

	dbServices, err := getAllServices(dbService, dbProvider, model)
	if err != nil {
		return nil, err
	}

	for serviceName, service := range request.Services {
		if !utils.Contains(types.SupportService, serviceName) {
			return nil, bcode.ErrUnSupportAIGCService
		}

		if !utils.Contains(types.SupportHybridPolicy, service.HybridPolicy) {
			return nil, bcode.ErrUnSupportHybridPolicy
		}

		if service.ServiceProviders.Local == "" && service.ServiceProviders.Remote == "" {
			return nil, bcode.ErrAIGCServiceBadRequest
		}

		if service.HybridPolicy != types.HybridPolicyDefault && service.HybridPolicy != "" {
			tmpService := dbServices.Services[serviceName]
			tmpService.HybridPolicy = service.HybridPolicy
			dbServices.Services[serviceName] = tmpService
		}
	}

	for providerName, p := range request.ServiceProviders {
		if !utils.Contains(types.SupportFlavor, p.APIFlavor) {
			return nil, bcode.ErrUnSupportFlavor
		}
		if !utils.Contains(types.SupportAuthType, p.AuthType) {
			return nil, bcode.ErrUnSupportAuthType
		}

		if !utils.Contains(types.SupportService, p.ServiceName) {
			return nil, bcode.ErrUnSupportAIGCService
		}

		//if len(p.Models) == 0 && p.ServiceName != types.ServiceModels {
		//	return nil, bcode.ErrProviderModelEmpty
		//}
		providerDefaultInfo := schedule.GetProviderServiceDefaultInfo(p.APIFlavor, p.ServiceName)
		tmpSp := &types.ServiceProvider{}
		tmpSp.ProviderName = providerName
		tmpSp.AuthKey = p.AuthKey
		tmpSp.AuthType = p.AuthType
		tmpSp.Desc = p.Desc
		tmpSp.Flavor = p.APIFlavor
		tmpSp.Method = p.Method
		tmpSp.ServiceName = p.ServiceName
		tmpSp.ServiceSource = p.ServiceSource
		tmpSp.URL = p.URL
		if p.URL == "" {
			tmpSp.URL = providerDefaultInfo.RequestUrl
		}
		tmpSp.Status = 0
		tmpSp.ExtraHeaders = providerDefaultInfo.ExtraHeaders
		tmpSp.ExtraJSONBody = "{}"
		tmpSp.Properties = "{}"
		if p.ServiceName == types.ServiceChat || p.ServiceName == types.ServiceGenerate {
			tmpSp.Properties = `{"max_input_tokens":2048,"supported_response_mode":["stream","sync"],"mode_is_changeable":true,"xpu":["GPU"]}`
		}

		// engineProvider := provider.GetModelEngine(tmpSp.Flavor)
		for _, m := range p.Models {
			if p.ServiceSource == types.ServiceSourceLocal && !utils.Contains(dbServices.ServiceProviders[providerName].Models, m) {
				logger.LogicLogger.Info(fmt.Sprintf("Pull model %s start ...", m))
				stream := false
				pullReq := &types.PullModelRequest{
					Model:     m,
					Stream:    &stream,
					ModelType: p.ServiceName,
				}
				modelObj := new(types.Model)
				modelObj.ProviderName = tmpSp.ProviderName
				modelObj.ModelName = m

				err = s.Ds.Get(ctx, modelObj)
				if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
					// todo debug log output
					return nil, bcode.ErrServer
				} else if errors.Is(err, datastore.ErrEntityInvalid) {
					modelObj.Status = "downloading"
					err = s.Ds.Put(ctx, modelObj)
					if err != nil {
						return nil, bcode.ErrAddModel
					}
				}
				if modelObj.Status == "failed" {
					modelObj.Status = "downloading"
				}
				go AsyncPullModel(tmpSp, modelObj, pullReq)
				//_, err := engineProvider.PullModel(ctx, pullReq, nil)
				//if err != nil {
				//	slog.Error(fmt.Sprintf("Pull model error: %s", err.Error()))
				//	return nil, bcode.ErrEnginePullModel
				//}
				//
				//slog.Info(fmt.Sprintf("Pull model %s completed ...", m))
			} else if p.ServiceSource == types.ServiceSourceRemote && !utils.Contains(dbServices.ServiceProviders[providerName].Models, m) {
				server := ChooseCheckServer(*tmpSp, m)
				checkRes := server.CheckServer()
				if !checkRes {
					return nil, bcode.ErrProviderIsUnavailable
				}
				tmpSp.Status = 1
				tmpModel := &types.Model{}
				tmpModel.ModelName = m
				tmpModel.ProviderName = tmpSp.ProviderName
				tmpModel.Status = "downloaded"
				tmpModel.UpdatedAt = time.Now()

				isExist, err := s.Ds.IsExist(ctx, tmpModel)
				if err != nil || !isExist {
					err := s.Ds.Put(ctx, tmpModel)
					if err != nil {
						return nil, bcode.ErrAddModel
					}
				}
			}
		}

		if _, ok := dbServices.ServiceProviders[providerName]; ok {
			checkSp := &types.ServiceProvider{
				ProviderName: providerName,
			}
			err = s.Ds.Get(ctx, checkSp)
			if err != nil {
				return nil, bcode.ErrProviderUpdateFailed
			}
			tmpSp.ID = checkSp.ID
			tmpSp.Status = checkSp.Status
			if tmpSp.AuthType == "none" {
				tmpSp.AuthType = checkSp.AuthType
				tmpSp.AuthKey = checkSp.AuthKey
			}

			err := s.Ds.Put(ctx, tmpSp)
			if err != nil {
				return nil, bcode.ErrProviderUpdateFailed
			}

		} else {
			err := s.Ds.Put(ctx, tmpSp)
			if err != nil {
				return nil, err
			}
		}

		if p.ServiceSource == types.ServiceSourceLocal && p.ServiceName == types.ServiceChat {
			generateProviderServiceInfo := schedule.GetProviderServiceDefaultInfo(tmpSp.Flavor, types.ServiceGenerate)
			generateSp := &types.ServiceProvider{}
			generateSp.ProviderName = strings.Replace(tmpSp.ProviderName, "chat", "generate", -1)
			generateSp.ServiceSource = tmpSp.ServiceSource
			generateSp.AuthType = tmpSp.AuthType
			generateSp.Status = tmpSp.Status
			generateSp.Method = http.MethodPost
			generateSp.ServiceName = strings.Replace(tmpSp.ServiceName, "chat", "generate", -1)
			generateSp.Desc = strings.Replace(tmpSp.Desc, "chat", "generate", -1)
			generateSp.Flavor = tmpSp.Flavor
			generateSp.URL = generateProviderServiceInfo.RequestUrl
			generateSp.Properties = tmpSp.Properties
			generateSp.ExtraJSONBody = tmpSp.ExtraJSONBody
			generateSp.ExtraHeaders = tmpSp.ExtraHeaders

			generateSpIsExist, err := s.Ds.IsExist(ctx, generateSp)
			if err != nil {
				generateSpIsExist = false
			}

			if !generateSpIsExist {
				err := s.Ds.Put(ctx, generateSp)
				if err != nil {
					logger.LogicLogger.Error("Service Provider model already exist")
					return nil, bcode.ErrModelIsExist
				}
			}

		}

		// Check whether LocalProvider and RemoteProvider exist in DBServices. If they do not exist, add them.
		dbService.Name = p.ServiceName
		err := s.Ds.Get(ctx, dbService)
		if err != nil {
			return nil, err
		}
		dbService.HybridPolicy = dbServices.Services[p.ServiceName].HybridPolicy

		err = s.Ds.Put(ctx, dbService)
		if err != nil {
			return nil, bcode.ErrServiceUpdateFailed
		}
	}

	return &dto.ImportServiceResponse{
		Bcode: *bcode.AIGCServiceCode,
	}, nil
}

func (s *AIGCServiceImpl) GetAIGCServices(ctx context.Context, request *dto.GetAIGCServicesRequest) (*dto.GetAIGCServicesResponse, error) {
	service := &types.Service{}
	if request.ServiceName != "" {
		service.Name = request.ServiceName
	}

	list, err := s.Ds.List(ctx, service, &datastore.ListOptions{PageSize: 10, Page: 0})
	if err != nil {
		return nil, err
	}

	respData := make([]dto.Service, 0)

	for _, v := range list {
		tmp := dto.Service{}

		dsService := v.(*types.Service)
		tmp.ServiceName = dsService.Name
		serviceStatus := 1
		tmp.HybridPolicy = dsService.HybridPolicy
		tmp.Status = serviceStatus
		tmp.UpdatedAt = dsService.UpdatedAt
		tmp.CreatedAt = dsService.CreatedAt

		respData = append(respData, tmp)
	}

	return &dto.GetAIGCServicesResponse{
		Bcode: *bcode.AIGCServiceCode,
		Data:  respData,
	}, nil
}

func getAllServices(service *types.Service, provider *types.ServiceProvider, model *types.Model) (*dto.ImportServiceRequest, error) {
	ds := datastore.GetDefaultDatastore()

	serviceList, err := ds.List(context.Background(), service, &datastore.ListOptions{Page: 0, PageSize: 10})
	if err != nil {
		return nil, bcode.ErrAIGCServiceBadRequest
	}

	providerList, err := ds.List(context.Background(), provider, &datastore.ListOptions{Page: 0, PageSize: 100})
	if err != nil {
		return nil, bcode.ErrProviderInvalid
	}

	modelList, err := ds.List(context.Background(), model, &datastore.ListOptions{Page: 0, PageSize: 100})
	if err != nil {
		return nil, bcode.ErrModelRecordNotFound
	}

	dbServices := new(dto.ImportServiceRequest)
	dbServices.Services = make(map[string]dto.ServiceEntry)
	dbServices.ServiceProviders = make(map[string]dto.ServiceProviderEntry)
	for _, v := range serviceList {
		tmp := v.(*types.Service)
		tmpService := dbServices.Services[tmp.Name]
		tmpService.HybridPolicy = tmp.HybridPolicy
		dm := new(types.Model)
		dm.ModelName = model.ModelName
		localDefaultModelList, err := ds.List(context.Background(), dm, &datastore.ListOptions{
			FilterOptions: datastore.FilterOptions{
				Queries: []datastore.FuzzyQueryOption{
					{Key: "IsDefault", Query: "true"},
					{Key: "service_name", Query: tmp.Name},
					{Key: "service_source", Query: types.ServiceSourceLocal},
				},
			},
			Page: 0, PageSize: 100})
		if err == nil && len(localDefaultModelList) > 0 {
			localDmObj := localDefaultModelList[0].(*types.Model)
			tmpService.ServiceProviders.Local = localDmObj.ProviderName
		}
		remoteDefaultModelList, err := ds.List(context.Background(), dm, &datastore.ListOptions{
			FilterOptions: datastore.FilterOptions{
				Queries: []datastore.FuzzyQueryOption{
					{Key: "IsDefault", Query: "true"},
					{Key: "service_name", Query: tmp.Name},
					{Key: "service_source", Query: types.ServiceSourceRemote},
				},
			},
			Page: 0, PageSize: 100})
		if err == nil && len(remoteDefaultModelList) > 0 {
			RemoteDmObj := localDefaultModelList[0].(*types.Model)
			tmpService.ServiceProviders.Remote = RemoteDmObj.ProviderName
		}
		dbServices.Services[tmp.Name] = tmpService
	}

	for _, v := range providerList {
		tmp := v.(*types.ServiceProvider)
		tmpProvider := dbServices.ServiceProviders[tmp.ProviderName]
		tmpProvider.AuthKey = tmp.AuthKey
		tmpProvider.AuthType = tmp.AuthType
		tmpProvider.Desc = tmp.Desc
		tmpProvider.APIFlavor = tmp.Flavor
		tmpProvider.Method = tmp.Method
		tmpProvider.ServiceName = tmp.ServiceName
		tmpProvider.ServiceSource = tmp.ServiceSource
		tmpProvider.URL = tmp.URL
		tmpProvider.Models = []string{}
		for _, m := range modelList {
			if m.(*types.Model).ProviderName == tmp.ProviderName {
				tmpProvider.Models = append(tmpProvider.Models, m.(*types.Model).ModelName)
			}
		}
		dbServices.ServiceProviders[tmp.ProviderName] = tmpProvider
	}

	return dbServices, nil
}

func getRecommendConfig(service string) types.RecommendConfig {
	recommendModelMap, _ := RecommendModels()
	recommendModelList := recommendModelMap[service]
	switch service {
	case types.ServiceChat:
		modelName := constants.DefaultChatModelName
		if len(recommendModelList) > 0 {
			modelName = recommendModelList[0].Name
		}
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   modelName,
		}
	case types.ServiceEmbed:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   constants.DefaultEmbedModelName,
		}
	case types.ServiceModels:
		return types.RecommendConfig{}
	case types.ServiceGenerate:
		modelName := constants.DefaultChatModelName
		if len(recommendModelList) > 0 {
			modelName = recommendModelList[0].Name
		}
		return types.RecommendConfig{
			ModelEngine: types.FlavorOllama,
			ModelName:   modelName,
		}
	case types.ServiceTextToImage:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultTextToImageModel,
		}
	case types.ServiceSpeechToText:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultSpeechToTextModel,
		}
	case types.ServiceSpeechToTextWS:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultSpeechToTextModel,
		}
	case types.ServiceTextToSpeech:
		return types.RecommendConfig{
			ModelEngine: types.FlavorOpenvino,
			ModelName:   constants.DefaultTextToSpeechModel,
		}
	case types.ServiceImageToImage:
		return types.RecommendConfig{
			ModelEngine: types.FlavorAliYun,
			ModelName:   constants.DefaultImageToImageModel,
		}
	case types.ServiceImageToVideo:
		return types.RecommendConfig{
			ModelEngine: types.FlavorAliYun,
			ModelName:   constants.DefaultImageToVideoModel,
		}
	default:
		return types.RecommendConfig{}
	}
}
