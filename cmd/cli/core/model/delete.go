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
	"fmt"
	"log/slog"
	"net/http"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewDeleteModelCommand creates the delete model command
func NewDeleteModelCommand() *cobra.Command {
	var (
		serviceName  string
		providerName string
		remote       bool
	)

	deleteModelCmd := &cobra.Command{
		Use:    "model <model_name>",
		Short:  "Remove a model for a specific service",
		Long:   `Remove a model for a specific service with optional remote flag.`,
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			DeleteModelHandler(cmd, args)
		},
	}

	deleteModelCmd.Flags().StringVarP(&serviceName, "for", "f", "", "Name of the service to delete the model for, e.g: chat/embed (required)")
	deleteModelCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the service provider to remove the model for, e.g: local_ollama_chat (required)") // -p is more common
	deleteModelCmd.Flags().BoolVarP(&remote, "remote", "r", false, "delete the model from a remote source (default: false)")

	if err := deleteModelCmd.MarkFlagRequired("provider"); err != nil {
		slog.Error("Error: --provider is required")
	}

	return deleteModelCmd
}

// DeleteModelHandler handles model deletion
func DeleteModelHandler(cmd *cobra.Command, args []string) {
	remote, err := cmd.Flags().GetBool("remote")
	if err != nil {
		fmt.Println("Error: failed to get remote flag")
		return
	}
	serviceName, err := cmd.Flags().GetString("for")
	if err != nil {
		fmt.Println("Error: failed to get service name")
		return
	}
	providerName, err := cmd.Flags().GetString("provider")
	if err != nil {
		fmt.Println("Error: failed to get provider name")
		return
	}
	modelName := args[0]

	req := dto.DeleteModelRequest{}
	resp := bcode.Bcode{}

	req.ModelName = modelName
	req.ServiceSource = types.ServiceSourceLocal
	if remote {
		req.ServiceSource = types.ServiceSourceRemote
	}
	req.ServiceName = serviceName
	req.ProviderName = providerName

	c := common.NewAOGClient()
	routerPath := fmt.Sprintf("/aog/%s/model", version.SpecVersion)

	err = common.DoHTTPRequest(c, http.MethodDelete, routerPath, req, &resp)
	if err != nil {
		fmt.Printf("\rDelete model failed: %s", err.Error())
		return
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rDelete model  failed: %s", resp.Message)
		return
	}

	fmt.Println("Delete model success!")
}
