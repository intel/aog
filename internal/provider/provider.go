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
	"strconv"
	"sync"
	"time"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	sdkprovider "github.com/intel/aog/plugin-sdk/provider"
)

// ModelServiceProvider is an alias to the unified interface in plugin-sdk
// Both built-in engines and plugins implement this interface
type ModelServiceProvider = sdkprovider.ModelServiceProvider

// EngineManager defines unified engine management operations
type EngineManager interface {
	InitializeEngines(enabledEngines []string) error
	RegisterEngine(name string, provider ModelServiceProvider) error
	StartAllEngines(mode string) error
	StopAllEngines() error
	StartKeepAlive()
	StopKeepAlive()
	GetEngineStatus() map[string]string
}

// Global provider factory
var (
	globalProviderFactory ProviderFactory
	factoryOnce           sync.Once
)

// InitProviderFactory 初始化全局 Provider 工厂（只能调用一次）
func InitProviderFactory(factory ProviderFactory) {
	factoryOnce.Do(func() {
		globalProviderFactory = factory
		logger.EngineLogger.Info("Provider factory initialized")
	})
}

// GetModelEngine 根据引擎名称获取 model service provider
// 不再提供默认兜底，未找到引擎将返回错误
func GetModelEngine(engineName string) (ModelServiceProvider, error) {
	if globalProviderFactory == nil {
		return nil, fmt.Errorf("provider factory not initialized")
	}
	return globalProviderFactory.GetProvider(engineName)
}

// EnsureEngineReady ensures the engine is in ready state
func EnsureEngineReady(engineName string) error {
	ctx := context.Background()

	engineProvider, err := GetModelEngine(engineName)
	if err != nil {
		logger.EngineLogger.Error("Failed to get engine", "engine", engineName, "error", err)
		return fmt.Errorf("failed to get engine %s: %w", engineName, err)
	}

	// 1. Check if engine is installed
	logger.EngineLogger.Info("Checking model engine " + engineName + " installation...")

	isInstalled, err := engineProvider.CheckEngine()
	if err != nil {
		logger.EngineLogger.Error("Check engine "+engineName+" failed", "error", err)
		return fmt.Errorf("check engine failed: %w", err)
	}

	if !isInstalled {
		logger.EngineLogger.Info("Model engine " + engineName + " not exist, start download...")
		err := engineProvider.InstallEngine(ctx)
		if err != nil {
			logger.EngineLogger.Error("Install model "+engineName+" engine failed", "error", err)
			return bcode.ErrAIGCServiceInstallEngine
		}
		logger.EngineLogger.Info("Model engine " + engineName + " download completed...")
	}

	// 2. Initialize environment
	logger.EngineLogger.Info("Setting env...")
	err = engineProvider.InitEnv()
	if err != nil {
		logger.EngineLogger.Error("Setting env error", "error", err)
		return bcode.ErrAIGCServiceInitEnv
	}

	// 3. Check engine health status
	err = engineProvider.HealthCheck(ctx)
	if err != nil {
		// If health check fails, try to start the engine
		logger.EngineLogger.Info("Engine " + engineName + " not running, starting...")
		err = engineProvider.StartEngine(types.EngineStartModeDaemon)
		if err != nil {
			logger.EngineLogger.Error("Start engine "+engineName+" error", "error", err)
			return bcode.ErrAIGCServiceStartEngine
		}

		// Wait for engine to start (up to 60 seconds)
		logger.EngineLogger.Info("Waiting " + engineName + " start 60s...")
		for i := 60; i > 0; i-- {
			time.Sleep(time.Second * 1)
			err = engineProvider.HealthCheck(ctx)
			if err == nil {
				break
			}
			logger.EngineLogger.Info("Waiting "+engineName+" start ...", strconv.Itoa(i), "s")
		}
	}

	// 4. Final health check
	err = engineProvider.HealthCheck(ctx)
	if err != nil {
		logger.EngineLogger.Error(engineName + " failed start...")
		return bcode.ErrAIGCServiceStartEngine
	}

	logger.EngineLogger.Info("Engine " + engineName + " is ready")
	return nil
}

// Global engine manager instance
var (
	globalEngineManager EngineManager
	engineManagerOnce   sync.Once
)

// GetEngineManager returns the global engine manager instance (singleton)
func GetEngineManager() EngineManager {
	engineManagerOnce.Do(func() {
		globalEngineManager = newEngineManager()
	})
	return globalEngineManager
}

// InitEngines 初始化引擎（便捷方法）
func InitEngines(enabledEngines []string) error {
	manager := GetEngineManager().(*engineManager)
	return manager.InitializeEngines(enabledEngines)
}
