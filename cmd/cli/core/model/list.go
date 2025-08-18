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

package model

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

// NewListModelsCommand creates the list models command
func NewListModelsCommand() *cobra.Command {
	var providerName string

	listModelCmd := &cobra.Command{
		Use:    "models",
		Short:  "List models for a specific service",
		Long:   `List models for a specific service.`,
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			req := dto.GetModelsRequest{}
			resp := dto.GetModelsResponse{}

			if providerName != "" {
				req.ProviderName = providerName
			}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/aog/%s/model", version.SpecVersion)

			err := c.Client.Do(context.Background(), http.MethodGet, routerPath, req, &resp)
			if err != nil {
				fmt.Printf("\rGet model list failed: %s", err.Error())
				return
			}

			fmt.Printf("%-30s %-25s %-10s %-25s\n", "MODEL NAME", "PROVIDER NAME", "STATUS", "CREATE AT") // Table header

			for _, model := range resp.Data {
				fmt.Printf("%-30s %-20s %-15s %-25s\n",
					model.ModelName,
					model.ProviderName,
					model.Status,
					model.CreatedAt.Format(time.RFC3339),
				)
			}
		},
	}

	listModelCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the service provider, e.g: local_ollama_chat")

	return listModelCmd
}
