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
	"os"
	"strings"

	"github.com/intel/aog/internal/utils/progress"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/types"
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
		Use:   "pull <model_name>",
		Short: "Download and install AI models",
		Long: `Download and install AI models for specific services.
		
Examples:
  # Download llama3.2 model for chat service using ollama provider
  aog pull llama3.2 --for chat --provider local_ollama_chat

  # Download embedding model for embed service
  aog pull all-minilm --for embed --provider local_ollama_embed

  # Install remote model
  aog pull gpt-4 --for chat --provider remote_openai_chat --remote`,
		Args:   cobra.ExactArgs(1),
		PreRun: common.CheckAOGServer,
		Run:    PullHandler,
	}

	pullModelCmd.Flags().StringVarP(&serviceName, "for", "f", "", "Target service name (required): chat, embed, or generate")
	pullModelCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Service provider name (required), e.g., local_ollama_chat")
	pullModelCmd.Flags().BoolVarP(&remote, "remote", "r", false, "Download from remote source instead of local")

	if err := pullModelCmd.MarkFlagRequired("for"); err != nil {
		slog.Error("Error: --for is required")
	}

	return pullModelCmd
}

// PullHandler handles model pulling
func PullHandler(cmd *cobra.Command, args []string) {
	remote, err := cmd.Flags().GetBool("remote")
	if err != nil {
		fmt.Println("❌ Error: failed to get remote flag")
		return
	}
	serviceName, err := cmd.Flags().GetString("for")
	if err != nil {
		fmt.Println("❌ Error: failed to get service name")
		return
	}
	providerName, err := cmd.Flags().GetString("provider")
	if err != nil {
		fmt.Println("❌ Error: failed to get provider name")
		return
	}
	modelName := args[0]

	req := dto.CreateModelRequest{}

	req.ModelName = modelName
	req.ServiceSource = types.ServiceSourceLocal
	if remote {
		req.ServiceSource = types.ServiceSourceRemote
	}
	req.ServiceName = serviceName
	req.ProviderName = providerName

	c := common.NewAOGClient()
	routerPath := fmt.Sprintf("/aog/%s/model/stream", version.SpecVersion)

	//
	p := progress.NewProgress(os.Stdout)
	defer p.Stop()

	bars := make(map[string]*progress.Bar)

	var status string
	var spinner *progress.Spinner

	fn := func(resp types.ProgressResponse) error {
		if resp.Digest != "" {
			if resp.Completed == 0 {
				// This is the initial status update for the
				// layer, which the server sends before
				// beginning the download, for clients to
				// compute total size and prepare for
				// downloads, if needed.
				//
				// Skipping this here to avoid showing a 0%
				// progress bar, which *should* clue the user
				// into the fact that many things are being
				// downloaded and that the current active
				// download is not that last. However, in rare
				// cases it seems to be triggering to some, and
				// it isn't worth explaining, so just ignore
				// and regress to the old UI that keeps giving
				// you the "But wait, there is more!" after
				// each "100% done" bar, which is "better."
				return nil
			}

			if spinner != nil {
				spinner.Stop()
			}

			bar, ok := bars[resp.Digest]
			if !ok {
				name, isDigest := strings.CutPrefix(resp.Digest, "sha256:")
				name = strings.TrimSpace(name)
				if isDigest {
					name = name[:min(12, len(name))]
				}
				bar = progress.NewBar(fmt.Sprintf("pulling %s:", name), resp.Total, resp.Completed)
				bars[resp.Digest] = bar
				p.Add(resp.Digest, bar)
			}

			bar.Set(resp.Completed)
		} else if status != resp.Status {
			if spinner != nil {
				spinner.Stop()
			}

			status = resp.Status
			spinner = progress.NewSpinner(status)
			p.Add(status, spinner)
		}

		return nil
	}
	err = common.DoHTTPRequestStream(c, http.MethodPost, routerPath, req, fn)
	if err != nil {
		fmt.Printf("\rPull model failed: %s", err.Error())
		return
	}

	// fmt.Println("You can use the command `aog get models` to check if the model is downloaded successfully.")
	// fmt.Println("Model is downloading in the background, please wait...")
}
