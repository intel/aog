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
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewInfoPluginCommand creates a command to show detailed information about a plugin
func NewInfoPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "info <plugin-name>",
		Short:  "Show detailed information about a plugin",
		Long:   "Show detailed information about an installed AOG plugin.",
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showPluginInfo(args[0])
		},
	}

	return cmd
}

func showPluginInfo(pluginName string) error {
	c := config.NewAOGClient()
	routerPath := fmt.Sprintf("/%s/%s/plugin/info", constants.AppName, version.SpecVersion)

	req := dto.GetPluginInfoRequest{Name: pluginName}
	resp := dto.GetPluginInfoResponse{}

	if err := c.Client.Do(context.Background(), "GET", routerPath, req, &resp); err != nil {
		return fmt.Errorf("failed to get plugin info: %w", err)
	}

	info := resp.Data

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	bold := color.New(color.Bold).SprintFunc()

	fmt.Fprintf(w, "%s\t%s\n", bold("Name:"), info.Name)
	fmt.Fprintf(w, "%s\t%s\n", bold("Provider:"), info.ProviderName)
	fmt.Fprintf(w, "%s\t%s\n", bold("Version:"), info.Version)
	fmt.Fprintf(w, "%s\t%s\n", bold("Status:"), getStatusString(info.Status))
	fmt.Fprintf(w, "%s\t%s\n", bold("Description:"), info.Description)

	fmt.Fprintln(w)
	fmt.Fprintln(w, bold("Services:"))
	if len(info.Services) == 0 {
		fmt.Fprintln(w, "  No services defined")
	} else {
		for _, svc := range info.Services {
			fmt.Fprintf(w, "  %s\n", svc)
		}
	}

	w.Flush()
	return nil
}

func getStatusString(status int) string {
	switch status {
	case 0:
		return "Stopped"
	case 1:
		return "Running"
	case 2:
		return "Error"
	default:
		return "Unknown"
	}
}
