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
	"os/exec"
	"sync"
	"time"

	"github.com/intel/aog/internal/logger"
)

// BaseProcessManager provides common process management functionality
type BaseProcessManager struct {
	mu           sync.RWMutex
	name         string
	handle       *ProcessHandle
	status       ProcessStatus
	platformImpl PlatformProcess
	lastError    error
	healthCheck  func() error
	startTime    time.Time
}

// NewProcessManager creates a new process manager instance
func NewProcessManager(name string) ProcessManager {
	return &BaseProcessManager{
		name:         name,
		status:       ProcessStatusStopped,
		platformImpl: newPlatformProcess(),
	}
}

// Start starts the process with given configuration
func (m *BaseProcessManager) Start(ctx context.Context, config *StartConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == ProcessStatusRunning {
		logger.EngineLogger.Info(fmt.Sprintf("[Process] %s is already running", m.name))
		return nil
	}

	if m.status == ProcessStatusStarting {
		return fmt.Errorf("process %s is already starting", m.name)
	}

	m.status = ProcessStatusStarting
	m.lastError = nil
	m.healthCheck = config.HealthCheck

	logger.EngineLogger.Info(fmt.Sprintf("[Process] Starting %s...", m.name))

	// Build command
	cmd := exec.CommandContext(ctx, config.ExecPath, config.Args...)
	if config.WorkDir != "" {
		cmd.Dir = config.WorkDir
	}
	if len(config.Env) > 0 {
		cmd.Env = append(os.Environ(), config.Env...)
	}

	// Start the process using platform-specific implementation
	handle, err := m.platformImpl.StartProcess(cmd, config.Mode)
	if err != nil {
		m.status = ProcessStatusError
		m.lastError = err
		logger.EngineLogger.Error(fmt.Sprintf("[Process] Failed to start %s: %v", m.name, err))
		return fmt.Errorf("failed to start process %s: %v", m.name, err)
	}

	m.handle = handle
	m.startTime = time.Now()

	// Wait for process to be ready if health check is provided
	if config.HealthCheck != nil && config.Timeout > 0 {
		if err := m.waitForHealth(ctx, config.Timeout, config.HealthCheck); err != nil {
			// If health check fails, stop the process
			m.platformImpl.KillProcess(m.handle)
			m.handle = nil
			m.status = ProcessStatusError
			m.lastError = err
			logger.EngineLogger.Error(fmt.Sprintf("[Process] %s failed health check: %v", m.name, err))
			return fmt.Errorf("process %s failed health check: %v", m.name, err)
		}
	}

	m.status = ProcessStatusRunning
	logger.EngineLogger.Info(fmt.Sprintf("[Process] %s started successfully with PID: %d", m.name, handle.PID))

	return nil
}

// Stop gracefully stops the process
func (m *BaseProcessManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != ProcessStatusRunning {
		logger.EngineLogger.Info(fmt.Sprintf("[Process] %s is not running", m.name))
		return nil
	}

	if m.handle == nil {
		m.status = ProcessStatusStopped
		return nil
	}

	m.status = ProcessStatusStopping
	logger.EngineLogger.Info(fmt.Sprintf("[Process] Gracefully stopping %s (PID: %d)...", m.name, m.handle.PID))

	// Use platform-specific graceful shutdown
	if err := m.platformImpl.GracefulShutdown(ctx, m.handle); err != nil {
		logger.EngineLogger.Error(fmt.Sprintf("[Process] Failed to gracefully stop %s: %v", m.name, err))
		// Force kill as fallback
		if killErr := m.platformImpl.KillProcess(m.handle); killErr != nil {
			logger.EngineLogger.Error(fmt.Sprintf("[Process] Failed to kill %s: %v", m.name, killErr))
			return fmt.Errorf("failed to stop process %s: %v (kill also failed: %v)", m.name, err, killErr)
		}
	}

	m.handle = nil
	m.status = ProcessStatusStopped
	logger.EngineLogger.Info(fmt.Sprintf("[Process] %s stopped successfully", m.name))

	return nil
}

// Kill forcefully terminates the process
func (m *BaseProcessManager) Kill() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handle == nil {
		m.status = ProcessStatusStopped
		return nil
	}

	logger.EngineLogger.Info(fmt.Sprintf("[Process] Force killing %s (PID: %d)...", m.name, m.handle.PID))

	if err := m.platformImpl.KillProcess(m.handle); err != nil {
		logger.EngineLogger.Error(fmt.Sprintf("[Process] Failed to kill %s: %v", m.name, err))
		return fmt.Errorf("failed to kill process %s: %v", m.name, err)
	}

	m.handle = nil
	m.status = ProcessStatusStopped
	logger.EngineLogger.Info(fmt.Sprintf("[Process] %s killed successfully", m.name))

	return nil
}

// IsRunning checks if the process is currently running
func (m *BaseProcessManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.handle == nil {
		return false
	}

	return m.platformImpl.IsProcessRunning(m.handle)
}

// Status returns the current process status
func (m *BaseProcessManager) Status() ProcessStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// PID returns the process ID (0 if not running)
func (m *BaseProcessManager) PID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.handle == nil {
		return 0
	}
	return m.handle.PID
}

// Info returns process information
func (m *BaseProcessManager) Info() ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ProcessInfo{
		PID:    m.PID(),
		Status: m.status,
		Name:   m.name,
	}
}

// waitForHealth waits for the process to pass health check
func (m *BaseProcessManager) waitForHealth(ctx context.Context, timeout time.Duration, healthCheck func() error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timeout after %v", timeout)
		case <-ticker.C:
			if err := healthCheck(); err == nil {
				return nil
			}
			// Continue checking
		}
	}
}
