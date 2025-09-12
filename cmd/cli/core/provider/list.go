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
	"github.com/intel/aog/internal/constants"
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
		Use:   "service_providers",
		Short: "List service providers",
		Long: `Display information about all service providers including their status and configuration.
		
Examples:
  # List all providers
  aog get service_providers

  # List providers for chat service
  aog get service_providers --service chat

  # List only remote providers
  aog get service_providers --remote remote`,
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

			fmt.Printf("%-20s %-12s %-10s %-10s %-10s %-10s %-25s\n", "PROVIDER", "SERVICE", "SOURCE", "FLAVOR", "AUTH", "STATUS", "UPDATED") // Table header

			for _, p := range resp.Data {
				providerStatus := constants.ProviderStatusHealthy
				if p.Status == 0 {
					providerStatus = constants.ProviderStatusUnhealthy
				}

				fmt.Printf("%-20s %-12s %-10s %-10s %-10s %-10s %-25s\n",
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

	listProviderCmd.Flags().StringVarP(&serviceName, "service", "s", "", "Filter by service name: chat, embed, or generate")
	listProviderCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Filter by specific provider name")
	listProviderCmd.Flags().StringVarP(&remote, "remote", "r", "", "Filter by source type: local or remote")

	return listProviderCmd
}
