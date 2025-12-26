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

// NewStopPluginCommand creates a command to stop a running plugin
func NewStopPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "stop <plugin-name>",
		Short:  "Stop a running plugin",
		Long:   "Stop a running AOG plugin by name.",
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			return stopPlugin(args[0])
		},
	}

	return cmd
}

func stopPlugin(name string) error {
	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/plugin/stop", constants.AppName, version.SpecVersion)

	req := dto.PluginStopRequest{Name: name}
	resp := dto.PluginStopResponse{}

	if err := c.Client.Do(context.Background(), http.MethodPost, routerPath, req, &resp); err != nil {
		return fmt.Errorf("failed to stop plugin %s: %w", name, err)
	}

	fmt.Printf("Successfully stopped plugin: %s\n", name)
	return nil
}
