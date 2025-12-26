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

package plugin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewRegisterPluginCommand creates a command to Load an existing plugin on server
func NewLoadPluginCommand() *cobra.Command {
	var pluginName string

	cmd := &cobra.Command{
		Use:    "load <plugin-name>",
		Short:  "load an existing plugin",
		Long:   "load an existing AOG plugin on the running server.",
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			pluginName = args[0]
			return loadPlugin(pluginName)
		},
	}

	return cmd
}

func loadPlugin(pluginName string) error {
	if pluginName == "" {
		return fmt.Errorf("plugin name is required")
	}

	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/plugin/load", constants.AppName, version.SpecVersion)

	req := dto.PluginLoadRequest{Name: pluginName}
	resp := dto.PluginLoadResponse{}

	if err := c.Client.Do(context.Background(), http.MethodPost, routerPath, req, &resp); err != nil {
		return fmt.Errorf("failed to Load plugin %s: %w", pluginName, err)
	}

	fmt.Printf("Successfully Loaded plugin: %s\n", pluginName)
	return nil
}
