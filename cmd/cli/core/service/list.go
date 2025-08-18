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
	"time"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewListServicesCommand creates the list services command
func NewListServicesCommand() *cobra.Command {
	listServiceCmd := &cobra.Command{
		Use:    "services <service_name>",
		Short:  "Display all available service information.",
		Long:   `Display all available service information.`,
		Args:   cobra.MaximumNArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			req := dto.GetAIGCServicesRequest{}
			resp := dto.GetAIGCServicesResponse{}

			if len(args) > 0 {
				req.ServiceName = args[0]
			}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/aog/%s/service", version.SpecVersion)

			err := c.Client.Do(context.Background(), http.MethodGet, routerPath, req, &resp)
			if err != nil {
				fmt.Printf("\rGet service list failed: %s", err.Error())
				return
			}

			fmt.Printf("%-10s %-15s %-15s %-15s %-5s %-15s %-15s\n", "SERVICE NAME", "HYBRID POLICY", "REMOTE PROVIDER", "LOCAL PROVIDER", "STATUS", "CREATE AT", "UPDATE AT") // Table header

			for _, service := range resp.Data {
				serviceStatus := "healthy"
				if service.Status == 0 {
					serviceStatus = "unhealthy"
				}

				fmt.Printf("%-10s %-15s %-15s %-15s %-5s %-15s %-15s\n",
					service.ServiceName,
					service.HybridPolicy,
					service.RemoteProvider,
					service.LocalProvider,
					serviceStatus,
					service.CreatedAt.Format(time.RFC3339),
					service.UpdatedAt.Format(time.RFC3339),
				)
			}
		},
	}

	return listServiceCmd
}
