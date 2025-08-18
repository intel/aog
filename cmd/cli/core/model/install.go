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

// NewInstallModelCommand creates the install model command
func NewInstallModelCommand() *cobra.Command {
	var (
		serviceName  string
		providerName string
		remote       bool
	)

	pullModelCmd := &cobra.Command{
		Use:    "pull <model_name>",
		Short:  "Pull a model for a specific service",
		Long:   `Pull a model for a specific service with optional remote flag.`,
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			PullHandler(cmd, args)
		},
	}

	pullModelCmd.Flags().StringVarP(&serviceName, "for", "f", "", "Name of the service to pull the model for, e.g: chat/embed (required)")
	pullModelCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Name of the service provider to pull the model for, e.g: local_ollama_chat (required)")
	pullModelCmd.Flags().BoolVarP(&remote, "remote", "r", false, "Pull the model from a remote source (default: false)")

	if err := pullModelCmd.MarkFlagRequired("for"); err != nil {
		slog.Error("Error: --for is required")
	}

	return pullModelCmd
}

// PullHandler handles model pulling
func PullHandler(cmd *cobra.Command, args []string) {
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

	req := dto.CreateModelRequest{}
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

	err = common.DoHTTPRequest(c, http.MethodPost, routerPath, req, &resp)
	if err != nil {
		fmt.Printf("\rPull model failed: %s", err.Error())
		return
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rPull model  failed: %s", resp.Message)
		return
	}

	fmt.Println("You can use the command `aog get models` to check if the model is downloaded successfully.")
	fmt.Println("Model is downloading in the background, please wait...")
}
