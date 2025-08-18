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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/intel/aog/cmd/cli/core/common"
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
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	"github.com/spf13/cobra"
)

// NewApiserverCommand creates the server management command
func NewApiserverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage " + constants.AppName + " server",
		Long:  "Manage " + constants.AppName + " server (start, stop, etc.)",
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
		Short: "apiserver is a aipc open gateway",
		Long:  "apiserver is a aipc open gateway",
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
				logLevel = config.LogLevelWarn
			} else {
				logLevel = config.LogLevelError
			}

			config.GlobalEnvironment.LogLevel = logLevel
			logger.InitLogger(logger.LogConfig{LogLevel: logLevel, LogPath: config.GlobalEnvironment.LogDir})

			if isDaemon {
				common.StartAOGServer(cmd, args)
				return nil
			}

			startMode := types.EngineStartModeDaemon
			if isDebug {
				startMode = types.EngineStartModeStandard
			}

			err = StartModelEngine("openvino", startMode)
			if err != nil {
				return err
			}

			err = StartModelEngine("ollama", startMode)
			if err != nil {
				return err
			}

			return Run(context.Background())
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
		Short: "Stop daemon server.",
		Long:  "Stop daemon server.",
		Args:  cobra.ExactArgs(0),
		RunE:  stopAogServer,
	}
}

// stopAogServer stops the AOG server
func stopAogServer(cmd *cobra.Command, args []string) error {
	files, err := filepath.Glob(filepath.Join(config.GlobalEnvironment.RootDir, "*.pid"))
	if err != nil {
		return fmt.Errorf("failed to list pid files: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("No running processes found")
		return nil
	}

	// Traverse all pid files.
	for _, pidFile := range files {
		pidData, err := os.ReadFile(pidFile)
		if err != nil {
			fmt.Printf("Failed to read PID file %s: %v\n", pidFile, err)
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if err != nil {
			fmt.Printf("Invalid PID in file %s: %v\n", pidFile, err)
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("Failed to find process with PID %d: %v\n", pid, err)
			continue
		}

		if err := process.Kill(); err != nil {
			if strings.Contains(err.Error(), "process already finished") {
				fmt.Printf("Process with PID %d is already stopped\n", pid)
			} else {
				fmt.Printf("Failed to kill process with PID %d: %v\n", pid, err)
				continue
			}
		} else {
			fmt.Printf("Successfully stopped process with PID %d\n", pid)
		}

		// remove pid file
		if err := os.Remove(pidFile); err != nil {
			fmt.Printf("Failed to remove PID file %s: %v\n", pidFile, err)
		}
	}
	if runtime.GOOS == "windows" {
		extraProcessName := "ollama-lib.exe"
		extraCmd := exec.Command("taskkill", "/IM", extraProcessName, "/F")
		_, err := extraCmd.CombinedOutput()
		if err != nil {
			// fmt.Printf("failed to kill process: %s", extraProcessName)
			return nil
		}

		ovmsProcessName := "ovms.exe"
		ovmsCmd := exec.Command("taskkill", "/IM", ovmsProcessName, "/F")
		_, err = ovmsCmd.CombinedOutput()
		if err != nil {
			// fmt.Printf("failed to kill process: %s", ovmsProcessName)
			return nil
		}

		fmt.Printf("Successfully killed process: %s\n", extraProcessName)
	}

	return nil
}

// Run starts the AOG server
func Run(ctx context.Context) error {
	// Initialize the datastore
	ds, err := sqlite.New(config.GlobalEnvironment.Datastore)
	if err != nil {
		slog.Error("[Init] Failed to load datastore", "error", err)
		return err
	}
	// migrate data
	vm := sqlite.NewSQLiteVersionManager(ds)
	err = sqlite.MigrateToLatest(vm, ds)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		return err
	}

	datastore.SetDefaultDatastore(ds)

	// Initialize control panel
	jds := jsonds.NewJSONDatastore(jsondsTemplate.JsonDataStoreFS)

	err = jds.Init()
	if err != nil {
		fmt.Printf("Failed to initialize json file store: %v\n", err)
	}
	datastore.SetDefaultJsonDatastore(jds)

	// Initialize core core app server
	// 注意：logger已在命令初始化时根据参数设置完成，此处不再重复初始化
	aogServer := api.NewAOGCoreServer()
	aogServer.Register()

	logger.LogicLogger.Info("start_app")

	// load all flavors
	// this loads all config based API Flavors. You need to manually
	// create and RegisterAPIFlavor for costimized API Flavors
	err = schedule.InitAPIFlavors()
	if err != nil {
		slog.Error("[Init] Failed to load API Flavors", "error", err)
		return nil
	}

	// start
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

	// Inject all flavors to the router
	// Setup flavors
	for _, flavor := range schedule.AllAPIFlavors() {
		flavor.InstallRoutes(aogServer.Router)
		schedule.InitProviderDefaultModelTemplate(flavor)
	}

	pidFile := filepath.Join(config.GlobalEnvironment.RootDir, constants.AppName+".pid")
	err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
	if err != nil {
		slog.Error("[Run] Failed to write pid file", "error", err)
		return err
	}

	_ = console.RegisterConsoleRoutes(aogServer.Router)

	go ListenModelEngineHealth()

	// Run the server
	err = aogServer.Run(ctx, config.GlobalEnvironment.ApiHost)
	if err != nil {
		slog.Error("[Run] Failed to run server", "error", err)
		return err
	}

	_, _ = color.New(color.FgHiGreen).Println("AOG Gateway starting on port", config.GlobalEnvironment.ApiHost)

	return nil
}
