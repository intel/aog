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
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider/engine"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
)

// ModelServiceProvider model service provider interface
// EngineLifecycleManager defines core engine lifecycle operations
type EngineLifecycleManager interface {
	StartEngine(mode string) error
	StopEngine() error
	HealthCheck() error
	GetConfig() *types.EngineRecommendConfig
}

// EngineInstaller defines engine installation and setup operations
type EngineInstaller interface {
	CheckEngine() bool
	InstallEngine(cover bool) error
	InitEnv() error
	UpgradeEngine() error
}

// ModelManager defines model management operations
type ModelManager interface {
	PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error)
	PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error)
	DeleteModel(ctx context.Context, req *types.DeleteRequest) error
	ListModels(ctx context.Context) (*types.ListResponse, error)
	LoadModel(ctx context.Context, req *types.LoadRequest) error
	UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error
	GetRunningModels(ctx context.Context) (*types.ListResponse, error)
}

// EngineInfoProvider defines informational operations
type EngineInfoProvider interface {
	GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error)
	GetOperateStatus() int
	SetOperateStatus(status int)
}

// ModelServiceProvider defines the complete interface for AI model service providers
// This is a composite interface that includes all specialized interfaces
type ModelServiceProvider interface {
	EngineLifecycleManager
	EngineInstaller
	ModelManager
	EngineInfoProvider
}

// EngineManager defines unified engine management operations
type EngineManager interface {
	// 启动所有引擎
	StartAllEngines(mode string) error
	// 停止所有引擎
	StopAllEngines() error
	// 启动引擎保活监控
	StartKeepAlive()
	// 停止引擎保活监控
	StopKeepAlive()
	// 获取引擎状态
	GetEngineStatus() map[string]string
	// 注册引擎到管理器
	RegisterEngine(name string, provider ModelServiceProvider)
}

// Engine instance cache using sync.Once for thread-safe singleton
var (
	ollamaInstance   ModelServiceProvider
	openvinoInstance ModelServiceProvider
	ollamaOnce       sync.Once
	openvinoOnce     sync.Once
)

// GetModelEngine get model service provider by engine name (singleton pattern)
func GetModelEngine(engineName string) ModelServiceProvider {
	switch engineName {
	case "ollama":
		ollamaOnce.Do(func() {
			ollamaInstance = engine.NewOllamaProvider(nil)
		})
		return ollamaInstance
	case "openvino":
		openvinoOnce.Do(func() {
			openvinoInstance = engine.NewOpenvinoProvider(nil)
		})
		return openvinoInstance
	default:
		// Default to ollama for unknown engines
		ollamaOnce.Do(func() {
			ollamaInstance = engine.NewOllamaProvider(nil)
		})
		return ollamaInstance
	}
}

// EnsureEngineReady ensures the engine is in ready state
func EnsureEngineReady(engineName string) error {
	engineProvider := GetModelEngine(engineName)

	if engineName == types.FlavorOpenvino && runtime.GOOS == "linux" {
		logger.EngineLogger.Info("OpenVINO engine is not supported on Linux currently.")
		return fmt.Errorf("openvino engine is not supported on Linux currently")
	}

	// 1. Check if engine is installed
	logger.EngineLogger.Info("Checking model engine " + engineName + " installation...")

	if !engineProvider.CheckEngine() {
		logger.EngineLogger.Info("Model engine " + engineName + " not exist, start download...")
		err := engineProvider.InstallEngine(false)
		if err != nil {
			logger.EngineLogger.Error("Install model "+engineName+" engine failed", "error", err)
			return bcode.ErrAIGCServiceInstallEngine
		}
		logger.EngineLogger.Info("Model engine " + engineName + " download completed...")
	}

	// 2. Initialize environment
	logger.EngineLogger.Info("Setting env...")
	err := engineProvider.InitEnv()
	if err != nil {
		logger.EngineLogger.Error("Setting env error", "error", err)
		return bcode.ErrAIGCServiceInitEnv
	}

	// 3. Check engine health status
	err = engineProvider.HealthCheck()
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
			err = engineProvider.HealthCheck()
			if err == nil {
				break
			}
			logger.EngineLogger.Info("Waiting "+engineName+" start ...", strconv.Itoa(i), "s")
		}
	}

	// 4. Final health check
	err = engineProvider.HealthCheck()
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
