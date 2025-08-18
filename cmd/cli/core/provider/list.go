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
	"fmt"
	"net/http"
	"time"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewListProvidersCommand creates the list providers command
func NewListProvidersCommand() *cobra.Command {
	var serviceName string
	var providerName string
	var remote string

	listProviderCmd := &cobra.Command{
		Use:    "service_providers",
		Short:  "List models for a specific service",
		Long:   `List models for a specific service.`,
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			req := dto.GetServiceProvidersRequest{}
			resp := dto.GetServiceProvidersResponse{}

			if serviceName != "" {
				req.ServiceName = serviceName
			}
			if providerName != "" {
				req.ProviderName = providerName
			}
			if remote != "" && (remote == types.ServiceSourceRemote || remote == types.ServiceSourceLocal) {
				req.ServiceSource = remote
			}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/aog/%s/service_provider", version.SpecVersion)

			err := c.Client.Do(context.Background(), http.MethodGet, routerPath, req, &resp)
			if err != nil {
				fmt.Printf("\rGet service provider list failed: %s", err.Error())
				return
			}

			fmt.Printf("%-20s %-10s %-10s %-10s %-10s %-15s %-25s\n", "PROVIDER NAME", "SERVICE NAME", "SERVICE SOURCE", "FLAVOR", "AUTH TYPE", "STATUS", "UPDATE AT") // Table header

			for _, p := range resp.Data {
				providerStatus := "healthy"
				if p.Status == 0 {
					providerStatus = "unhealthy"
				}

				fmt.Printf("%-20s %-10s %-10s %-10s %-10s %-15s %-25s\n",
					p.ProviderName,
					p.ServiceName,
					p.ServiceSource,
					p.Flavor,
					p.AuthType,
					providerStatus,
					p.UpdatedAt.Format(time.RFC3339),
				)
			}
		},
	}

	listProviderCmd.Flags().StringVarP(&serviceName, "service", "s", "", "Name of the service to list models for, e.g: chat/embed ")
	listProviderCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the service provider, e.g: local_ollama_chat")
	listProviderCmd.Flags().StringVarP(&remote, "remote", "r", "", "")

	return listProviderCmd
}
