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

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewDeleteProviderCommand creates the delete provider command
func NewDeleteProviderCommand() *cobra.Command {
	deleteProviderCmd := &cobra.Command{
		Use:    "service_provider <provider_name>",
		Short:  "Remove a provider for a specific service",
		Long:   `Remove a provider for a specific service with optional remote flag.`,
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			DeleteProviderHandler(cmd, args)
		},
	}

	return deleteProviderCmd
}

// DeleteProviderHandler handles provider deletion
func DeleteProviderHandler(cmd *cobra.Command, args []string) {
	providerName := args[0]

	req := dto.DeleteServiceProviderRequest{}
	resp := dto.DeleteServiceProviderResponse{}

	req.ProviderName = providerName

	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/aog/%s/service_provider", version.SpecVersion)

	err := c.Client.Do(context.Background(), http.MethodDelete, routerPath, req, &resp)
	if err != nil {
		fmt.Printf("\rDelete service provider failed: %s", err.Error())
		return
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rDelete service provider  failed: %s", resp.Message)
		return
	}

	fmt.Println("Delete service provider success!")
}
