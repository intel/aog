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
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/intel/aog/config"
	"github.com/intel/aog/console"
	"github.com/intel/aog/internal/api"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/datastore/jsonds"
	jsondsTemplate "github.com/intel/aog/internal/datastore/jsonds/data"
	"github.com/intel/aog/internal/datastore/sqlite"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/manager"
	"github.com/intel/aog/internal/plugin/registry"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
	"github.com/spf13/cobra"
)

// Global AOG service manager for graceful shutdown
var globalServiceManager *process.AOGServiceManager

// NewApiserverCommand creates the server management command
func NewApiserverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the AOG server lifecycle",
		Long:  "Start, stop, and manage the AOG (AIPC Open Gateway) server for AI services.",
	}

	cmd.AddCommand(
		NewStartApiServerCommand(),
		NewStopApiServerCommand(),
	)

	return cmd
}

// NewStartApiServerCommand creates the start server command
func NewStartApiServerCommand() *cobra.Command {
	config.GlobalEnvironment = config.NewAOGEnvironment()
	logger.InitLogger(logger.LogConfig{LogLevel: config.LogLevelWarn, LogPath: config.GlobalEnvironment.LogDir})
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the AOG server",
		Long:  "Start the AOG (AIPC Open Gateway) server to provide AI services.",
		RunE: func(cmd *cobra.Command, args []string) error {
			isDaemon, err := cmd.Flags().GetBool("daemon")
			if err != nil {
				return err
			}

			isDebug, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return err
			}

			var logLevel string
			if isDebug {
				logLevel = config.LogLevelDebug
			} else if isDaemon {
				logLevel = config.DefaultLogLevel
			} else {
				logLevel = config.LogLevelDebug
			}

			config.GlobalEnvironment.LogLevel = logLevel
			logger.InitLogger(logger.LogConfig{LogLevel: logLevel, LogPath: config.GlobalEnvironment.LogDir})

			var startMode string
			if isDebug {
				startMode = types.EngineStartModeStandard
			} else {
				startMode = types.EngineStartModeDaemon
			}

			// daemon模式：让进程在后台运行
			if isDaemon {
				if err := detachProcess(); err != nil {
					logger.EngineLogger.Error(fmt.Sprintf("Failed to detach process: %v", err))
					return fmt.Errorf("failed to start in daemon mode: %v", err)
				}
			}

			// Initialize Provider Factory
			// 1. Create builtin factory for built-in engines
			builtinFactory := provider.NewBuiltinProviderFactory()

			// 2. Initialize converters before plugin discovery (plugins need converters for template loading)
			if err := schedule.InitConverters(); err != nil {
				logger.EngineLogger.Error("Failed to initialize converters", "error", err)
				return fmt.Errorf("failed to initialize converters: %w", err)
			}
			logger.EngineLogger.Info("Converters initialized successfully")

			// 3. Initialize datastore early for plugin service registration
			ds, err := sqlite.New(config.GlobalEnvironment.Datastore)
			if err != nil {
				logger.EngineLogger.Error("Failed to initialize datastore", "error", err)
				return fmt.Errorf("failed to initialize datastore: %w", err)
			}
			vm := sqlite.NewSQLiteVersionManager(ds)
			if err := sqlite.MigrateToLatest(vm, ds); err != nil {
				logger.EngineLogger.Error("Failed to migrate database", "error", err)
				return fmt.Errorf("failed to migrate database: %w", err)
			}
			datastore.SetDefaultDatastore(ds)
			logger.EngineLogger.Info("Datastore initialized successfully")

			// 4. Create plugin registry and discover plugins
			// Development mode: use current working directory's plugins/ if it exists
			// Production mode: use configured plugin directory
			pluginDir := filepath.Join(config.GlobalEnvironment.RootDir, "plugins")
			if cwd, err := os.Getwd(); err == nil {
				devPluginDir := filepath.Join(cwd, "plugins")
				if _, err := os.Stat(devPluginDir); err == nil {
					pluginDir = devPluginDir
					logger.EngineLogger.Info("Using development plugin directory", "path", pluginDir)
				}
			}
			logger.EngineLogger.Info("Initializing plugin system...", "pluginDir", pluginDir)

			pluginRegistry := registry.NewPluginRegistry(pluginDir, ds)

			registry.SetGlobalPluginRegistry(pluginRegistry)

			pluginRegistry.SetFlavorRegistrar(func(manifest *sdktypes.PluginManifest) error {
				flavor, err := schedule.NewPluginBasedAPIFlavor(manifest)
				if err != nil {
					return fmt.Errorf("failed to create plugin flavor: %w", err)
				}
				schedule.RegisterAPIFlavor(flavor)
				logger.EngineLogger.Info("Plugin registered as APIFlavor",
					"plugin", manifest.Provider.Name,
					"services", len(manifest.Services))
				return nil
			})

			if err := pluginRegistry.DiscoverPlugins(); err != nil {
				logger.EngineLogger.Warn("Failed to discover plugins, continuing with built-in engines only", "error", err)
			} else {
				// 显示插件发现结果
				manifests := pluginRegistry.GetAllManifests()
				if len(manifests) > 0 {
					logger.EngineLogger.Info("Plugin discovery succeeded",
						"total", len(manifests),
						"directory", pluginDir)
				} else {
					logger.EngineLogger.Info("No plugins found in directory", "directory", pluginDir)
				}
			}
			// Note: Plugin service providers are automatically registered to datastore
			// during DiscoverPlugins() via FlavorRegistrar callback

			// 5. Create composite factory (built-in engines first, then plugins)
			compositeFactory := provider.NewCompositeProviderFactory(builtinFactory, pluginRegistry)
			provider.InitProviderFactory(compositeFactory)

			logger.EngineLogger.Info("Plugin hot-plug monitoring: planned for future release")

			defer pluginRegistry.Shutdown()

			if err := provider.InitEngines([]string{}); err != nil {
				logger.EngineLogger.Error("Failed to initialize engines", "error", err)
				return fmt.Errorf("failed to initialize engines: %v", err)
			}

			// Initialize global AOG service manager and inject engine manager
			globalServiceManager = process.GetAOGServiceManager()
			globalServiceManager.SetEngineManager(provider.GetEngineManager())
			ctx := context.Background()
			go pluginRegistry.ScheduleLoadPlugin(ctx)

			return Run(ctx, startMode)
		},
	}

	cmd.Flags().BoolP("daemon", "d", false, "Start the server in daemon mode")
	cmd.Flags().BoolP("verbose", "v", false, "Enable debug mode")
	return cmd
}

// NewStopApiServerCommand creates the stop server command
func NewStopApiServerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the AOG server",
		Long:  "Stop the running AOG (AIPC Open Gateway) server process.",
		Args:  cobra.ExactArgs(0),
		RunE:  stopAogServer,
	}
}

// stopAogServer stops the AOG server gracefully
func stopAogServer(cmd *cobra.Command, args []string) error {
	fmt.Println("Stopping AOG server gracefully...")

	// Send graceful shutdown signal to running AOG process
	if err := sendShutdownSignal(); err != nil {
		fmt.Printf("Warning: Failed to send graceful shutdown signal: %v\n", err)
		fmt.Println("Falling back to process termination...")

		// Fallback to process termination
		return stopAOGProcesses()
	}

	// Check if the process actually stopped after graceful shutdown
	if runtime.GOOS == "windows" {
		// On Windows, check if the process actually terminated
		pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
		for i := 0; i < 10; i++ { // Check for up to 10 seconds
			if _, err := os.Stat(pidFile); os.IsNotExist(err) {
				fmt.Println("AOG server stopped successfully")
				return nil
			}
			time.Sleep(1 * time.Second)
		}

		fmt.Println("Warning: Process may not have stopped completely, checking manually...")
		// If still running, fallback to force termination
		return stopAOGProcesses()
	}

	fmt.Println("Graceful shutdown signal sent successfully")
	return nil
}

// sendShutdownSignal sends a graceful shutdown signal to running AOG process (cross-platform)
func sendShutdownSignal() error {
	if runtime.GOOS == "windows" {
		return sendWindowsShutdownSignal()
	}
	return sendUnixShutdownSignal()
}

// sendUnixShutdownSignal sends SIGTERM signal on Unix-like systems
func sendUnixShutdownSignal() error {
	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return fmt.Errorf("no running AOG process found")
	}

	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return fmt.Errorf("invalid PID format: %v", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %v", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %v", err)
	}

	return nil
}

// sendWindowsShutdownSignal sends shutdown signal on Windows using HTTP endpoint
func sendWindowsShutdownSignal() error {
	// Use HTTP API to trigger graceful shutdown
	client := &http.Client{Timeout: 5 * time.Second}

	// Try default API endpoint for graceful shutdown
	shutdownURL := fmt.Sprintf("http://%s/_internal/shutdown", config.GlobalEnvironment.ApiHost)

	resp, err := client.Post(shutdownURL, "application/json", strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf("failed to send shutdown request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shutdown request failed with status: %d", resp.StatusCode)
	}

	fmt.Println("Graceful shutdown signal sent via HTTP API")

	// Wait a moment to allow graceful shutdown to complete
	fmt.Println("Waiting for graceful shutdown to complete...")
	time.Sleep(1 * time.Second)

	return nil
}

// stopAOGProcesses forcefully stops AOG processes (fallback method)
func stopAOGProcesses() error {
	files, err := filepath.Glob(filepath.Join(config.GlobalEnvironment.RootDir, "*.pid"))
	if err != nil {
		return fmt.Errorf("failed to list pid files: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("No running processes found")
		return nil
	}

	// Stop processes using PID files (fallback only)
	for _, pidFile := range files {
		if err := stopProcessByPIDFile(pidFile); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
	}

	// Clean up remaining processes on Windows
	if runtime.GOOS == "windows" {
		cleanupWindowsProcesses()
	}

	return nil
}

// stopProcessByPIDFile stops a process using its PID file
func stopProcessByPIDFile(pidFile string) error {
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file %s: %v", pidFile, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return fmt.Errorf("invalid PID in file %s: %v", pidFile, err)
	}

	// Check if process actually exists before trying to kill
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process not found, just clean up the PID file
		os.Remove(pidFile)
		return nil
	}

	// Try to kill the process
	if err := process.Kill(); err != nil {
		if strings.Contains(err.Error(), "process already finished") ||
			strings.Contains(err.Error(), "parameter is incorrect") {
			// Process already stopped, just clean up
			os.Remove(pidFile)
			return nil
		} else {
			return fmt.Errorf("failed to kill process with PID %d: %v", pid, err)
		}
	} else {
		fmt.Printf("Successfully stopped process with PID %d\n", pid)
	}

	// Remove PID file
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: Failed to remove PID file %s: %v\n", pidFile, err)
	}

	return nil
}

// cleanupWindowsProcesses cleans up remaining processes on Windows
func cleanupWindowsProcesses() {
	processes := []string{"ollama-lib.exe", "ovms.exe"}

	for _, processName := range processes {
		cmd := exec.Command("taskkill", "/IM", processName, "/F")
		if err := cmd.Run(); err == nil {
			fmt.Printf("Successfully killed process: %s\n", processName)
		}
	}
}

// Run starts AOG server with graceful shutdown support
func Run(ctx context.Context, startMode string) error {
	// Start AOG internal services with engines
	logger.EngineLogger.Info("[AOGService] Starting AOG internal services with engines...")
	if err := globalServiceManager.StartServices(ctx, true, startMode); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("Some engines failed to start: %v", err))
		// Continue with HTTP server startup even if some engines fail
	}

	// Initialize all other components (datastore, API server, etc.)
	aogServer, err := initializeAOGServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize AOG server: %v", err)
	}

	// Create HTTP server for graceful shutdown support
	httpServer := &http.Server{
		Addr:    config.GlobalEnvironment.ApiHost,
		Handler: aogServer.Router,
	}

	// Register HTTP server with service manager for graceful shutdown
	globalServiceManager.SetHTTPServer(httpServer)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in a goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		logger.EngineLogger.Info("Starting HTTP server...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- fmt.Errorf("HTTP server error: %v", err)
		}
	}()

	// Write PID file (only for external stop command compatibility)
	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("Failed to write PID file: %v", err))
	}

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigCh:
		logger.EngineLogger.Info(fmt.Sprintf("Received signal %v, initiating graceful shutdown...", sig))
		return performGracefulShutdown(ctx)
	case err := <-serverErrCh:
		logger.EngineLogger.Error(fmt.Sprintf("HTTP server error: %v", err))
		return performGracefulShutdown(ctx)
	case <-globalServiceManager.WaitForShutdown():
		logger.EngineLogger.Info("Shutdown signal received from service manager")
		return performGracefulShutdown(ctx)
	}
}

// initializeAOGServer initializes all AOG server components
// Note: Datastore is already initialized before plugin discovery
func initializeAOGServer(ctx context.Context) (*api.AOGCoreServer, error) {
	// Get the already initialized datastore
	ds := datastore.GetDefaultDatastore()
	if ds == nil {
		return nil, fmt.Errorf("datastore not initialized")
	}

	// Initialize control panel
	jds := jsonds.NewJSONDatastore(jsondsTemplate.JsonDataStoreFS)
	err := jds.Init()
	if err != nil {
		fmt.Printf("Failed to initialize json file store: %v\n", err)
	}
	datastore.SetDefaultJsonDatastore(jds)

	// Initialize core app server
	aogServer := api.NewAOGCoreServer()
	aogServer.Register()

	logger.LogicLogger.Info("start_app")

	// Load all flavors
	err = schedule.InitAPIFlavors()
	if err != nil {
		slog.Error("[Init] Failed to load API Flavors", "error", err)
		return nil, err
	}

	// Start scheduler
	schedule.StartScheduler("basic")

	// Initialize the model memory manager
	mmm := manager.GetModelManager()
	mmm.SetIdleTimeout(config.GlobalEnvironment.ModelIdleTimeout)
	mmm.Start(config.GlobalEnvironment.ModelCleanupInterval)
	logger.LogicLogger.Info("[Init] Model memory manager started",
		"idle_timeout", config.GlobalEnvironment.ModelIdleTimeout,
		"cleanup_interval", config.GlobalEnvironment.ModelCleanupInterval)

	// Inject the router
	api.InjectRouter(aogServer)

	// Setup flavors
	for _, flavor := range schedule.AllAPIFlavors() {
		flavor.InstallRoutes(aogServer.Router)
		schedule.InitProviderDefaultModelTemplate(flavor)
	}

	// Register console routes
	err = console.RegisterConsoleRoutes(aogServer.Router)
	if err != nil {
		fmt.Println("[Warn] Console static files not found, skip console panel:", err)
	}

	// Register internal shutdown endpoint for Windows graceful shutdown
	aogServer.Router.POST("/_internal/shutdown", handleGracefulShutdown)

	return aogServer, nil
}

// handleGracefulShutdown handles HTTP-based graceful shutdown requests (for Windows)
func handleGracefulShutdown(c *gin.Context) {
	logger.EngineLogger.Info("Received graceful shutdown request via HTTP API")

	// Send success response immediately
	c.JSON(http.StatusOK, gin.H{"message": "Graceful shutdown initiated"})

	// Trigger graceful shutdown asynchronously
	go func() {
		// Small delay to ensure response is sent
		time.Sleep(100 * time.Millisecond)

		if globalServiceManager != nil {
			ctx := context.Background()
			logger.EngineLogger.Info("Initiating graceful shutdown via HTTP API...")
			_ = performGracefulShutdown(ctx)
			// Exit the process after graceful shutdown
			os.Exit(0)
		}
	}()
}

// performGracefulShutdown performs graceful shutdown of all components
func performGracefulShutdown(ctx context.Context) error {
	logger.EngineLogger.Info("Performing graceful shutdown...")

	// Shutdown plugins first (before engines, to avoid hanging connections)
	pluginRegistry := registry.GetGlobalPluginRegistry()
	if pluginRegistry != nil {
		logger.EngineLogger.Info("Shutting down all plugins...")
		pluginRegistry.Shutdown()
		logger.EngineLogger.Info("All plugins shut down successfully")
	}

	// Stop AOG internal services (includes engines)
	if globalServiceManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := globalServiceManager.StopServices(ctx); err != nil {
			logger.EngineLogger.Error(fmt.Sprintf("Error during AOG service shutdown: %v", err))
		}
	}

	// Clean up PID file
	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		logger.EngineLogger.Warn(fmt.Sprintf("Failed to remove PID file: %v", err))
	}

	logger.EngineLogger.Info("Graceful shutdown completed")
	return nil
}

// detachProcess detaches the current process to run in background (daemon mode)
func detachProcess() error {
	if runtime.GOOS == "windows" {
		// Windows doesn't have fork, we'll implement a different approach
		return detachWindowsProcess()
	} else {
		// Unix-like systems: fork and exit parent
		return detachUnixProcess()
	}
}

// detachWindowsProcess handles daemon mode on Windows
func detachWindowsProcess() error {
	// On Windows, we create a new process and exit the parent
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Build arguments without the -d flag to avoid infinite recursion
	args := []string{"server", "start"}
	for _, arg := range os.Args[1:] {
		if arg != "-d" && arg != "--daemon" {
			args = append(args, arg)
		}
	}

	// Create command
	cmd := exec.Command(execPath, args...)

	// Redirect outputs to log file
	logPath := config.GlobalEnvironment.ConsoleLog
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// Start the background process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background process: %v", err)
	}

	fmt.Printf("AOG server started in background with PID: %d\n", cmd.Process.Pid)

	// Write PID file for compatibility
	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("Failed to write PID file: %v", err))
	}

	// Exit parent process
	os.Exit(0)
	return nil
}

// detachUnixProcess handles daemon mode on Unix-like systems
func detachUnixProcess() error {
	// Check if we're already detached (to avoid double fork)
	if os.Getppid() == 1 {
		// We're already running as a daemon (parent PID is init)
		return nil
	}

	// Fork to create child process
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Build arguments without the -d flag to avoid infinite recursion
	args := []string{"server", "start"}
	for _, arg := range os.Args[1:] {
		if arg != "-d" && arg != "--daemon" {
			args = append(args, arg)
		}
	}

	// Create child process
	cmd := exec.Command(execPath, args...)
	// For Unix-like systems, detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	// Redirect outputs to log file
	logPath := config.GlobalEnvironment.ConsoleLog
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil // Close stdin

	// Start the background process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background process: %v", err)
	}

	fmt.Printf("AOG server started in background with PID: %d\n", cmd.Process.Pid)

	// Write PID file for compatibility
	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644); err != nil {
		logger.EngineLogger.Warn(fmt.Sprintf("Failed to write PID file: %v", err))
	}

	// Exit parent process
	os.Exit(0)
	return nil
}
