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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewImportServiceCommand creates the import service command
func NewImportServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "import <file_path>",
		Short:  "Import service configuration from a file",
		Long:   "Import service configuration from a file and send it to the API.",
		PreRun: common.CheckAOGServer,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("please provide a .aog file path")
			}
			filePath := args[0]
			// Read the file content
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			// Parse the file content into ImportServiceRequest
			var req dto.ImportServiceRequest
			var resp dto.ImportServiceResponse

			err = json.Unmarshal(data, &req)
			if err != nil {
				return fmt.Errorf("please check your json format: %w", err)
			}

			err = common.ShowProgressWithMessage("Importing service configuration", func() error {
				c := config.NewAOGClient()
				routerPath := fmt.Sprintf("/aog/%s/service/import", version.SpecVersion)
				return c.Client.Do(context.Background(), http.MethodPost, routerPath, req, &resp)
			})
			if err != nil {
				fmt.Printf("\r %s", err.Error())
				return err
			}

			fmt.Println("\rImport service configuration succeeded")
			return nil
		},
	}
	return cmd
}

// NewExportServiceCommand creates the export service command
func NewExportServiceCommand() *cobra.Command {
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export service",
		Long:  "Export service",
	}

	// 添加子命令
	exportCmd.AddCommand(NewExportServiceToFileCommand())
	exportCmd.AddCommand(NewExportServiceToStdoutCommand())

	return exportCmd
}

// NewExportServiceToFileCommand creates the export to file command
func NewExportServiceToFileCommand() *cobra.Command {
	var filePath, service, provider, model string

	cmd := &cobra.Command{
		Use:    "to-file",
		Short:  "Export service to file",
		Long:   "Export service to file",
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			req := &dto.ExportServiceRequest{
				ServiceName:  service,
				ProviderName: provider,
				ModelName:    model,
			}
			resp := &dto.ExportServiceResponse{}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/aog/%s/service/export", version.SpecVersion)

			err := c.Client.Do(context.Background(), http.MethodPost, routerPath, req, resp)
			if err != nil {
				fmt.Println("Error exporting service:", err)
				return
			}

			data, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				fmt.Println("Error marshaling JSON:", err)
				return
			}

			err = os.WriteFile(filePath, data, 0o600)
			if err != nil {
				fmt.Println("Error writing to file:", err)
				return
			}
			fmt.Println("Exported to file successfully.")
		},
	}

	// 在子命令上定义所有参数
	cmd.Flags().StringVarP(&filePath, "file", "f", "./.aog", "Output file path")
	cmd.Flags().StringVar(&service, "service", "", "Service name")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&model, "model", "", "Model name")

	return cmd
}

// NewExportServiceToStdoutCommand creates the export to stdout command
func NewExportServiceToStdoutCommand() *cobra.Command {
	var service, provider, model string

	cmd := &cobra.Command{
		Use:    "to-stdout",
		Short:  "Export service to stdout",
		Long:   "Export service to stdout",
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			req := &dto.ExportServiceRequest{
				ServiceName:  service,
				ProviderName: provider,
				ModelName:    model,
			}
			resp := &dto.ExportServiceResponse{}

			c := config.NewAOGClient()
			routerPath := fmt.Sprintf("/aog/%s/service/export", version.SpecVersion)

			err := c.Client.Do(context.Background(), http.MethodPost, routerPath, req, resp)
			if err != nil {
				fmt.Println("Error exporting service:", err)
				return
			}

			data, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				fmt.Println("Error marshaling JSON:", err)
				return
			}
			fmt.Println(string(data))
		},
	}

	// 在子命令上定义参数
	cmd.Flags().StringVar(&service, "service", "", "Service name")
	cmd.Flags().StringVar(&provider, "provider", "", "Provider name")
	cmd.Flags().StringVar(&model, "model", "", "Model name")

	return cmd
}
