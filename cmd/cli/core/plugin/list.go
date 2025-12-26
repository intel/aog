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
	"os"
	"text/tabwriter"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewListPluginsCommand creates a command to list all installed plugins
func NewListPluginsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "list",
		Short:  "List all installed plugins",
		Long:   "List all installed AOG plugins with their details.",
		Args:   cobra.ExactArgs(0),
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listPlugins()
		},
	}

	return cmd
}

func listPlugins() error {
	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/plugin/list", constants.AppName, version.SpecVersion)

	req := struct{}{}
	resp := dto.GetPluginListResponse{}

	if err := c.Client.Do(context.Background(), http.MethodGet, routerPath, req, &resp); err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(resp.Data) == 0 {
		fmt.Println("No plugins installed.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSTATUS\tSERVICES")

	for _, p := range resp.Data {
		status := "UnRegister"
		if p.Status == constants.PluginStatusRunning {
			status = "Running"
		} else if p.Status == constants.PluginStatStopped {
			status = "Stopped"
		}

		services := ""
		for i, s := range p.Services {
			if i > 0 {
				services += ", "
			}
			services += s
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			p.Name,
			p.Version,
			status,
			services,
		)
	}

	w.Flush()
	return nil
}
