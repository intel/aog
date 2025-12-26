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
	"strings"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewDeletePluginCommand creates a command to Delete a plugin
func NewDeletePluginCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:    "delete <plugin-name> [plugin-name ...]",
		Short:  "delete one or more plugins",
		Long:   "delete one or more AOG plugins by name.",
		Args:   cobra.MinimumNArgs(1),
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			return deletePlugins(args, force)
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force uninstall without confirmation")

	return cmd
}

func deletePlugins(pluginNames []string, force bool) error {
	toDelete := append([]string{}, pluginNames...)

	if !force && len(toDelete) > 0 {
		fmt.Printf("The following plugins will be Deleteed:\n  %s\n", strings.Join(toDelete, "\n  "))
		if !confirm("Are you sure you want to continue? (y/N): ") {
			return nil
		}
	}

	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/plugin/delete", constants.AppName, version.SpecVersion)

	for _, name := range toDelete {
		req := dto.PluginDeleteRequest{Name: name}
		resp := dto.PluginDeleteResponse{}

		if err := c.Client.Do(context.Background(), http.MethodDelete, routerPath, req, &resp); err != nil {
			return fmt.Errorf("failed to Delete plugin %s: %w", name, err)
		}
		fmt.Printf("Successfully Delete plugin: %s\n", name)
	}

	return nil
}

// confirm prompts for user confirmation
func confirm(prompt string) bool {
	var response string
	fmt.Print(prompt)
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}
