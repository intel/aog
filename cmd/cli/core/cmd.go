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
	"github.com/intel/aog/cmd/cli/core/provider"
	"github.com/intel/aog/cmd/cli/core/server"
	"github.com/intel/aog/cmd/cli/core/service"
	"github.com/intel/aog/internal/constants"
	"github.com/spf13/cobra"
)

// NewCommand will contain all commands
func NewCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use: constants.AppName,
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
	)

	return cmds
}

// NewGetCommand creates the get command with subcommands
func NewGetCommand() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get resources",
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
		Short: "Edit resources",
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
		Short: "Delete resources",
	}
	deleteCmd.AddCommand(
		model.NewDeleteModelCommand(),
		provider.NewDeleteProviderCommand(),
	)

	return deleteCmd
}
