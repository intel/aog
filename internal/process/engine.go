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

package process

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
)

// EngineProcessManager manages engine processes with specialized configuration building
type EngineProcessManager struct {
	ProcessManager
	engineName   string
	engineConfig *types.EngineRecommendConfig
}

// NewEngineProcessManager creates a new engine process manager
func NewEngineProcessManager(engineName string, config *types.EngineRecommendConfig) *EngineProcessManager {
	return &EngineProcessManager{
		ProcessManager: NewProcessManager(engineName),
		engineName:     engineName,
		engineConfig:   config,
	}
}

// StartEngine starts the engine with specified mode
func (m *EngineProcessManager) StartEngine(mode string, healthCheck func() error) error {
	logger.EngineLogger.Info(fmt.Sprintf("[Debug] Starting engine %s with mode %s", m.engineName, mode))

	// 检查引擎是否存在，如果不存在则跳过启动（与原有逻辑保持一致）
	if !m.checkEngineExists() {
		return bcode.ErrEngineNotFound.SetMessage(fmt.Sprintf("Engine %s executable not found", m.engineName))
	}

	logger.EngineLogger.Info(fmt.Sprintf("[Debug] Engine %s executable found, proceeding with startup", m.engineName))
	ctx := context.Background()

	config, err := m.buildStartConfig(mode, healthCheck)
	if err != nil {
		logger.EngineLogger.Error(fmt.Sprintf("[Debug] Failed to build config for engine %s: %v", m.engineName, err))
		return bcode.WrapError(bcode.ErrInvalidProcessConfig.SetMessage(fmt.Sprintf("Engine %s has invalid configuration", m.engineName)), err)
	}

	logger.EngineLogger.Info(fmt.Sprintf("[Debug] Starting engine %s with exec path: %s", m.engineName, config.ExecPath))
	return m.Start(ctx, config)
}

// checkEngineExists checks if the engine executable exists
func (m *EngineProcessManager) checkEngineExists() bool {
	if m.engineConfig == nil {
		return false
	}

	switch m.engineName {
	case "ollama":
		return m.checkOllamaExists()
	case "openvino":
		return m.checkOpenvinoExists()
	default:
		return false
	}
}

// checkOllamaExists checks if Ollama engine exists
func (m *EngineProcessManager) checkOllamaExists() bool {
	switch runtime.GOOS {
	case "windows":
		execFile := filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile)
		if _, err := os.Stat(execFile); err != nil {
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] Ollama not found at %s: %v", execFile, err))
			return false
		}
		return true
	case "linux":
		var execFile string
		//if m.engineConfig.DeviceType == types.GPUTypeIntelArc {
		//	execFile = filepath.Join(m.engineConfig.ExecPath, "ollama", m.engineConfig.ExecFile)
		//} else {
		execFile = filepath.Join(m.engineConfig.ExecPath, "ollama/bin", m.engineConfig.ExecFile)
		//}
		if _, err := os.Stat(execFile); err != nil {
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] Ollama not found at %s: %v", execFile, err))
			return false
		}
		return true
	case "darwin":
		// macOS: check system installation or configured path
		var execFile string
		if m.engineConfig.ExecPath != "" && m.engineConfig.ExecFile != "" {
			execFile = filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile)
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] Checking Ollama config path: %s", execFile))
			if _, err := os.Stat(execFile); err == nil {
				return true
			}
		}
		// Fallback to system installation
		execFile = "/Applications/Ollama.app/Contents/Resources/ollama"
		_, err := os.Stat(execFile)
		return err == nil
	default:
		return false
	}
}

// checkOpenvinoExists checks if OpenVINO engine exists
func (m *EngineProcessManager) checkOpenvinoExists() bool {
	switch runtime.GOOS {
	case "windows":
		execPath := filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile)
		if _, err := os.Stat(execPath); err != nil {
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] OpenVINO exec not found: %v", err))
			return false
		}
		// Also check if models directory exists
		modelDir := fmt.Sprintf("%s/models", m.engineConfig.EnginePath)
		if _, err := os.Stat(modelDir); err != nil {
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] OpenVINO models not found: %v", err))
			return false
		}
		return true
	case "linux":
		execPath := filepath.Join(m.engineConfig.ExecPath, "ovms", "bin", m.engineConfig.ExecFile)
		if _, err := os.Stat(execPath); err != nil {
			logger.EngineLogger.Info(fmt.Sprintf("[Debug] OpenVINO not found: %v", err))
			return false
		}
		return true
	default:
		return false
	}
}

// StopEngine stops the engine gracefully
func (m *EngineProcessManager) StopEngine() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return m.Stop(ctx)
}

// buildStartConfig builds start configuration for different engines
func (m *EngineProcessManager) buildStartConfig(mode string, healthCheck func() error) (*StartConfig, error) {
	switch m.engineName {
	case "ollama":
		return m.buildOllamaConfig(mode, healthCheck)
	case "openvino":
		return m.buildOpenvinoConfig(mode, healthCheck)
	default:
		return nil, fmt.Errorf("unsupported engine: %s", m.engineName)
	}
}

// buildOllamaConfig builds start configuration for Ollama engine
func (m *EngineProcessManager) buildOllamaConfig(mode string, healthCheck func() error) (*StartConfig, error) {
	if m.engineConfig == nil {
		return nil, fmt.Errorf("engine config is nil")
	}

	// Build environment variables directly using config information
	var env []string
	env = append(env, fmt.Sprintf("OLLAMA_HOST=%s", m.engineConfig.Host))
	env = append(env, fmt.Sprintf("OLLAMA_ORIGIN=%s", m.engineConfig.Origin))

	if runtime.GOOS == "linux" {
		env = append(env, "OLLAMA_MODELS=/var/lib/aog/engine/ollama/models")

		// Intel Arc specific environment variables
		//if m.engineConfig.DeviceType == types.GPUTypeIntelArc {
		//	env = append(env, "OLLAMA_NUM_GPU=999")
		//	env = append(env, "no_proxy=localhost,127.0.0.1")
		//	env = append(env, "ZES_ENABLE_SYSMAN=1")
		//	env = append(env, "OLLAMA_KEEP_ALIVE=10m")
		//	env = append(env, "SYCL_PI_LEVEL_ZERO_USE_IMMEDIATE_COMMANDLISTS=1")
		//}
	}

	switch runtime.GOOS {
	case "windows":
		return m.buildOllamaWindowsConfig(mode, healthCheck, env)
	case "linux":
		return m.buildOllamaLinuxConfig(mode, healthCheck, env)
	case "darwin":
		return m.buildOllamaMacOSConfig(mode, healthCheck, env)
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// buildOllamaWindowsConfig builds Windows-specific Ollama configuration
func (m *EngineProcessManager) buildOllamaWindowsConfig(mode string, healthCheck func() error, env []string) (*StartConfig, error) {
	// Check if Intel GPU should use ipex-llm
	// This would need to be imported or exposed from utils package
	// For now, assume it's standard ollama

	execPath := filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile)
	args := []string{"serve"}

	startMode := StartModeBackground
	if mode == types.EngineStartModeStandard {
		startMode = StartModeForeground
	}

	return &StartConfig{
		Name:        m.engineName,
		ExecPath:    execPath,
		Args:        args,
		Env:         env,
		WorkDir:     m.engineConfig.ExecPath,
		Mode:        startMode,
		Timeout:     60 * time.Second,
		HealthCheck: healthCheck,
	}, nil
}

// buildOllamaLinuxConfig builds Linux-specific Ollama configuration
func (m *EngineProcessManager) buildOllamaLinuxConfig(mode string, healthCheck func() error, env []string) (*StartConfig, error) {
	var execPath string
	var args []string
	var workDir string

	//if m.engineConfig.DeviceType == types.GPUTypeIntelArc {
	//	execPath = filepath.Join(m.engineConfig.ExecPath, "ollama", m.engineConfig.ExecFile)
	//	args = []string{"serve"}
	//	workDir = filepath.Join(m.engineConfig.ExecPath, "ollama")
	//} else {
	execPath = filepath.Join(m.engineConfig.ExecPath, "ollama/bin", m.engineConfig.ExecFile)
	args = []string{"serve"}
	workDir = m.engineConfig.ExecPath
	//}

	startMode := StartModeBackground
	if mode == types.EngineStartModeStandard {
		startMode = StartModeForeground
	}

	return &StartConfig{
		Name:        m.engineName,
		ExecPath:    execPath,
		Args:        args,
		Env:         env,
		WorkDir:     workDir,
		Mode:        startMode,
		Timeout:     60 * time.Second,
		HealthCheck: healthCheck,
	}, nil
}

// buildOllamaMacOSConfig builds macOS-specific Ollama configuration
func (m *EngineProcessManager) buildOllamaMacOSConfig(mode string, healthCheck func() error, env []string) (*StartConfig, error) {
	var execPath string

	// Prefer configured path, use system installed Ollama if not found
	if m.engineConfig.ExecPath != "" && m.engineConfig.ExecFile != "" {
		execPath = filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile)
		if _, err := os.Stat(execPath); err != nil {
			// Fallback to system installation
			execPath = "/Applications/Ollama.app/Contents/Resources/ollama"
		}
	} else {
		// Use system installed Ollama
		execPath = "/Applications/Ollama.app/Contents/Resources/ollama"
	}

	if _, err := os.Stat(execPath); err != nil {
		return nil, fmt.Errorf("ollama executable not found: %s", execPath)
	}

	args := []string{"serve"}

	startMode := StartModeBackground
	if mode == types.EngineStartModeStandard {
		startMode = StartModeForeground
	}

	workDir := ""
	if m.engineConfig.ExecPath != "" {
		workDir = m.engineConfig.ExecPath
	}

	return &StartConfig{
		Name:        m.engineName,
		ExecPath:    execPath,
		Args:        args,
		Env:         env,
		WorkDir:     workDir,
		Mode:        startMode,
		Timeout:     60 * time.Second,
		HealthCheck: healthCheck,
	}, nil
}

// buildOpenvinoConfig builds start configuration for OpenVINO engine
func (m *EngineProcessManager) buildOpenvinoConfig(mode string, healthCheck func() error) (*StartConfig, error) {
	if m.engineConfig == nil {
		return nil, fmt.Errorf("engine config is nil")
	}

	switch runtime.GOOS {
	case "windows":
		return m.buildOpenvinoWindowsConfig(mode, healthCheck)
	case "linux":
		return m.buildOpenvinoLinuxConfig(mode, healthCheck)
	default:
		return nil, fmt.Errorf("openVINO is not supported on %s", runtime.GOOS)
	}
}

// buildOpenvinoWindowsConfig builds Windows-specific OpenVINO configuration
func (m *EngineProcessManager) buildOpenvinoWindowsConfig(mode string, healthCheck func() error) (*StartConfig, error) {
	// Create batch script for Windows
	batchContent := m.generateWindowsBatchScript()
	batchFile := filepath.Join(m.engineConfig.ExecPath, "start_ovms.bat")

	if err := os.WriteFile(batchFile, []byte(batchContent), 0o644); err != nil {
		return nil, fmt.Errorf("failed to create batch file: %v", err)
	}

	startMode := StartModeBackground
	if mode == types.EngineStartModeStandard {
		startMode = StartModeForeground
	}

	return &StartConfig{
		Name:        m.engineName,
		ExecPath:    "cmd",
		Args:        []string{"/C", batchFile},
		WorkDir:     m.engineConfig.EnginePath,
		Mode:        startMode,
		Timeout:     60 * time.Second,
		HealthCheck: healthCheck,
	}, nil
}

// buildOpenvinoLinuxConfig builds Linux-specific OpenVINO configuration
func (m *EngineProcessManager) buildOpenvinoLinuxConfig(mode string, healthCheck func() error) (*StartConfig, error) {
	// Create shell script for Linux
	shellContent := m.generateLinuxShellScript()
	scriptFile := filepath.Join(m.engineConfig.ExecPath, "start_ovms.sh")

	if err := os.WriteFile(scriptFile, []byte(shellContent), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create script file: %v", err)
	}

	startMode := StartModeBackground
	if mode == types.EngineStartModeStandard {
		startMode = StartModeForeground
	}

	return &StartConfig{
		Name:        m.engineName,
		ExecPath:    "/bin/bash",
		Args:        []string{scriptFile},
		WorkDir:     m.engineConfig.EnginePath,
		Mode:        startMode,
		Timeout:     60 * time.Second,
		HealthCheck: healthCheck,
	}, nil
}

// generateWindowsBatchScript generates Windows batch script for OpenVINO
func (m *EngineProcessManager) generateWindowsBatchScript() string {
	modelDir := fmt.Sprintf("%s/models", m.engineConfig.EnginePath)

	return fmt.Sprintf(`@echo on
call "%s\setupvars.bat"
set PATH=%s\python\Scripts;%%PATH%%
set HF_HOME=%s\.cache
set HF_ENDPOINT=https://hf-mirror.com
%s --port 9000 --grpc_bind_address 127.0.0.1 --config_path %s\config.json`,
		m.engineConfig.ExecPath,
		m.engineConfig.ExecPath,
		m.engineConfig.EnginePath,
		filepath.Join(m.engineConfig.ExecPath, m.engineConfig.ExecFile),
		modelDir)
}

// generateLinuxShellScript generates Linux shell script for OpenVINO
func (m *EngineProcessManager) generateLinuxShellScript() string {
	modelDir := fmt.Sprintf("%s/models", m.engineConfig.EnginePath)
	libPath := fmt.Sprintf("%s/ovms/lib", m.engineConfig.ExecPath)
	envPath := fmt.Sprintf("%s/ovms/bin", m.engineConfig.ExecPath)
	pythonPath := fmt.Sprintf("%s/python", libPath)

	return fmt.Sprintf(`#!/bin/bash
export LD_LIBRARY_PATH=%s
export PATH=$PATH:%s
export PYTHONPATH=%s
ovms --port 9000 --grpc_bind_address 127.0.0.1 --config_path %s/config.json`,
		libPath,
		envPath,
		pythonPath,
		modelDir)
}
