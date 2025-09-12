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
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/logger"
)

// EngineManagerInterface defines the interface for engine management
type EngineManagerInterface interface {
	StartAllEngines(mode string) error
	StopAllEngines() error
	StartKeepAlive()
	StopKeepAlive()
}

// =============================================================================
// AOGServiceManager - 内部服务管理器
// 用于AOG进程内部管理引擎和HTTP服务的生命周期
// =============================================================================

// AOGServiceManager manages AOG internal services (engines, HTTP server)
type AOGServiceManager struct {
	mu         sync.RWMutex
	status     ProcessStatus
	startTime  time.Time
	httpServer interface{}   // HTTP服务器引用，用于优雅关闭
	shutdownCh chan struct{} // 优雅关闭信号通道
	ctx        context.Context
	cancel     context.CancelFunc

	// 引擎管理器，通过接口依赖注入
	engineManager EngineManagerInterface
}

// NewAOGServiceManager creates a new AOG service manager for internal use
func NewAOGServiceManager() *AOGServiceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &AOGServiceManager{
		status:     ProcessStatusStopped,
		shutdownCh: make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetEngineManager sets the engine manager for the service manager
func (m *AOGServiceManager) SetEngineManager(engineManager EngineManagerInterface) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engineManager = engineManager
}

// SetHTTPServer sets the HTTP server reference for graceful shutdown
func (m *AOGServiceManager) SetHTTPServer(server interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.httpServer = server
}

// StartServices starts internal services (engines, etc.) - NO process creation
func (m *AOGServiceManager) StartServices(ctx context.Context, startEngines bool, engineMode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == ProcessStatusRunning {
		logger.EngineLogger.Info("[AOGService] AOG services are already running")
		return nil
	}

	m.status = ProcessStatusStarting
	m.startTime = time.Now()

	logger.EngineLogger.Info("[AOGService] Starting AOG internal services...")

	// Start engines if requested
	if startEngines && m.engineManager != nil {
		logger.EngineLogger.Info("[AOGService] Starting engines...")
		if err := m.engineManager.StartAllEngines(engineMode); err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("[AOGService] Some engines failed to start: %v", err))
		}

		// 启动引擎保活监控
		m.engineManager.StartKeepAlive()
	}

	m.status = ProcessStatusRunning
	logger.EngineLogger.Info("[AOGService] AOG internal services started successfully")

	return nil
}

// StopServices gracefully stops internal services - NO process termination
func (m *AOGServiceManager) StopServices(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == ProcessStatusStopped {
		logger.EngineLogger.Info("[AOGService] AOG services are already stopped")
		return nil
	}

	m.status = ProcessStatusStopping
	logger.EngineLogger.Info("[AOGService] Gracefully stopping AOG internal services...")

	// Signal shutdown to any waiting goroutines (check if channel is still open)
	select {
	case <-m.shutdownCh:
		// Channel already closed
	default:
		close(m.shutdownCh)
	}

	// Cancel context to stop all operations
	if m.cancel != nil {
		m.cancel()
	}

	// Gracefully shutdown HTTP server if available
	if m.httpServer != nil {
		if server, ok := m.httpServer.(interface{ Shutdown(context.Context) error }); ok {
			logger.EngineLogger.Info("[AOGService] Shutting down HTTP server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := server.Shutdown(shutdownCtx); err != nil {
				logger.EngineLogger.Error(fmt.Sprintf("[AOGService] HTTP server shutdown error: %v", err))
			} else {
				logger.EngineLogger.Info("[AOGService] HTTP server stopped gracefully")
			}
		}
	}

	// 停止保活监控和所有引擎
	if m.engineManager != nil {
		m.engineManager.StopKeepAlive()
		logger.EngineLogger.Info("[AOGService] Stopping all engines...")
		if err := m.engineManager.StopAllEngines(); err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("[AOGService] Some engines failed to stop gracefully: %v", err))
		}
	}

	m.status = ProcessStatusStopped
	logger.EngineLogger.Info("[AOGService] AOG internal services stopped successfully")

	return nil
}

// WaitForShutdown waits for shutdown signal
func (m *AOGServiceManager) WaitForShutdown() <-chan struct{} {
	return m.shutdownCh
}

// GetStatus returns the service status
func (m *AOGServiceManager) GetStatus() ProcessStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// =============================================================================
// AOGProcessManager - 外部进程管理器
// 用于从外部启动/停止AOG进程本身
// =============================================================================

// AOGProcessManager manages the AOG server process from external commands
type AOGProcessManager struct {
	processManager ProcessManager
	execPath       string
	logPath        string
	rootDir        string
}

// globalServiceManager for internal service management
var globalServiceManager *AOGServiceManager

// GetAOGServiceManager returns the global AOG service manager for internal use
func GetAOGServiceManager() *AOGServiceManager {
	if globalServiceManager == nil {
		globalServiceManager = NewAOGServiceManager()
	}
	return globalServiceManager
}

// globalProcessManager for external process management
var (
	globalProcessManager *AOGProcessManager
	processManagerOnce   sync.Once
)

// GetAOGProcessManager returns the global AOG process manager for external use
func GetAOGProcessManager() (*AOGProcessManager, error) {
	var err error
	processManagerOnce.Do(func() {
		globalProcessManager, err = newAOGProcessManager()
	})
	if err != nil {
		return nil, err
	}
	return globalProcessManager, nil
}

// newAOGProcessManager creates a new AOG process manager
func newAOGProcessManager() (*AOGProcessManager, error) {
	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %v", err)
	}

	// Get configuration paths
	logPath := config.GlobalEnvironment.ConsoleLog
	rootDir := config.GlobalEnvironment.RootDir

	return &AOGProcessManager{
		processManager: NewProcessManager("aog-server"),
		execPath:       execPath,
		logPath:        logPath,
		rootDir:        rootDir,
	}, nil
}

// StartProcess starts the AOG server process
func (m *AOGProcessManager) StartProcess(daemon bool) error {
	if daemon {
		return m.startProcessDaemon()
	} else {
		return m.startProcessForeground()
	}
}

// StartProcessDaemon starts the AOG server process in daemon mode
func (m *AOGProcessManager) StartProcessDaemon() error {
	return m.StartProcess(true)
}

// startProcessDaemon starts the server process in daemon mode
func (m *AOGProcessManager) startProcessDaemon() error {
	ctx := context.Background()

	// Prepare log file
	logFile, err := os.OpenFile(m.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	// For daemon mode, start server without -d flag (to avoid double-daemon)
	config := &StartConfig{
		Name:        "aog-server",
		ExecPath:    m.execPath,
		Args:        []string{"server", "start"}, // No -d flag needed here
		WorkDir:     m.rootDir,
		Mode:        StartModeBackground,
		Timeout:     30 * time.Second,
		HealthCheck: m.healthCheck,
	}

	// Start the process using ProcessManager's graceful capabilities
	if err := m.processManager.Start(ctx, config); err != nil {
		return fmt.Errorf("failed to start AOG server: %v", err)
	}

	logger.EngineLogger.Info(fmt.Sprintf("[AOGProcess] Server started in daemon mode with PID: %d", m.processManager.PID()))

	// Wait for server to be ready
	if err := m.waitForReady(10 * time.Second); err != nil {
		return fmt.Errorf("server failed to become ready: %v", err)
	}

	return nil
}

// startProcessForeground starts the server process in foreground mode
func (m *AOGProcessManager) startProcessForeground() error {
	ctx := context.Background()

	config := &StartConfig{
		Name:        "aog-server",
		ExecPath:    m.execPath,
		Args:        []string{"server", "start"},
		WorkDir:     m.rootDir,
		Mode:        StartModeForeground,
		Timeout:     30 * time.Second,
		HealthCheck: m.healthCheck,
	}

	// Start the process using ProcessManager's graceful capabilities
	if err := m.processManager.Start(ctx, config); err != nil {
		return fmt.Errorf("failed to start AOG server: %v", err)
	}

	logger.EngineLogger.Info(fmt.Sprintf("[AOGProcess] Server started in foreground with PID: %d", m.processManager.PID()))

	return nil
}

// StopProcess stops the AOG server process gracefully
func (m *AOGProcessManager) StopProcess() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.EngineLogger.Info("[AOGProcess] Stopping server process gracefully...")

	if err := m.processManager.Stop(ctx); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("[AOGProcess] Graceful stop failed, force killing: %v", err))
		return m.processManager.Kill()
	}

	logger.EngineLogger.Info("[AOGProcess] Server process stopped successfully")
	return nil
}

// StopProcessWithFallback stops the process with legacy PID file fallback
func (m *AOGProcessManager) StopProcessWithFallback(pidFilePath string) error {
	// Try graceful shutdown first
	err := m.StopProcess()
	if err == nil {
		// Clean up any remaining PID files
		m.cleanupStalePIDFiles(pidFilePath)
		return nil
	}

	logger.EngineLogger.Warn(fmt.Sprintf("Graceful shutdown failed, falling back to legacy method: %v", err))

	// Fallback to old PID file method
	return m.stopProcessLegacy(pidFilePath)
}

// IsProcessRunning checks if the AOG process is running
func (m *AOGProcessManager) IsProcessRunning() bool {
	if m.processManager == nil {
		// If no process manager, fall back to health check
		return m.healthCheck() == nil
	}
	
	// If process manager knows about a running process, trust it
	if m.processManager.IsRunning() {
		return true
	}
	
	// If process manager doesn't know about a running process,
	// check via health endpoint (maybe server was started by another instance)
	return m.healthCheck() == nil
}

// IsProcessHealthy checks if the process is running and healthy
func (m *AOGProcessManager) IsProcessHealthy() bool {
	if !m.IsProcessRunning() {
		return false
	}
	return m.healthCheck() == nil
}

// GetProcessPID returns the process PID, or -1 if not running
func (m *AOGProcessManager) GetProcessPID() int {
	if m.processManager == nil {
		return -1
	}
	return m.processManager.PID()
}

// WaitForReady waits for the process to become ready with timeout
func (m *AOGProcessManager) WaitForReady(timeout time.Duration) error {
	return m.waitForReady(timeout)
}

// healthCheck checks if the AOG server is healthy
func (m *AOGProcessManager) healthCheck() error {
	client := &http.Client{Timeout: 3 * time.Second}

	// Try primary health endpoint
	resp, err := client.Get("http://localhost:8012/health")
	if err != nil {
		// Try fallback endpoint
		resp, err = client.Get("http://127.0.0.1:16688/health")
		if err != nil {
			return fmt.Errorf("health check failed: %v", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// waitForReady waits for the server to be ready with timeout
func (m *AOGProcessManager) waitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server to be ready")
		case <-ticker.C:
			if err := m.healthCheck(); err == nil {
				return nil
			}
			// Continue checking
		}
	}
}

// stopProcessLegacy provides fallback using the old PID file method
func (m *AOGProcessManager) stopProcessLegacy(pidFilePath string) error {
	files, err := filepath.Glob(pidFilePath)
	if err != nil {
		return fmt.Errorf("failed to list pid files: %v", err)
	}

	if len(files) == 0 {
		logger.EngineLogger.Info("No running processes found")
		return nil
	}

	for _, pidFile := range files {
		pidData, err := os.ReadFile(pidFile)
		if err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("Failed to read PID file %s: %v", pidFile, err))
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("Invalid PID in file %s: %v", pidFile, err))
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("Failed to find process with PID %d: %v", pid, err))
			continue
		}

		if err := process.Kill(); err != nil {
			if strings.Contains(err.Error(), "process already finished") {
				logger.EngineLogger.Info(fmt.Sprintf("Process with PID %d is already stopped", pid))
			} else {
				logger.EngineLogger.Warn(fmt.Sprintf("Failed to kill process with PID %d: %v", pid, err))
				continue
			}
		} else {
			logger.EngineLogger.Info(fmt.Sprintf("Successfully stopped process with PID %d", pid))
		}

		if err := os.Remove(pidFile); err != nil {
			logger.EngineLogger.Warn(fmt.Sprintf("Failed to remove PID file %s: %v", pidFile, err))
		}
	}
	return nil
}

// cleanupStalePIDFiles removes any stale PID files
func (m *AOGProcessManager) cleanupStalePIDFiles(pidFilePath string) {
	files, err := filepath.Glob(pidFilePath)
	if err != nil {
		return
	}

	for _, pidFile := range files {
		if err := os.Remove(pidFile); err == nil {
			logger.EngineLogger.Info(fmt.Sprintf("Cleaned up stale PID file: %s", pidFile))
		}
	}
}

// =============================================================================
// 向后兼容接口 - 保持现有代码正常工作
// =============================================================================

// GetAOGManager 返回进程管理器以保持向后兼容
// Deprecated: Use GetAOGProcessManager for external process management
func GetAOGManager() (*AOGProcessManager, error) {
	return GetAOGProcessManager()
}
