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

package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/ollama-plugin/internal/utils"
)

// =============== EngineLifecycleManager Implementation ===============

// processManager global process manager instance
var processManager *utils.ProcessManager

// StartEngine starts ollama engine (using simplified process manager)
func (p *OllamaProvider) StartEngine(mode string) error {
	log.Printf("[ollama-plugin] [INFO] Starting engine with mode: %s", mode)
	config, err := p.getConfig()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get config: %v", err)
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Check if ollama is installed
	log.Printf("[ollama-plugin] [DEBUG] Checking if engine is installed...")
	installed, err := p.CheckEngine()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Engine check failed: %v", err)
		return fmt.Errorf("failed to check engine: %w", err)
	}
	if !installed {
		log.Printf("[ollama-plugin] [ERROR] Engine not installed at: %s", config.ExecPath)
		return fmt.Errorf("ollama not installed, please run InstallEngine first")
	}

	// Initialize process manager
	if processManager == nil {
		log.Printf("[ollama-plugin] [DEBUG] Initializing process manager...")
		log.Printf("[ollama-plugin] [DEBUG] ExecPath: %s", config.ExecPath)
		log.Printf("[ollama-plugin] [DEBUG] Host: %s", config.Host)
		log.Printf("[ollama-plugin] [DEBUG] ModelsDir: %s", config.ModelsDir)
		processManager = utils.NewProcessManager(
			config.ExecPath,
			config.Host,
			config.ModelsDir,
		)
	}

	// Start process
	log.Printf("[ollama-plugin] [INFO] Starting ollama process...")
	if err := processManager.Start(mode); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to start ollama: %v", err)
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] ✅ Engine started successfully")
	return nil
}

// StopEngine stops ollama engine
func (p *OllamaProvider) StopEngine() error {
	log.Printf("[ollama-plugin] [INFO] Stopping engine...")

	// First unload running models (ported from built-in engine)
	log.Printf("[ollama-plugin] [DEBUG] Unloading running models...")
	if err := p.unloadRunningModels(); err != nil {
		// Continue stopping engine even if unload fails
		log.Printf("[ollama-plugin] [WARN] Failed to unload models (continuing): %v", err)
	}

	// Stop process (will find and stop all ollama serve processes)
	if processManager != nil {
		log.Printf("[ollama-plugin] [INFO] Stopping ollama process (searching for all 'ollama serve' processes)...")
		if err := processManager.Stop(); err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to stop ollama: %v", err)
			return fmt.Errorf("failed to stop ollama: %w", err)
		}
	} else {
		log.Printf("[ollama-plugin] [DEBUG] No process manager available, trying to stop by process name...")
		// Even if processManager is nil, try to stop process
		pm := utils.NewProcessManager("", "", "")
		if err := pm.Stop(); err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to stop ollama by process name: %v", err)
			return fmt.Errorf("failed to stop ollama: %w", err)
		}
	}

	log.Printf("[ollama-plugin] [INFO] ✅ Engine stopped successfully")
	return nil
}

// HealthCheck performs health check
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	log.Printf("[ollama-plugin] [DEBUG] Performing health check...")
	if processManager != nil {
		err := processManager.HealthCheck()
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Health check failed: %v", err)
		} else {
			log.Printf("[ollama-plugin] [DEBUG] Health check passed")
		}
		return err
	}

	// If no process manager, directly check HTTP
	config, err := p.getConfig()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get config for health check: %v", err)
		return err
	}

	url := fmt.Sprintf("http://%s", config.Host)
	log.Printf("[ollama-plugin] [DEBUG] Health check URL: %s", url)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to create health check request: %v", err)
		return err
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Health check request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("[ollama-plugin] [DEBUG] Health check passed (status: %d)", resp.StatusCode)
		return nil
	}

	log.Printf("[ollama-plugin] [ERROR] Health check failed with status: %d", resp.StatusCode)
	return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
}

// GetConfig gets engine configuration (implements EngineLifecycleManager interface)
func (p *OllamaProvider) GetConfig(ctx context.Context) (*types.EngineRecommendConfig, error) {
	log.Printf("[ollama-plugin] [DEBUG] Getting engine config...")
	config, err := p.getConfig()
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get config: %v", err)
		return nil, err
	}

	// Extract filename from ExecPath
	execFile := filepath.Base(config.ExecPath)

	engineConfig := &types.EngineRecommendConfig{
		Host:           config.Host,
		Scheme:         config.Scheme,
		RecommendModel: config.DefaultModel,
		EnginePath:     config.EngineDir,
		ExecPath:       config.ExecPath,
		ExecFile:       execFile,
		DeviceType:     config.DeviceType,
	}

	log.Printf("[ollama-plugin] [DEBUG] Engine config: Host=%s, ExecPath=%s, DeviceType=%s",
		config.Host, config.ExecPath, config.DeviceType)
	return engineConfig, nil
}

// unloadRunningModels unloads running models (ported from built-in engine)
func (p *OllamaProvider) unloadRunningModels() error {
	log.Printf("[ollama-plugin] [DEBUG] Getting running models for unload...")
	ctx := context.Background()

	// Get running models
	runningModels, err := p.GetRunningModels(ctx)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to get running models: %v", err)
		return err
	}

	// Unload each model
	var modelNames []string
	for _, model := range runningModels.Models {
		modelNames = append(modelNames, model.Name)
	}

	if len(modelNames) > 0 {
		log.Printf("[ollama-plugin] [INFO] Unloading %d running models: %v", len(modelNames), modelNames)
		// Call UnloadModel to unload all models
		unloadReq := &types.UnloadModelRequest{
			Models: modelNames,
		}
		if err := p.UnloadModel(ctx, unloadReq); err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to unload models: %v", err)
			return fmt.Errorf("failed to unload models: %w", err)
		}
		log.Printf("[ollama-plugin] [INFO] Successfully unloaded %d models", len(modelNames))
	} else {
		log.Printf("[ollama-plugin] [DEBUG] No running models to unload")
	}

	return nil
}
