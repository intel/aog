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

package schedule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/manager"
	"github.com/intel/aog/internal/plugin/registry"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

const (
	// ModelPreparationTimeout 模型准备超时时间
	ModelPreparationTimeout = 5 * time.Minute
)

type ServiceTaskEventType int

const (
	ServiceTaskEnqueue ServiceTaskEventType = iota
	ServiceTaskFailed
	ServiceTaskDone
)

type ServiceTaskEvent struct {
	Type  ServiceTaskEventType
	Task  *ServiceTask
	Error error // only for ServiceTaskFailed
}

type ServiceScheduler interface {
	// Enqueue itself is a non-blocking API. It returns the task ID and
	// a chan to return results as the Restful service
	Enqueue(*types.ServiceRequest) (uint64, chan *types.ServiceResult)
	Start()
	TaskComplete(*ServiceTask, error)
}

type BasicServiceScheduler struct {
	curID       uint64
	WaitingList *utils.SafeList
	RunningList *utils.SafeList
	ChEvent     chan *ServiceTaskEvent
}

func NewBasicServiceScheduler() *BasicServiceScheduler {
	return &BasicServiceScheduler{
		WaitingList: utils.NewSafeList(),
		RunningList: utils.NewSafeList(),
		ChEvent:     make(chan *ServiceTaskEvent, 600),
	}
}

func (ss *BasicServiceScheduler) Enqueue(req *types.ServiceRequest) (uint64, chan *types.ServiceResult) {
	ch := make(chan *types.ServiceResult, 600)
	ss.curID += 1
	// we don't close ch here. It should be closed when the task is done
	task := &ServiceTask{Request: req, Ch: ch}
	task.Schedule.Id = ss.curID
	ss.ChEvent <- &ServiceTaskEvent{Type: ServiceTaskEnqueue, Task: task}
	return task.Schedule.Id, ch
}

func (ss *BasicServiceScheduler) TaskComplete(task *ServiceTask, err error) {
	if task.Schedule.ListMark == nil {
		panic("[Schedule] See a task without a list mark")
	}
	if err == nil {
		ss.ChEvent <- &ServiceTaskEvent{Type: ServiceTaskDone, Task: task}
	} else {
		ss.ChEvent <- &ServiceTaskEvent{Type: ServiceTaskFailed, Task: task, Error: err}
	}
}

func (ss *BasicServiceScheduler) Start() {
	logger.LogicLogger.Info("[Init] Start basic service scheduler ...")
	go func() {
		for taskEvent := range ss.ChEvent {
			task := taskEvent.Task
			switch taskEvent.Type {
			case ServiceTaskEnqueue:
				ss.onTaskEnqueue(task)
			case ServiceTaskDone:
				ss.onTaskDone(task)
			case ServiceTaskFailed:
				ss.onTaskFailed(task, taskEvent.Error)
			}
			ss.schedule()
		}
	}()
}

func (ss *BasicServiceScheduler) onTaskEnqueue(task *ServiceTask) {
	logger.LogicLogger.Info("[Schedule] Enqueue", "taskID", task.Schedule.Id, "service", task.Request.Service)
	ss.addToList(task, "waiting")
	task.Schedule.TimeEnqueue = time.Now()
}

func (ss *BasicServiceScheduler) onTaskDone(task *ServiceTask) {
	logger.LogicLogger.Info("[Schedule] Task Done", "taskID", task.Schedule.Id, "since queued", time.Since(task.Schedule.TimeEnqueue),
		"since run", time.Since(task.Schedule.TimeRun))
	task.Schedule.TimeComplete = time.Now()
	close(task.Ch)
	ss.removeFromList(task)
}

func (ss *BasicServiceScheduler) onTaskFailed(task *ServiceTask, err error) {
	logger.LogicLogger.Error("[Service] Task Failed", "taskID", task.Schedule.Id, "error", err.Error(), "since queued",
		time.Since(task.Schedule.TimeEnqueue), "since run", time.Since(task.Schedule.TimeRun))
	task.Error = err
	task.Schedule.TimeComplete = time.Now()
	close(task.Ch)
	ss.removeFromList(task)
}

func (ss *BasicServiceScheduler) addToList(task *ServiceTask, list string) {
	switch list {
	case "waiting":
		mark := ss.WaitingList.PushBack(task)
		task.Schedule.ListMark = mark
	case "running":
		mark := ss.RunningList.PushBack(task)
		task.Schedule.ListMark = mark
	default:
		panic("[Schedule] Invalid list name: " + list)
	}
}

func (ss *BasicServiceScheduler) removeFromList(task *ServiceTask) {
	if task.Schedule.IsRunning {
		ss.RunningList.Remove(task.Schedule.ListMark)
	} else {
		ss.WaitingList.Remove(task.Schedule.ListMark)
	}
}

// returns priority, smaller more preferred to pick
// 1 - if exactly match
// 2 - if ask is prefix of got, e.g. asks llama3.1, got llama3.1-int8
// 3 - if got is prefix of ask, e.g. asks llama3.1, got llama3
// 4 - if ask is part of got but not prefix, e.g. asks llama3.1, got my-llama3.1-int8
// 5 - if got is part of ask but not prefix, e.g. asks llama3.1, got lama
// 6 - otherwise
func modelPriority(ask, got string) int {
	if ask == got {
		return 1
	}
	if strings.HasPrefix(got, ask) {
		return 2
	}
	if strings.HasPrefix(ask, got) {
		return 3
	}
	if strings.Contains(got, ask) {
		return 4
	}
	if strings.Contains(ask, got) {
		return 5
	}
	return 6
}

// Decide the running details - local or remote, which model, which xpu etc.
// It will fill in the task.Target field if need to run now
// So if task.Target is still nil, it means the task is not ready to run
func (ss *BasicServiceScheduler) dispatch(task *ServiceTask) (*types.ServiceTarget, error) {
	// Location Selection
	// ================
	// TODO: so far we all dispatch to local, unless force

	location := types.ServiceSourceLocal
	model := task.Request.Model
	if task.Request.HybridPolicy == "always_local" {
		location = types.ServiceSourceLocal
	} else if task.Request.HybridPolicy == "always_remote" {
		location = types.ServiceSourceRemote
	} else if task.Request.HybridPolicy == "default" {
		if model == "" {
			//gpuUtilization, err := utils.GetGpuInfo()
			//if err != nil {
			//cpuTotalPercent, _ := cpu.Percent(15*time.Second, false)
			//if cpuTotalPercent[0] > 80.0 {
			//	location = types.ServiceSourceRemote
			//}
			//}
			//if gpuUtilization >= 80.0 {
			//	location = types.ServiceSourceRemote
			//}
		}
	}
	ds := datastore.GetDefaultDatastore()
	service := &types.Service{
		Name: task.Request.Service,
	}

	err := ds.Get(context.Background(), service)
	if err != nil {
		logger.LogicLogger.Error("[Schedule] Failed to get service", "error", err, "service", task.Request.Service)
		return nil, bcode.ErrServiceRecordNotFound
	}

	m := &types.Model{}

	// Provider Selection
	// ================
	var providerName string
	if model != "" {
		m.ModelName = model
		m.ServiceName = task.Request.Service
		err = ds.Get(context.Background(), m)
		if err != nil {
			logger.LogicLogger.Error("[Schedule] Failed to get model", "error", err, "model", model)
			return nil, bcode.ErrModelRecordNotFound
		}
		if m.Status != "downloaded" {
			logger.LogicLogger.Error("[Schedule] model is available", "error", err, "model", model)
			return nil, bcode.ErrModelRecordNotFound
		}

	} else {
		sortOption := []datastore.SortOption{
			{Key: "updated_at", Order: -1},
		}
		ms, err := ds.List(context.Background(), m, &datastore.ListOptions{
			FilterOptions: datastore.FilterOptions{
				Queries: []datastore.FuzzyQueryOption{
					{Key: "service_name", Query: task.Request.Service},
					{Key: "status", Query: "downloaded"},
				},
			},
			SortBy: sortOption,
		})
		if err != nil {
			logger.LogicLogger.Error("[Schedule] model not found", "error", err, "model", task.Request.Model)
			return nil, bcode.ErrModelRecordNotFound
		}
		if len(ms) == 0 {
			logger.LogicLogger.Error("[Schedule] no downloaded models found",
				"service", task.Request.Service)
			return nil, bcode.ErrModelRecordNotFound
		}
		m = ms[0].(*types.Model)
	}
	providerName = m.ProviderName

	sp := &types.ServiceProvider{
		ProviderName: providerName,
	}
	err = ds.Get(context.Background(), sp)
	if err != nil {
		logger.LogicLogger.Error("[Schedule] service provider not found",
			"location", location,
			"service", task.Request.Service,
			"error", err)
		return nil, bcode.ErrProviderNotExist
	}

	location = sp.ServiceSource
	providerProperties := &types.ServiceProviderProperties{}
	// 处理空字符串情况（旧数据可能为空）
	propertiesJSON := sp.Properties
	if propertiesJSON == "" {
		propertiesJSON = "{}"
	}
	err = json.Unmarshal([]byte(propertiesJSON), providerProperties)
	if err != nil {
		logger.LogicLogger.Error("[Schedule] failed to unmarshal service provider properties", "error", err, "properties", sp.Properties)
		return nil, bcode.ErrUnmarshalProviderProperties
	}
	// Non-query model services do not require model validation
	if task.Request.Service != types.ServiceModels {
		if model == "" {
			switch location {
			case types.ServiceSourceLocal:
				m := &types.Model{
					ProviderName: sp.ProviderName,
				}
				// 先查找 is_default=true 且 downloaded 的模型
				ms, err := ds.List(context.Background(), m, &datastore.ListOptions{
					FilterOptions: datastore.FilterOptions{
						Queries: []datastore.FuzzyQueryOption{
							{Key: "status", Query: "downloaded"},
							{Key: "is_default", Query: "true"},
						},
					},
				})

				if err == nil && len(ms) > 0 {
					model = ms[0].(*types.Model).ModelName
				} else {
					// 没有default，再查找第一个downloaded模型
					sortOption := []datastore.SortOption{
						{Key: "updated_at", Order: -1},
					}
					ms, err := ds.List(context.Background(), m, &datastore.ListOptions{
						FilterOptions: datastore.FilterOptions{
							Queries: []datastore.FuzzyQueryOption{
								{Key: "status", Query: "downloaded"},
							},
						},
						SortBy: sortOption,
					})
					if err != nil {
						return nil, fmt.Errorf("model not found for %s of Service %s", location, task.Request.Service)
					}
					if len(ms) == 0 {
						return nil, fmt.Errorf("model not found for %s of Service %s", location, task.Request.Service)
					}
					model = ms[0].(*types.Model).ModelName
				}
			case types.ServiceSourceRemote:
				defaultInfo := GetProviderServiceDefaultInfo(sp.Flavor, task.Request.Service)
				model = defaultInfo.DefaultModel
			}
		}
	}

	// Model Selection
	// ================
	// pick the smallest priority number, which means the most preferred
	// if more than one candidate for the same priority, pick the 1st one
	// TODO(Strategies to be discussed later)

	// Stream Mode Selection
	// ================
	stream := task.Request.AskStreamMode
	// assume it supports stream mode if not specified supported_response_mode
	if stream && len(providerProperties.SupportedResponseMode) > 0 {
		stream = false
		for _, mode := range providerProperties.SupportedResponseMode {
			if mode == "stream" {
				stream = true
				break
			}
		}
		if !stream {
			logger.LogicLogger.Warn("[Schedule] Asks for stream mode but it is not supported by the service provider",
				"id_service_provider", sp.ProviderName, "supported_response_mode", providerProperties.SupportedResponseMode)
		}
	}

	// Stream Mode Selection
	// ================
	// TODO: XPU selection

	// 从配置中获取服务协议类型（支持内置和插件）
	protocol, exposeProtocol, err := ss.getServiceProtocols(sp, task.Request.Service)
	if err != nil {
		logger.LogicLogger.Error("[Schedule] Failed to get service protocols",
			"provider", sp.ProviderName,
			"service", task.Request.Service,
			"error", err)
		return nil, bcode.WrapError(bcode.ErrServer, err)
	}

	// 模型内存管理：确保本地模型已加载
	if location == types.ServiceSourceLocal && model != "" {
		// 获取模型引擎实例
		modelEngine, err := provider.GetModelEngine(sp.Flavor)
		if err != nil {
			logger.LogicLogger.Error("[Schedule] Failed to get model engine",
				"flavor", sp.Flavor, "error", err)
			return nil, bcode.ErrProviderNotExist
		}

		mmm := manager.GetModelManager()
		ctx := context.Background()

		// 请求分流：判断是否需要进入排队机制
		if manager.NeedsQueuing(location, task.Request.Service) {
			// 本地非embed请求：进入排队机制
			logger.LogicLogger.Debug("[Schedule] Enqueueing local non-embed model request",
				"model", model, "service", task.Request.Service, "provider", sp.ProviderName)

			readyChan, errorChan, err := mmm.EnqueueLocalModelRequest(ctx, model, modelEngine, sp.ProviderName, sp.Flavor, task.Schedule.Id)
			if err != nil {
				logger.LogicLogger.Error("[Schedule] Failed to enqueue local model request",
					"model", model, "provider", sp.ProviderName, "error", err)
				return nil, fmt.Errorf("failed to enqueue model request %s: %w", model, err)
			}

			logger.LogicLogger.Info("[Schedule] Local model request enqueued, waiting for model preparation",
				"model", model, "provider", sp.ProviderName, "taskID", task.Schedule.Id)

			// 等待队列处理完成（模型切换和准备）
			select {
			case <-readyChan:
				// 检查是否有错误
				select {
				case queueErr := <-errorChan:
					logger.LogicLogger.Error("[Schedule] Model preparation failed",
						"model", model, "taskID", task.Schedule.Id, "error", queueErr)
					return nil, fmt.Errorf("model preparation failed for %s: %w", model, queueErr)
				default:
					logger.LogicLogger.Info("[Schedule] Model preparation completed, ready to execute task",
						"model", model, "taskID", task.Schedule.Id)
				}
			case <-ctx.Done():
				logger.LogicLogger.Warn("[Schedule] Context cancelled while waiting for model preparation",
					"model", model, "taskID", task.Schedule.Id, "error", ctx.Err())
				return nil, ctx.Err()
			case <-time.After(ModelPreparationTimeout):
				logger.LogicLogger.Error("[Schedule] Timeout waiting for model preparation",
					"model", model, "taskID", task.Schedule.Id, "timeout", ModelPreparationTimeout)
				return nil, fmt.Errorf("timeout waiting for model preparation: %s (timeout: %v)", model, ModelPreparationTimeout)
			}
		} else if location == types.ServiceSourceLocal && task.Request.Service == types.ServiceEmbed {
			// Local embed request: ensure model is loaded (skip queuing to stay lightweight)
			logger.LogicLogger.Debug("[Schedule] Ensuring embed model is loaded",
				"model", model, "provider", sp.ProviderName)

			if err := ensureModelLoaded(ctx, model, modelEngine, sp.ProviderName); err != nil {
				logger.LogicLogger.Error("[Schedule] Failed to ensure embed model loaded",
					"model", model, "provider", sp.ProviderName, "error", err)
				return nil, fmt.Errorf("failed to load embed model %s: %w", model, err)
			}

			logger.LogicLogger.Debug("[Schedule] Embed model is ready",
				"model", model, "provider", sp.ProviderName)
		}
		// Remote request: execute directly without model management
	}

	return &types.ServiceTarget{
		Location:        location,
		Stream:          stream,
		Model:           model,
		ToFavor:         sp.Flavor,
		Protocol:        protocol, // 设置Protocol字段
		ExposeProtocol:  exposeProtocol,
		ServiceProvider: sp,
	}, nil
}

// ensureModelLoaded ensures the model is loaded into memory
// Used for services that don't need queuing (e.g. embed), loads model directly without going through queue
// LoadModel internally checks if model is already loaded and returns immediately if so
func ensureModelLoaded(ctx context.Context, modelName string, providerInstance provider.ModelServiceProvider, providerName string) error {
	logger.LogicLogger.Debug("[Schedule] Ensuring model is loaded",
		"model", modelName, "provider", providerName)

	loadReq := &sdktypes.LoadRequest{
		Model: modelName,
	}

	// LoadModel is idempotent: returns immediately if already loaded, otherwise loads the model
	if err := providerInstance.LoadModel(ctx, loadReq); err != nil {
		logger.LogicLogger.Error("[Schedule] Failed to load model",
			"model", modelName, "provider", providerName, "error", err)
		return fmt.Errorf("failed to load model: %w", err)
	}

	logger.LogicLogger.Debug("[Schedule] Model is ready",
		"model", modelName, "provider", providerName)

	return nil
}

// getServiceProtocols 获取服务的协议信息（支持内置和插件）
func (ss *BasicServiceScheduler) getServiceProtocols(sp *types.ServiceProvider, serviceName string) (protocol, exposeProtocol string, err error) {
	// 1. 检查是否为插件
	if sp.Scope == "plugin" {
		// 从插件 manifest 获取服务信息
		pluginRegistry := registry.GetGlobalPluginRegistry()
		if pluginRegistry == nil {
			return "", "", fmt.Errorf("plugin registry not initialized")
		}

		manifest, err := pluginRegistry.GetPluginManifest(sp.Flavor)
		if err != nil {
			return "", "", fmt.Errorf("plugin manifest not found for provider: %s", sp.Flavor)
		}

		// 在插件的服务定义中查找
		serviceDef, err := manifest.GetServiceByName(serviceName)
		if err != nil {
			return "", "", fmt.Errorf("service %s not found in plugin %s: %w", serviceName, sp.Flavor, err)
		}

		return serviceDef.Protocol, serviceDef.ExposeProtocol, nil
	}

	// 2. 内置 provider：从 flavor 定义读取
	flavorDef := GetFlavorDef(sp.Flavor)
	if serviceDef, exists := flavorDef.Services[serviceName]; exists {
		return serviceDef.Protocol, serviceDef.ExposeProtocol, nil
	}

	return "", "", fmt.Errorf("service %s not supported by provider %s", serviceName, sp.Flavor)
}

// this is invoked by schedule goroutine
func (ss *BasicServiceScheduler) schedule() {
	// TODO: currently, we run all of the
	for e := ss.WaitingList.Front(); e != nil; e = e.Next() {
		task := e.Value.(*ServiceTask)
		target, err := ss.dispatch(task)
		if err != nil {
			task.Ch <- &types.ServiceResult{Type: types.ServiceResultFailed, TaskId: task.Schedule.Id, Error: err}
			ss.onTaskFailed(task, err)
			continue
		}
		task.Target = target
		ss.removeFromList(task)
		ss.addToList(task, "running")
		task.Schedule.IsRunning = true
		task.Schedule.TimeRun = time.Now()
		logger.LogicLogger.Info("[Schedule] Start to run the task", "taskID", task.Schedule.Id, "service", task.Request.Service,
			"location", task.Target.Location, "service_provider", task.Target.ServiceProvider)
		// REALLY run the task
		go func() {
			err := task.Run()
			// need to send back error to the client
			if err != nil {
				task.Ch <- &types.ServiceResult{Type: types.ServiceResultFailed, TaskId: task.Schedule.Id, Error: err}
			}
			ss.TaskComplete(task, err)
		}()
	}
}

var scheduler ServiceScheduler

func StartScheduler(s string) {
	if scheduler != nil {
		panic("Default scheduler is already set")
	}
	switch s {
	case "basic":
		scheduler = NewBasicServiceScheduler()
		scheduler.Start()
	default:
		panic(fmt.Sprintf("Invalid scheduler type: %s", s))
	}
}

func GetScheduler() ServiceScheduler {
	if scheduler == nil {
		panic("Scheduler is not started yet")
	}
	return scheduler
}

func InvokeService(fromFlavor string, service string, request *http.Request) (uint64, chan *types.ServiceResult, error) {
	logger.LogicLogger.Info("[Service] Invoking Service", "fromFlavor", fromFlavor, "service", service)

	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		logger.LogicLogger.Error("[Service] Unsupported request method", "method", request.Method)
		return 0, nil, bcode.ErrUnSupportRequestMethod
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to read request body", "error", err)
		return 0, nil, bcode.ErrReadRequestBody
	}

	// 重新设置请求体，以便后续处理可能需要再次读取
	request.Body = io.NopCloser(bytes.NewReader(body))

	wsConnID := request.Header.Get("X-WebSocket-ConnID")
	isWebSocket := wsConnID != ""

	if request.Method == http.MethodGet && !isWebSocket {
		queryParams := request.URL.Query()
		queryParamsJSON, err := json.Marshal(queryParams)
		if err != nil {
			logger.LogicLogger.Error("[Service] Failed to unmarshal GET request", "error", err, "body", string(body))
			return 0, nil, bcode.ErrUnmarshalRequestBody
		}
		logger.LogicLogger.Debug("[Service] GET Request Query Params", "params", string(queryParamsJSON))

		body = queryParamsJSON
	}

	hybridPolicy := "default"
	if service != "" {
		ds := datastore.GetDefaultDatastore()
		sp := &types.Service{
			Name:   service,
			Status: 1,
		}
		err = ds.Get(context.Background(), sp)
		if err != nil {
			logger.LogicLogger.Error("[Schedule] Failed to get service", "error", err, "service", service)
		}
		hybridPolicy = sp.HybridPolicy
	}

	serviceRequest := types.ServiceRequest{
		FromFlavor:      fromFlavor,
		Service:         service,
		Priority:        0,
		HTTP:            types.HTTPContent{Body: body, Header: request.Header},
		OriginalRequest: request,
		HybridPolicy:    hybridPolicy,
	}

	// 根据Content-Type决定如何处理请求体
	contentType := request.Header.Get("Content-Type")
	isBinary := strings.Contains(contentType, "application/octet-stream") ||
		strings.Contains(contentType, "audio/") ||
		strings.Contains(contentType, "video/") ||
		strings.Contains(contentType, "image/")

	// 处理WebSocket相关信息
	if isWebSocket {
		// 如果是WebSocket请求，添加相关标记
		serviceRequest.WebSocketConnID = wsConnID

		// 记录WebSocket请求特有的日志
		logger.LogicLogger.Debug("[Service] Processing WebSocket request",
			"wsConnID", wsConnID,
			"contentType", contentType,
			"isBinary", isBinary,
			"bodySize", len(body))

		// 如果是二进制数据，使用更适合的日志格式
		if isBinary {
			logger.LogicLogger.Debug("[Service] Binary WebSocket data received",
				"wsConnID", wsConnID,
				"bodySize", len(body),
				"firstBytes", fmt.Sprintf("%x", body[:utils.Min(20, len(body))]))
		}
	}

	// 只有当不是二进制数据时，才尝试解析JSON
	if !isBinary {
		err = json.Unmarshal(body, &serviceRequest)
		if err != nil {
			// 对于WebSocket文本消息，可能是特定指令而不是完整的JSON结构
			if isWebSocket {
				logger.LogicLogger.Debug("[Service] WebSocket message is not valid JSON, treating as raw message",
					"wsConnID", wsConnID,
					"error", err)
			} else {
				logger.LogicLogger.Error("[Service] Failed to unmarshal POST request", "error", err, "body", string(body))
				return 0, nil, err
			}
		}
	}

	taskID, ch := GetScheduler().Enqueue(&serviceRequest)

	// 对于WebSocket请求，记录分配的taskID
	if isWebSocket {
		logger.LogicLogger.Info("[Service] WebSocket request enqueued",
			"wsConnID", wsConnID,
			"taskID", taskID)
	}

	return taskID, ch, err
}
