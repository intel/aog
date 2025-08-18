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

package service

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewEditServiceCommand creates the edit service command
func NewEditServiceCommand() *cobra.Command {
	var hybridPolicy string
	var remoteProvider string
	var localProvider string

	updateServiceCmd := &cobra.Command{
		Use:    "service <service_name>",
		Short:  "Edit service data",
		Long:   "Edit service status and scheduler policy",
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			serviceName := args[0]
			hybridPolicy, err := cmd.Flags().GetString("hybrid_policy")
			remoteProvider, err := cmd.Flags().GetString("remote_provider")
			localProvider, err := cmd.Flags().GetString("local_provider")
			if err != nil {
				fmt.Println("An error occurred while obtaining the hybrid_policy parameter:", err)
				os.Exit(1)
			}

			if err := common.ValidateHybridPolicy(hybridPolicy); err != nil {
				fmt.Printf("\r%s\n", err.Error())
				os.Exit(1)
			}

			req := dto.UpdateAIGCServiceRequest{
				ServiceName:  serviceName,
				HybridPolicy: hybridPolicy,
			}
			resp := bcode.Bcode{}

			if remoteProvider != "" {
				req.RemoteProvider = remoteProvider
			}
			if localProvider != "" {
				req.LocalProvider = localProvider
			}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/%s/%s/service", "aog", version.SpecVersion)

			err = c.Client.Do(context.Background(), http.MethodPut, routerPath, req, &resp)
			if err != nil {
				return
			}
			if resp.HTTPCode > 200 {
				fmt.Println(resp.Message)
			}
			fmt.Printf("Service edit success!")
		},
	}

	updateServiceCmd.Flags().StringVar(&hybridPolicy, "hybrid_policy", "default", "only support default/always_local/always_remote.")
	updateServiceCmd.Flags().StringVarP(&remoteProvider, "remote_provider", "", "", "remote ai service provider")
	updateServiceCmd.Flags().StringVarP(&localProvider, "local_provider", "", "", "local ai service provider")

	return updateServiceCmd
}
