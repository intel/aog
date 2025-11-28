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
	"fmt"
	"sync"
	"time"

	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
)

// engineManager implements EngineManager interface
type engineManager struct {
	mu      sync.RWMutex
	engines map[string]ModelServiceProvider

	// 保活监控相关
	keepAliveEnabled bool
	keepAliveTicker  *time.Ticker
	keepAliveCtx     context.Context
	keepAliveCancel  context.CancelFunc
}

// newEngineManager creates a new engine manager instance
func newEngineManager() *engineManager {
	manager := &engineManager{
		engines: make(map[string]ModelServiceProvider),
	}
	return manager
}

// InitializeEngines 初始化引擎（配置驱动，不再自动注册）
func (m *engineManager) InitializeEngines(enabledEngines []string) error {
	if globalProviderFactory == nil {
		return fmt.Errorf("provider factory not initialized")
	}

	available := globalProviderFactory.ListAvailableProviders()

	// 如果未指定启用的引擎，则全部启用
	if len(enabledEngines) == 0 {
		enabledEngines = available
		logger.EngineLogger.Info("No engines specified, enabling all available engines")
	}

	logger.EngineLogger.Info(fmt.Sprintf("Initializing %d engines...", len(enabledEngines)))

	for _, name := range enabledEngines {
		provider, err := globalProviderFactory.GetProvider(name)
		if err != nil {
			logger.EngineLogger.Warn("Failed to get engine", "name", name, "error", err)
			continue
		}

		if err := m.RegisterEngine(name, provider); err != nil {
			logger.EngineLogger.Error("Failed to register engine", "name", name, "error", err)
			return err
		}
	}

	logger.EngineLogger.Info(fmt.Sprintf("Engine initialization completed, %d engines registered", len(m.engines)))
	return nil
}

// RegisterEngine 注册引擎（添加唯一性校验）
func (m *engineManager) RegisterEngine(name string, provider ModelServiceProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 唯一性校验
	if _, exists := m.engines[name]; exists {
		return fmt.Errorf("engine already registered: %s", name)
	}

	m.engines[name] = provider
	logger.EngineLogger.Info("Engine registered", "name", name)
	return nil
}

// StartAllEngines starts all registered engines
func (m *engineManager) StartAllEngines(mode string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.engines) == 0 {
		logger.EngineLogger.Info("No engines registered, skipping engine startup")
		return nil
	}

	logger.EngineLogger.Info(fmt.Sprintf("Starting %d engines...", len(m.engines)))

	var startErrors []error
	successCount := 0

	for engineName, engineProvider := range m.engines {
		logger.EngineLogger.Info(fmt.Sprintf("Starting engine: %s", engineName))

		if err := m.startSingleEngine(engineName, engineProvider, mode); err != nil {
			// 检查是否是引擎不存在的错误（可容忍）
			if isEngineNotFoundError(err) {
				logger.EngineLogger.Info(fmt.Sprintf("Engine %s not installed, skipping", engineName))
			} else {
				logger.EngineLogger.Error(fmt.Sprintf("Failed to start engine %s: %v", engineName, err))
				startErrors = append(startErrors, fmt.Errorf("engine %s failed to start: %v", engineName, err))
			}
		} else {
			successCount++
			logger.EngineLogger.Info(fmt.Sprintf("Engine %s started successfully", engineName))
		}
	}

	if len(startErrors) > 0 {
		if successCount > 0 {
			logger.EngineLogger.Info(fmt.Sprintf("%d engines started successfully, %d failed", successCount, len(startErrors)))
			return fmt.Errorf("some engines failed to start (%d/%d succeeded)", successCount, len(m.engines))
		}
		return fmt.Errorf("all engines failed to start: %v", startErrors)
	}

	logger.EngineLogger.Info("All engines started successfully")
	return nil
}

// startSingleEngine starts a single engine
func (m *engineManager) startSingleEngine(engineName string, engineProvider ModelServiceProvider, mode string) error {
	// 启动引擎
	if err := engineProvider.StartEngine(mode); err != nil {
		return err
	}

	// 引擎启动成功后执行升级操作
	logger.EngineLogger.Info(fmt.Sprintf("Upgrading engine %s...", engineName))
	engineProvider.UpgradeEngine(context.Background())
	logger.EngineLogger.Info(fmt.Sprintf("Engine %s upgraded successfully", engineName))

	return nil
}

// StopAllEngines stops all registered engines gracefully
func (m *engineManager) StopAllEngines() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.engines) == 0 {
		logger.EngineLogger.Info("No engines registered, skipping engine shutdown")
		return nil
	}

	logger.EngineLogger.Info(fmt.Sprintf("Stopping %d engines...", len(m.engines)))

	var stopErrors []error
	successCount := 0

	for engineName, engineProvider := range m.engines {
		logger.EngineLogger.Info(fmt.Sprintf("Stopping engine: %s", engineName))

		if err := engineProvider.StopEngine(); err != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Failed to stop engine %s: %v", engineName, err))
			stopErrors = append(stopErrors, fmt.Errorf("engine %s failed to stop: %v", engineName, err))
		} else {
			successCount++
			logger.EngineLogger.Info(fmt.Sprintf("Engine %s stopped successfully", engineName))
		}
	}

	if len(stopErrors) > 0 {
		logger.EngineLogger.Warn(fmt.Sprintf("Some engines failed to stop: %v", stopErrors))
		// 不返回错误，因为我们希望服务被认为是已停止的
	}

	logger.EngineLogger.Info("All engines stopped")
	return nil
}

// GetEngineStatus returns status of all engines
func (m *engineManager) GetEngineStatus() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]string)
	for name, engineProvider := range m.engines {
		if err := engineProvider.HealthCheck(context.Background()); err != nil {
			status[name] = "stopped"
		} else {
			status[name] = "running"
		}
	}

	return status
}

// StartKeepAlive starts the engine keep-alive monitoring
func (m *engineManager) StartKeepAlive() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keepAliveEnabled {
		logger.EngineLogger.Info("Keep-alive monitoring is already running")
		return
	}

	m.keepAliveCtx, m.keepAliveCancel = context.WithCancel(context.Background())
	m.keepAliveTicker = time.NewTicker(60 * time.Second) // 每60秒检查一次
	m.keepAliveEnabled = true

	logger.EngineLogger.Info("Starting engine keep-alive monitoring...")

	// 启动保活监控goroutine
	go m.runKeepAliveMonitor()
}

// StopKeepAlive stops the engine keep-alive monitoring
func (m *engineManager) StopKeepAlive() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.keepAliveEnabled {
		return
	}

	logger.EngineLogger.Info("Stopping engine keep-alive monitoring...")

	m.keepAliveEnabled = false

	if m.keepAliveTicker != nil {
		m.keepAliveTicker.Stop()
		m.keepAliveTicker = nil
	}

	if m.keepAliveCancel != nil {
		m.keepAliveCancel()
		m.keepAliveCancel = nil
	}

	logger.EngineLogger.Info("Engine keep-alive monitoring stopped")
}

// runKeepAliveMonitor runs the engine keep-alive monitoring loop
func (m *engineManager) runKeepAliveMonitor() {
	defer func() {
		if r := recover(); r != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Keep-alive monitor panicked: %v", r))
		}
	}()

	logger.EngineLogger.Info("Engine keep-alive monitor started")

	for {
		select {
		case <-m.keepAliveCtx.Done():
			logger.EngineLogger.Info("Keep-alive monitor context cancelled")
			return
		case <-m.keepAliveTicker.C:
			m.performKeepAliveCheck()
		}
	}
}

// performKeepAliveCheck performs the actual keep-alive check
func (m *engineManager) performKeepAliveCheck() {
	ds := datastore.GetDefaultDatastore()
	if ds == nil {
		logger.EngineLogger.Warn("Default datastore not available, skipping keep-alive check")
		return
	}

	models := &types.Model{
		ServiceSource: types.ServiceSourceLocal,
	}

	list, err := ds.List(context.Background(), models, &datastore.ListOptions{Page: 0, PageSize: 100})
	if err != nil {
		logger.EngineLogger.Error(fmt.Sprintf("Failed to list models: %v", err))
		return
	}

	if len(list) == 0 {
		// 没有本地模型，跳过检查
		return
	}

	// 获取需要监控的引擎列表
	engineList := make([]string, 0)
	for _, item := range list {
		model := item.(*types.Model)
		sp := &types.ServiceProvider{
			ProviderName: model.ProviderName,
		}

		err := ds.Get(context.Background(), sp)
		if err != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Failed to get service provider: %v", err))
			continue
		}

		if utils.Contains(engineList, sp.Flavor) {
			continue
		}

		engineList = append(engineList, sp.Flavor)
	}

	// 对每个引擎进行健康检查和保活处理
	for _, engineName := range engineList {
		m.checkAndRestartEngine(engineName)
	}
}

// checkAndRestartEngine checks engine health and restarts if necessary
func (m *engineManager) checkAndRestartEngine(engineName string) {
	m.mu.RLock()
	engineProvider := m.engines[engineName]
	m.mu.RUnlock()

	if engineProvider == nil {
		// 引擎未注册，跳过
		return
	}

	// 检查引擎是否正在使用中
	if engineProvider.GetOperateStatus != nil && engineProvider.GetOperateStatus() == 0 {
		// 引擎正在使用中，跳过保活检查
		return
	}

	// 进行健康检查
	err := engineProvider.HealthCheck(context.Background())
	if err != nil {
		logger.EngineLogger.Error(fmt.Sprintf("Engine %s health check failed: %v", engineName, err))

		// 健康检查失败，先初始化环境
		err = engineProvider.InitEnv()
		if err != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Engine %s init env failed: %v", engineName, err))
			return
		}

		// 重启引擎（使用daemon模式）
		err = engineProvider.StartEngine(types.EngineStartModeDaemon)
		if err != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Failed to restart engine %s: %v", engineName, err))
			return
		}

		// 重启成功后执行升级操作
		engineProvider.UpgradeEngine(context.Background())
	}
}

// isEngineNotFoundError checks if the error is due to engine not found
func isEngineNotFoundError(err error) bool {
	if bcodeErr, ok := err.(*bcode.Bcode); ok {
		return bcodeErr.BusinessCode == bcode.ErrEngineNotFound.BusinessCode
	}
	// Also check for string matching as fallback
	return fmt.Sprintf("%v", err) == "executable not found" ||
		fmt.Sprintf("%v", err) == "engine not found" ||
		fmt.Sprintf("%v", err) == "no such file or directory"
}
