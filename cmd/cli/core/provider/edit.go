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
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewEditProviderCommand creates the edit provider command
func NewEditProviderCommand() *cobra.Command {
	var filePath string

	editProviderCmd := &cobra.Command{
		Use:    "provider <provider_name>",
		Short:  "Edit service data",
		Long:   "Edit service status and scheduler policy",
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				fmt.Println("Error: failed to get file path")
				return
			}
			if filePath == "" {
				fmt.Println("Error: file path is required for service_provider")
				return
			}
			err = updateServiceProviderHandler(args[0], filePath)
			if err != nil {
				fmt.Println("Error: service provider install failed ", err.Error())
				return
			}
		},
	}

	editProviderCmd.Flags().StringVarP(&filePath, "file", "f", "", "service provider config file path")

	return editProviderCmd
}

// updateServiceProviderHandler handles service provider updates
func updateServiceProviderHandler(providerName, configFile string) error {
	if err := common.ValidateFileExists(configFile); err != nil {
		return err
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	var spConf dto.UpdateServiceProviderRequest
	err = json.Unmarshal(data, &spConf)
	if err != nil {
		return fmt.Errorf("failed to parse configuration file: %w", err)
	}

	if err := common.ValidateServiceProvider(spConf.ServiceName, spConf.ServiceSource, spConf.ApiFlavor, spConf.AuthType, spConf.AuthKey); err != nil {
		return err
	}

	if spConf.ProviderName != providerName {
		return fmt.Errorf("please check whether the provider name is the same as the provider name in the file")
	}

	resp := dto.UpdateServiceProviderResponse{}

	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/service_provider", "aog", version.SpecVersion)

	err = c.Client.Do(context.Background(), http.MethodPut, routerPath, spConf, &resp)
	if err != nil {
		fmt.Printf("\rService provider edit failed: %s", err.Error())
		return err
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rService provider edit failed: %s", resp.Message)
		return fmt.Errorf(resp.Message)
	}

	fmt.Println("Service provider edit success!")

	return nil
}
