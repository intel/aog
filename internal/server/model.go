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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/provider/engine"
	"github.com/intel/aog/internal/provider/template"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
)

type Model interface {
	CreateModel(ctx context.Context, request *dto.CreateModelRequest) (*dto.CreateModelResponse, error)
	DeleteModel(ctx context.Context, request *dto.DeleteModelRequest) (*dto.DeleteModelResponse, error)
	GetModels(ctx context.Context, request *dto.GetModelsRequest) (*dto.GetModelsResponse, error)
	CreateModelStream(ctx context.Context, request *dto.CreateModelRequest) (chan []byte, chan error)
	ModelStreamCancel(ctx context.Context, req *dto.ModelStreamCancelRequest) (*dto.ModelStreamCancelResponse, error)
	GetRecommendModel() (*dto.RecommendModelResponse, error)
	GetSupportModelList(ctx context.Context, request *dto.GetSupportModelRequest) (*dto.GetSupportModelResponse, error)
}

type ModelImpl struct {
	Ds  datastore.Datastore
	JDs datastore.JsonDatastore
}

func NewModel() Model {
	return &ModelImpl{
		Ds:  datastore.GetDefaultDatastore(),
		JDs: datastore.GetDefaultJsonDatastore(),
	}
}

func (s *ModelImpl) CreateModel(ctx context.Context, request *dto.CreateModelRequest) (*dto.CreateModelResponse, error) {
	// ensure service avaliable first
	service := &types.Service{Name: request.ServiceName}
	err := s.Ds.Get(ctx, service)
	if err != nil {
		return nil, bcode.ErrServiceRecordNotFound
	}
	if service.Status == -1 {
		return nil, bcode.ErrModelServiceNotAvailable
	}

	sp := new(types.ServiceProvider)
	if request.ProviderName != "" {
		sp.ProviderName = request.ProviderName
	} else {
		// get default service provider
		if request.ServiceName != types.ServiceChat && request.ServiceName != types.ServiceGenerate && request.ServiceName != types.ServiceEmbed &&
			request.ServiceName != types.ServiceTextToImage {
			return nil, bcode.ErrServer
		}

		service := &types.Service{}
		service.Name = request.ServiceName

		err := s.Ds.Get(ctx, service)
		if err != nil {
			return nil, err
		}

		if request.ServiceSource == types.ServiceSourceLocal && service.LocalProvider != "" {
			sp.ProviderName = service.LocalProvider
		} else if request.ServiceSource == types.ServiceSourceRemote && service.RemoteProvider != "" {
			sp.ProviderName = service.RemoteProvider
		}
	}

	sp.ServiceName = request.ServiceName
	sp.ServiceSource = request.ServiceSource

	err = s.Ds.Get(ctx, sp)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		// todo debug log output
		return nil, bcode.ErrServer
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, bcode.ErrServiceRecordNotFound
	}

	m := new(types.Model)
	m.ProviderName = sp.ProviderName
	m.ModelName = request.ModelName
	m.ServiceName = request.ServiceName
	m.ServiceSource = request.ServiceSource

	err = s.Ds.Get(ctx, m)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		// todo debug log output
		return nil, bcode.ErrServer
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		m.Status = "downloading"
		err = s.Ds.Add(ctx, m)
		if err != nil {
			return nil, bcode.ErrAddModel
		}
	}
	if m.Status == "failed" {
		m.Status = "downloading"
	}
	stream := false
	pullReq := &types.PullModelRequest{
		Model:     request.ModelName,
		Stream:    &stream,
		ModelType: sp.ServiceName,
	}
	go AsyncPullModel(sp, m, pullReq)

	return &dto.CreateModelResponse{
		Bcode: *bcode.ModelCode,
	}, nil
}

func (s *ModelImpl) DeleteModel(ctx context.Context, request *dto.DeleteModelRequest) (*dto.DeleteModelResponse, error) {
	sp := new(types.ServiceProvider)
	sp.ProviderName = request.ProviderName

	err := s.Ds.Get(ctx, sp)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		// todo err debug log output
		return nil, bcode.ErrServer
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, bcode.ErrServiceRecordNotFound
	}

	m := new(types.Model)
	m.ProviderName = request.ProviderName
	m.ModelName = request.ModelName

	err = s.Ds.Get(ctx, m)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		// todo err debug log output
		return nil, bcode.ErrServer
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		return nil, bcode.ErrModelRecordNotFound
	}

	// Call engin to delete model.
	if m.Status == "downloaded" {
		modelEngine := provider.GetModelEngine(sp.Flavor)
		deleteReq := &types.DeleteRequest{
			Model: request.ModelName,
		}

		err = modelEngine.DeleteModel(ctx, deleteReq)
		if err != nil {
			// todo err debug log output
			return nil, bcode.ErrEngineDeleteModel
		}
	}

	err = s.Ds.Delete(ctx, m)
	if err != nil {
		// todo err debug log output
		return nil, err
	}
	if request.ServiceName == types.ServiceChat {
		generateM := types.Model{
			ProviderName: strings.Replace(request.ProviderName, "chat", "generate", -1),
			ModelName:    m.ModelName,
		}
		err = s.Ds.Get(ctx, &generateM)
		if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
			return nil, err
		}
		err = s.Ds.Delete(ctx, &generateM)
		if err != nil {
			return nil, err
		}
	}

	return &dto.DeleteModelResponse{
		Bcode: *bcode.ModelCode,
	}, nil
}

func (s *ModelImpl) GetModels(ctx context.Context, request *dto.GetModelsRequest) (*dto.GetModelsResponse, error) {
	m := &types.Model{}
	if request.ModelName != "" {
		m.ModelName = request.ModelName
	}
	if request.ProviderName != "" {
		m.ProviderName = request.ProviderName
	}
	list, err := s.Ds.List(ctx, m, &datastore.ListOptions{
		Page:     0,
		PageSize: 1000,
	})
	if err != nil {
		return nil, err
	}

	respData := make([]dto.Model, 0)
	for _, v := range list {
		tmp := new(dto.Model)
		dsModel := v.(*types.Model)

		tmp.ModelName = dsModel.ModelName
		tmp.ProviderName = dsModel.ProviderName
		tmp.Status = dsModel.Status
		tmp.CreatedAt = dsModel.CreatedAt
		tmp.UpdatedAt = dsModel.UpdatedAt
		tmp.ServiceName = dsModel.ServiceName
		tmp.ServiceSource = dsModel.ServiceSource
		tmp.IsDefault = dsModel.IsDefault

		respData = append(respData, *tmp)
	}

	return &dto.GetModelsResponse{
		Bcode: *bcode.ModelCode,
		Data:  respData,
	}, nil
}

func (s *ModelImpl) CreateModelStream(ctx context.Context, request *dto.CreateModelRequest) (chan []byte, chan error) {
	newDataChan := make(chan []byte, 100)
	newErrChan := make(chan error, 1)
	defer close(newDataChan)
	defer close(newErrChan)
	ds := datastore.GetDefaultDatastore()
	sp := new(types.ServiceProvider)

	service := &types.Service{}
	service.Name = request.ServiceName

	err := ds.Get(ctx, service)
	if err != nil {
		newErrChan <- err
		return newDataChan, newErrChan
	}
	if request.ProviderName != "" {
		sp.ProviderName = request.ProviderName
	} else {
		// get default service provider
		// todo Currently only chat and generate services support pulling models.
		if request.ServiceName != types.ServiceChat && request.ServiceName != types.ServiceGenerate && request.ServiceName != types.ServiceEmbed && request.ServiceName != types.ServiceTextToImage && request.ServiceName != types.ServiceSpeechToText {
			newErrChan <- bcode.ErrServer
			return newDataChan, newErrChan
		}
	}

	if request.Size != "" {
		// 判断剩余空间
		providerEngine := provider.GetModelEngine(sp.Flavor)
		var modelSavePath string
		switch eng := providerEngine.(type) {
		case *engine.OpenvinoProvider:
			modelSavePath = fmt.Sprintf("%s/models", eng.EngineConfig.EnginePath)
		case *engine.OllamaProvider:
			modelSavePath = eng.EngineConfig.DownloadPath
		}
		modelSizeGB := utils.ParseSizeToGB(request.Size)
		diskInfo, err := utils.SystemDiskSize(modelSavePath)
		if err != nil {
			newErrChan <- fmt.Errorf("Cannot Get Disk Size: %w", err)
			return newDataChan, newErrChan
		}
		if float64(diskInfo.FreeSize) < modelSizeGB {
			newErrChan <- fmt.Errorf("No enough disk space, need at least %.2f GB", modelSizeGB)
			return newDataChan, newErrChan
		}
	}

	err = ds.Get(ctx, sp)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		// todo debug log output
		newErrChan <- err
		return newDataChan, newErrChan
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		newErrChan <- err
		return newDataChan, newErrChan
	}
	m := new(types.Model)
	m.ModelName = request.ModelName
	m.ProviderName = sp.ProviderName
	m.ServiceName = request.ServiceName
	m.ServiceSource = request.ServiceSource

	err = ds.Get(ctx, m)
	if err != nil && !errors.Is(err, datastore.ErrEntityInvalid) {
		newErrChan <- err
	} else if errors.Is(err, datastore.ErrEntityInvalid) {
		m.Status = "downloading"
		err = ds.Add(ctx, m)
		if err != nil {
			newErrChan <- err
		}
	}
	modelName := request.ModelName
	// todo
	sp.Flavor = strings.Split(sp.ProviderName, "_")[1]
	providerEngine := provider.GetModelEngine(sp.Flavor)
	steam := true
	req := types.PullModelRequest{
		Model:     modelName,
		Stream:    &steam,
		ModelType: request.ServiceName,
	}
	dataChan, errChan := providerEngine.PullModelStream(ctx, &req)

	newDataCh := make(chan []byte, 100)
	newErrorCh := make(chan error, 1)
	go func() {
		defer close(newDataCh)
		defer close(newErrorCh)
		for {
			select {
			case data, ok := <-dataChan:
				if !ok {
					if data == nil {
						client.ModelClientMap[request.ModelName] = nil
						return
					}
				}

				var errResp map[string]interface{}
				if err := json.Unmarshal(data, &errResp); err != nil {
					continue
				}
				if _, ok := errResp["error"]; ok {
					m.Status = "failed"
					err = ds.Put(ctx, m)
					if err != nil {
						newErrorCh <- err
					}
					newErrorCh <- errors.New(string(data))
					return
				}
				var resp types.ProgressResponse
				if err := json.Unmarshal(data, &resp); err != nil {
					log.Printf("Error unmarshaling response: %v", err)

					continue
				}

				if resp.Completed > 0 || resp.Status == "success" {
					if resp.Status == "success" {
						logger.LogicLogger.Error("[Pull Model Stream] accept success label")
						m.Status = "downloaded"
						err = ds.Put(ctx, m)
						if err != nil {
							newErrorCh <- err
							logger.LogicLogger.Error("[Pull Model Stream] put model status failed")
							return
						}
						if service.Status != 1 {
							service.Status = 1
							_ = ds.Put(ctx, service)
						}
						err = createRelatedModels(ctx, s.Ds, sp, m, service, false)
						if err != nil {
							newErrorCh <- err
							return
						}
						logger.LogicLogger.Error("[Pull Model Stream] put model status success")

					}
					newDataCh <- data
				}

			case err, ok := <-errChan:
				if !ok {
					return
				}
				logger.LogicLogger.Info(fmt.Sprintf("Error: %v", err))
				client.ModelClientMap[request.ModelName] = nil
				if err != nil && strings.Contains(err.Error(), "context cancel") {
					if strings.Contains(err.Error(), "context cancel") {
						newErrorCh <- err
						return
					} else {
						m.Status = "failed"
						err = ds.Put(ctx, m)
						if err != nil {
							newErrorCh <- err
						}
						return
					}
				}
			case <-ctx.Done():
				newErrorCh <- ctx.Err()
			}
		}
	}()
	return newDataCh, newErrorCh
}

func (s *ModelImpl) ModelStreamCancel(ctx context.Context, req *dto.ModelStreamCancelRequest) (*dto.ModelStreamCancelResponse, error) {
	modelClientCancelArray := client.ModelClientMap[req.ModelName]
	if modelClientCancelArray != nil {
		for _, cancel := range modelClientCancelArray {
			cancel()
		}
	}
	return &dto.ModelStreamCancelResponse{
		Bcode: *bcode.ModelCode,
	}, nil
}

func AsyncPullModel(sp *types.ServiceProvider, m *types.Model, pullReq *types.PullModelRequest) {
	ctx := context.Background()
	ds := datastore.GetDefaultDatastore()
	modelEngine := provider.GetModelEngine(sp.Flavor)
	_, err := modelEngine.PullModel(ctx, pullReq, nil)
	if err != nil {
		logger.LogicLogger.Error("[Pull model] Pull model error: ", err.Error())
		m.Status = "failed"
		err = ds.Put(ctx, m)
		if err != nil {
			return
		}
		return
	}
	logger.LogicLogger.Info("Pull model %s completed ..." + m.ModelName)

	m.Status = "downloaded"
	err = ds.Put(ctx, m)
	if err != nil {
		logger.LogicLogger.Error("[Pull model] Update model error:", err.Error())
		return
	}

	// 查询该 service 下所有模型
	service := &types.Service{Name: sp.ServiceName}
	err = ds.Get(ctx, service)
	if err != nil {
		logger.LogicLogger.Error("[Pull model] Get service error:", err.Error())
		return
	}
	modelList, err := ds.List(ctx, &types.Model{ServiceName: sp.ServiceName}, &datastore.ListOptions{})
	if err != nil {
		logger.LogicLogger.Error("[Pull model] List models error:", err.Error())
		return
	}

	// 查找是否有默认模型
	var hasDefault bool
	var defaultModel *types.Model
	for _, v := range modelList {
		model := v.(*types.Model)
		if model.IsDefault {
			hasDefault = true
			defaultModel = model
			break
		}
	}

	if hasDefault {
		// 只校验默认模型状态
		checkServer := ChooseCheckServer(*sp, defaultModel.ModelName)
		if checkServer != nil && checkServer.CheckServer() {
			service.Status = 1
			sp.Status = 1
		}
	} else {
		// 没有默认模型则依次校验
		for _, v := range modelList {
			model := v.(*types.Model)
			checkServer := ChooseCheckServer(*sp, model.ModelName)
			if checkServer != nil && checkServer.CheckServer() {
				service.Status = 1
				sp.Status = 1
				break
			}
		}
	}

	err = ds.Put(ctx, service)
	if err != nil {
		logger.LogicLogger.Error("[Pull model] Update service status error:", err.Error())
	}

	err = ds.Put(ctx, sp)
	if err != nil {
		logger.LogicLogger.Error("[Pull model] Update service provider error: ", err.Error())
		return
	}

	if err := createRelatedModels(ctx, ds, sp, m, service, false); err != nil {
		return
	}
}

// create models for related services
func createRelatedModels(ctx context.Context, ds datastore.Datastore, sp *types.ServiceProvider, m *types.Model, service *types.Service, skipModel bool) error {
	currentServiceInfo := schedule.GetProviderServiceDefaultInfo(sp.Flavor, sp.ServiceName)
	providerServices := schedule.GetProviderServices(sp.Flavor)

	for serviceName, serviceInfo := range providerServices {
		if serviceInfo.TaskType == currentServiceInfo.TaskType && serviceName != sp.ServiceName {
			if !skipModel {
				relatedM := &types.Model{
					ModelName:     m.ModelName,
					ProviderName:  strings.Replace(sp.ProviderName, sp.ServiceName, serviceName, 1),
					ServiceName:   serviceName,
					ServiceSource: sp.ServiceSource,
					Status:        m.Status,
				}

				err := ds.Put(ctx, relatedM)
				if err != nil {
					logger.LogicLogger.Error("Add related model error: %s", err.Error())
					return err
				}
			}
			// update relate service
			relatedS := &types.Service{
				Name:         serviceName,
				Status:       service.Status,
				HybridPolicy: service.HybridPolicy,
				CanInstall:   service.CanInstall,
				Avatar:       service.Avatar,
			}
			err := ds.Put(ctx, relatedS)
			if err != nil {
				logger.LogicLogger.Error("Add related service error: %s", err.Error())
				return err
			}

			// update relate service_provider
			relatedSp := &types.ServiceProvider{
				ProviderName: strings.Replace(sp.ProviderName, sp.ServiceName, serviceName, 1),
			}
			err = ds.Get(ctx, relatedSp)
			if err != nil && !errors.Is(err, datastore.ErrRecordNotExist) {
				logger.LogicLogger.Error("Add related service error: %s", err.Error())
				return err
			} else if errors.Is(err, datastore.ErrRecordNotExist) {
				relatedSp.Scope = sp.Scope
				relatedSp.ServiceName = serviceName
				relatedSp.Flavor = sp.Flavor
				relatedSp.ExtraJSONBody = "{}"
				relatedSp.URL = serviceInfo.RequestUrl
				relatedSp.ExtraHeaders = serviceInfo.ExtraHeaders
				err := ds.Add(ctx, relatedSp)
				if err != nil {
					logger.LogicLogger.Error("Add related service provider error: %s", err.Error())
					return err
				}
			}
			relatedSp.Status = sp.Status
			err = ds.Put(ctx, relatedS)
			if err != nil {
				logger.LogicLogger.Error("Add related service provider error: %s", err.Error())
				return err
			}

		}
	}
	return nil
}

type RecommendServicesInfo struct {
	Service             string             `json:"service"`
	MemoryModelsMapList []MemoryModelsInfo `json:"memory_size_models_map_list"`
}

type MemoryModelsInfo struct {
	MemorySize int                      `json:"memory_size"`
	MemoryType []string                 `json:"memory_type"`
	Models     []dto.RecommendModelData `json:"models"`
}

func RecommendModels() (map[string][]dto.RecommendModelData, error) {
	recommendModelDataMap := make(map[string][]dto.RecommendModelData)
	memoryInfo, err := utils.GetMemoryInfo()
	if err != nil {
		return nil, err
	}
	fileContent, err := template.FlavorTemplateFs.ReadFile("recommend_models.json")
	if err != nil {
		fmt.Printf("Read file failed: %v\n", err)
		return nil, err
	}
	// parse struct
	var serviceModelInfo RecommendServicesInfo
	err = json.Unmarshal(fileContent, &serviceModelInfo)
	if err != nil {
		fmt.Printf("Parse JSON failed: %v\n", err)
		return nil, err
	}
	// Windows system needs to include memory module model detection.
	if runtime.GOOS == "windows" {
		windowsVersion := utils.GetSystemVersion()
		if windowsVersion < 10 {
			logger.LogicLogger.Error("[Model] windows version < 10")
			return nil, bcode.ErrNoRecommendModel
		}
		memoryTypeStatus := false
		for _, memoryModel := range serviceModelInfo.MemoryModelsMapList {
			for _, mt := range memoryModel.MemoryType {
				if memoryInfo.MemoryType == mt {
					memoryTypeStatus = true
					break
				}
			}
			if (memoryModel.MemorySize < memoryInfo.Size) && memoryTypeStatus {
				recommendModelDataMap[serviceModelInfo.Service] = memoryModel.Models
				return recommendModelDataMap, nil
			}
		}

	} else {
		// Non-Windows systems determine based only on memory size.
		for _, memoryModel := range serviceModelInfo.MemoryModelsMapList {
			if memoryModel.MemorySize < memoryInfo.Size {
				recommendModelDataMap[serviceModelInfo.Service] = memoryModel.Models
				return recommendModelDataMap, nil
			}
		}
	}

	return nil, err
}

func (s *ModelImpl) GetRecommendModel() (*dto.RecommendModelResponse, error) {
	recommendModel, err := RecommendModels()
	if err != nil {
		return &dto.RecommendModelResponse{Data: nil}, err
	}
	return &dto.RecommendModelResponse{Bcode: *bcode.ModelCode, Data: recommendModel}, nil
}

func (s *ModelImpl) GetSupportModelList(ctx context.Context, request *dto.GetSupportModelRequest) (*dto.GetSupportModelResponse, error) {
	page := request.Page
	if page == 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize == 0 {
		pageSize = 10
	}
	var resData dto.GetSupportModelResponseData
	resData.PageSize = pageSize
	resData.Page = page
	resultList := []dto.RecommendModelData{}
	queryOpList := []datastore.FuzzyQueryOption{}
	if request.Flavor != "" {
		queryOpList = append(queryOpList, datastore.FuzzyQueryOption{
			Key:   "flavor",
			Query: request.Flavor,
		})
	}
	if request.ServiceSource != "" {
		queryOpList = append(queryOpList, datastore.FuzzyQueryOption{
			Key:   "service_source",
			Query: request.ServiceSource,
		})
	}
	sm := &types.SupportModel{}
	sortOption := []datastore.SortOption{
		{Key: "name", Order: 1},
	}
	options := &datastore.ListOptions{FilterOptions: datastore.FilterOptions{Queries: queryOpList}, SortBy: sortOption}
	totalCount, err := s.JDs.Count(ctx, sm, &datastore.FilterOptions{Queries: queryOpList})
	if err != nil {
		return nil, err
	}
	resData.Total = int(totalCount)
	if int(totalCount)%pageSize == 0 {
		resData.TotalPage = int(totalCount) / pageSize
	} else {
		resData.TotalPage = int(totalCount)/pageSize + 1
	}
	if resData.TotalPage == 0 {
		resData.TotalPage = 1
	}
	options.Page = page
	options.PageSize = pageSize
	supportModelList, err := s.JDs.List(ctx, sm, options)
	if err != nil {
		return nil, err
	}

	recommendModel, _ := RecommendModels()
	for _, supportModel := range supportModelList {
		IsRecommend := false
		smInfo := supportModel.(*types.SupportModel)
		if smInfo.ApiFlavor == types.FlavorOllama {
			if recommendModel == nil {
				IsRecommend = false
			}
			rmServiceModelInfo := recommendModel[smInfo.ServiceName]
			if rmServiceModelInfo != nil {
				for _, rm := range rmServiceModelInfo {
					if rm.Name == smInfo.Name {
						IsRecommend = true
						break
					}
				}
			}
		}

		providerName := fmt.Sprintf("%s_%s_%s", smInfo.ServiceSource, types.FlavorOllama, smInfo.ServiceName)
		modelQuery := new(types.Model)
		modelQuery.ModelName = smInfo.Name
		modelQuery.ProviderName = providerName
		canSelect := true
		err := s.JDs.Get(context.Background(), modelQuery)
		if err != nil {
			canSelect = false
		}
		if modelQuery.Status != "downloaded" {
			canSelect = false
		}

		if canSelect {
			smInfo.CreatedAt = modelQuery.CreatedAt
		}

		providerServiceDefaultInfo := schedule.GetProviderServiceDefaultInfo(smInfo.Flavor, smInfo.ServiceName)
		authFields := []string{""}
		if providerServiceDefaultInfo.AuthType == types.AuthTypeToken {
			authFields = []string{"app_key", "access_key_id", "access_key_secret"}
		} else if providerServiceDefaultInfo.AuthType == types.AuthTypeApiKey {
			authFields = []string{"api_key"}
		} else if providerServiceDefaultInfo.AuthType == types.AuthTypeSign {
			authFields = []string{"secret_id", "secret_key"}
		}
		modelData := dto.RecommendModelData{
			Id:              smInfo.Id,
			Name:            smInfo.Name,
			Avatar:          smInfo.Avatar,
			Desc:            smInfo.Description,
			Service:         smInfo.ServiceName,
			ApiFlavor:       smInfo.ApiFlavor,
			Flavor:          smInfo.Flavor,
			AuthType:        providerServiceDefaultInfo.AuthType,
			AuthFields:      authFields,
			AuthApplyUrl:    providerServiceDefaultInfo.AuthApplyUrl,
			ServiceProvider: fmt.Sprintf("%s_%s_%s", smInfo.ServiceSource, types.FlavorOllama, smInfo.ServiceName),
			CanSelect:       canSelect,
			IsRecommended:   IsRecommend,
			Source:          smInfo.ServiceSource,
			InputLength:     smInfo.InputLength,
			OutputLength:    smInfo.OutputLength,
			Class:           smInfo.Class,
			Size:            smInfo.Size,
			OllamaId:        smInfo.OllamaId,
			Think:           smInfo.Think,
			ThinkSwitch:     smInfo.ThinkSwitch,
			Tools:           smInfo.Tools,
			Context:         smInfo.Context,
			CreatedAt:       smInfo.CreatedAt,
		}
		resultList = append(resultList, modelData)

	}
	resData.Total = len(resultList)
	resData.TotalPage = len(resultList) / pageSize
	if resData.TotalPage == 0 {
		resData.TotalPage = 1
	}

	resData.Data = resultList

	return &dto.GetSupportModelResponse{
		*bcode.ModelCode,
		resData,
	}, nil
}
