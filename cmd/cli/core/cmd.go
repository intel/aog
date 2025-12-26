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

package cli

import (
	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/cmd/cli/core/model"
	"github.com/intel/aog/cmd/cli/core/plugin"
	"github.com/intel/aog/cmd/cli/core/provider"
	"github.com/intel/aog/cmd/cli/core/server"
	"github.com/intel/aog/cmd/cli/core/service"
	"github.com/intel/aog/internal/constants"
	"github.com/spf13/cobra"
)

// NewCommand creates the root AOG command with all subcommands
func NewCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   constants.AppName,
		Short: "AOG (AIPC Open Gateway) - AI service management platform",
		Long: `AOG (AIPC Open Gateway) provides unified AI services on AI PCs.

AOG decouples AI applications from AI service providers, offering:
- One-click AI service installation
- Automatic API adaptation for popular providers
- Hybrid scheduling between local and cloud services
- Shared AI services to reduce resource consumption

For detailed documentation, visit: https://intel.github.io/aog/

Common commands:
  aog server start          Start the AOG server
  aog install chat          Install chat service
  aog get services          List installed services
  aog get models            List installed models

Use 'aog <command> --help' for more information about a command.`,
	}

	cmds.AddCommand(
		// Server management
		server.NewApiserverCommand(),

		// Common commands
		common.NewVersionCommand(),

		// Resource management
		NewGetCommand(),
		NewEditCommand(),
		NewDeleteCommand(),

		// Service management
		service.NewInstallServiceCommand(),
		service.NewExportServiceCommand(),
		service.NewImportServiceCommand(),

		// Model management
		model.NewInstallModelCommand(),

		// Plugin management
		plugin.NewPluginCommand(),
	)

	return cmds
}

// NewGetCommand creates the get command with subcommands
func NewGetCommand() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Display resource information",
		Long:  "Display information about services, models, and providers installed in AOG.",
	}
	getCmd.AddCommand(
		service.NewListServicesCommand(),
		model.NewListModelsCommand(),
		provider.NewListProvidersCommand(),
	)

	return getCmd
}

// NewEditCommand creates the edit command with subcommands
func NewEditCommand() *cobra.Command {
	editCmd := &cobra.Command{
		Use:   "edit",
		Short: "Modify resource configurations",
		Long:  "Edit configurations for services and providers.",
	}
	editCmd.AddCommand(
		service.NewEditServiceCommand(),
		provider.NewEditProviderCommand(),
	)

	return editCmd
}

// NewDeleteCommand creates the delete command with subcommands
func NewDeleteCommand() *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Remove resources",
		Long:  "Remove models and providers from AOG. Use with caution as this operation cannot be undone.",
	}
	deleteCmd.AddCommand(
		model.NewDeleteModelCommand(),
		provider.NewDeleteProviderCommand(),
	)

	return deleteCmd
}
