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
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/intel/aog/internal/datastore"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
)

// StartModelEngine starts a model engine
func StartModelEngine(engineName, mode string) error {
	if runtime.GOOS == "darwin" {
		return nil
	}
	// Check if the model engine service is started
	engineProvider := provider.GetModelEngine(engineName)
	engineConfig := engineProvider.GetConfig()

	err := engineProvider.HealthCheck()
	if err != nil {
		// cmd := exec.Command(engineConfig.ExecPath+engineConfig.ExecFile, "-h")
		// err := cmd.Run()
		// if err != nil {
		slog.Info("Check model engine " + engineName + " status")
		reCheckCmd := exec.Command(engineConfig.ExecPath+"/"+engineConfig.ExecFile, "-h")
		err = reCheckCmd.Run()
		_, isExistErr := os.Stat(engineConfig.ExecPath + "/" + engineConfig.ExecFile)
		if err != nil && isExistErr != nil {
			slog.Info("Model engine " + engineName + " status: not downloaded")
			return nil
		}
		//}

		slog.Info("Setting env...")
		err = engineProvider.InitEnv()
		if err != nil {
			slog.Error("Setting env error: ", err.Error())
			return err
		}

		slog.Info("Start " + engineName + "...")
		err = engineProvider.StartEngine(mode)
		if err != nil {
			slog.Error("Start engine "+engineName+" error: ", err.Error())
			return err
		}

		slog.Info("Waiting model engine " + engineName + " start 60s...")
		for i := 60; i > 0; i-- {
			time.Sleep(time.Second * 1)
			err = engineProvider.HealthCheck()
			if err == nil {
				slog.Info("Start " + engineName + " completed...")
				break
			}
			slog.Info("Waiting "+engineName+" start ...", strconv.Itoa(i), "s")
		}
	}

	err = engineProvider.HealthCheck()
	if err != nil {
		slog.Error(engineName + " failed start, Please try again later...")
		return err
	}

	slog.Info(engineName + " start successfully.")

	engineProvider.UpgradeEngine()

	return nil
}

// ListenModelEngineHealth monitors model engine health and keep alive
func ListenModelEngineHealth() {
	ds := datastore.GetDefaultDatastore()

	models := &types.Model{
		ServiceSource: types.ServiceSourceLocal,
	}

	for {
		list, err := ds.List(context.Background(), models, &datastore.ListOptions{Page: 0, PageSize: 100})
		if err != nil {
			logger.EngineLogger.Error("[Engine Listen]List models failed: ", err.Error())
			continue
		}

		if len(list) == 0 {
			continue
		}

		// get provider for models
		engineList := make([]string, 0)
		for _, item := range list {
			model := item.(*types.Model)
			sp := &types.ServiceProvider{
				ProviderName: model.ProviderName,
			}

			err := ds.Get(context.Background(), sp)
			if err != nil {
				logger.EngineLogger.Error("[Engine Listen]Get service provider failed: ", err.Error())
				continue
			}

			if utils.Contains(engineList, sp.Flavor) {
				continue
			}

			engineList = append(engineList, sp.Flavor)
		}

		for _, engine := range engineList {
			engineProvider := provider.GetModelEngine(engine)
			if engineProvider.GetOperateStatus != nil && engineProvider.GetOperateStatus() == 0 {
				// stop keeping alive if being used
				continue
			}
			err := engineProvider.HealthCheck()
			if err != nil {
				logger.EngineLogger.Error("[Engine Listen]"+engine+"engine health check failed: ", err.Error())
				err := engineProvider.InitEnv()
				if err != nil {
					logger.EngineLogger.Error("[Engine Listen]"+engine+" engine init env failed: ", err.Error())
					return
				}
				err = engineProvider.StartEngine(types.EngineStartModeDaemon)
				if err != nil {
					logger.EngineLogger.Error("[Engine Listen]"+engine+" engine start failed: ", err.Error())
					continue
				}
			}
		}

		time.Sleep(60 * time.Second)
	}
}
