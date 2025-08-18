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
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewInstallServiceCommand creates the install service command
func NewInstallServiceCommand() *cobra.Command {
	var (
		providerName  string
		remoteFlag    bool
		remoteURL     string
		authType      string
		method        string
		authKey       string
		flavor        string
		skipModelFlag bool
		model         string
	)

	installServiceCmd := &cobra.Command{
		Use:    "install <service>",
		Short:  "Install a service or service provider",
		Long:   `Install a service by name or a service provider from a file.`,
		Args:   cobra.ExactArgs(1),
		PreRun: common.StartAOGServer,
		Run: func(cmd *cobra.Command, args []string) {
			InstallServiceHandler(cmd, args)
		},
	}

	installServiceCmd.Flags().BoolVarP(&remoteFlag, "remote", "r", false, "Enable remote connect")
	installServiceCmd.Flags().StringVar(&providerName, "name", "", "Give the service an alias")
	installServiceCmd.Flags().StringVar(&remoteURL, "remote_url", "", "Remote URL for connect")
	installServiceCmd.Flags().StringVar(&authType, "auth_type", "none", "Authentication type (apikey/token/none)")
	installServiceCmd.Flags().StringVar(&method, "method", "POST", "HTTP method (default POST)")
	installServiceCmd.Flags().StringVar(&authKey, "auth_key", "", "Authentication key json format")
	installServiceCmd.Flags().StringVar(&flavor, "flavor", "", "Flavor (tencent/deepseek)")
	installServiceCmd.Flags().StringP("file", "f", "", "Path to the service provider file (required for service_provider)")
	installServiceCmd.Flags().BoolVarP(&skipModelFlag, "skip_model", "", false, "Skip the model download")
	installServiceCmd.Flags().StringVarP(&model, "model_name", "m", "", "Pull model name")

	return installServiceCmd
}

// InstallServiceHandler handles service installation
func InstallServiceHandler(cmd *cobra.Command, args []string) {
	serviceName := args[0]

	if serviceName == "service_provider" {
		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			fmt.Println("Error: failed to get file path")
			return
		}
		if filePath == "" {
			fmt.Println("Error: file path is required for service_provider")
			return
		}
		err = installServiceProviderHandler(filePath)
		if err != nil {
			fmt.Println("Error: service provider install failed", err.Error())
			return
		}
	} else {
		installRegularService(cmd, serviceName)
	}
}

// installRegularService installs a regular AI service
func installRegularService(cmd *cobra.Command, serviceName string) {
	remote, err := cmd.Flags().GetBool("remote")
	if err != nil {
		fmt.Println("Error: failed to get remote flag")
		return
	}
	providerName, err := cmd.Flags().GetString("name")
	if err != nil {
		fmt.Println("Error: failed to get provider name")
		return
	}

	if err := common.ValidateServiceName(serviceName); err != nil {
		fmt.Printf("\r%s", err.Error())
		return
	}

	req := dto.CreateAIGCServiceRequest{}
	resp := bcode.Bcode{}

	if remote {
		if err := setupRemoteService(cmd, &req); err != nil {
			fmt.Printf("\rError: %s", err.Error())
			return
		}
	} else {
		setupLocalService(&req, serviceName)
	}

	skipModelFlag, err := cmd.Flags().GetBool("skip_model")
	if err != nil {
		skipModelFlag = false
	}
	modelName, err := cmd.Flags().GetString("model_name")
	if err != nil {
		modelName = ""
	}
	req.SkipModelFlag = skipModelFlag
	req.ModelName = modelName
	req.ServiceName = serviceName
	req.ProviderName = providerName
	if req.ProviderName == "" {
		req.ProviderName = fmt.Sprintf("%s_%s_%s", req.ServiceSource, req.ApiFlavor, req.ServiceName)
	}

	err = common.ShowProgressWithMessage("Service installing", func() error {
		c := common.NewAOGClient()
		routerPath := fmt.Sprintf("/aog/%s/service/install", version.SpecVersion)
		return common.DoHTTPRequest(c, http.MethodPost, routerPath, req, &resp)
	})
	if err != nil {
		fmt.Printf("\rService install failed: %s", err.Error())
		return
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rService install failed: %s", resp.Message)
		return
	}

	fmt.Printf("Service %s install success!", serviceName)
	fmt.Println("Model is downloading in the background, please wait...")
	fmt.Println("You can use the command `aog get models` to check if the model is downloaded successfully.")

	if !remote && serviceName == types.ServiceChat {
		handleChatServicePostInstall()
	}
}

// setupRemoteService configures remote service parameters
func setupRemoteService(cmd *cobra.Command, req *dto.CreateAIGCServiceRequest) error {
	method, err := cmd.Flags().GetString("method")
	if err != nil {
		return fmt.Errorf("failed to get method")
	}
	authKey, err := cmd.Flags().GetString("auth_key")
	if err != nil {
		return fmt.Errorf("failed to get auth_key")
	}
	flavorName, err := cmd.Flags().GetString("flavor")
	if err != nil {
		return fmt.Errorf("failed to get flavor")
	}

	if authKey == "" {
		return fmt.Errorf("auth_key is required when auth_type is not none")
	}
	if err := common.ValidateFlavor(flavorName); err != nil {
		return err
	}

	providerInfo := schedule.GetProviderServiceDefaultInfo(flavorName, req.ServiceName)
	req.ServiceSource = types.ServiceSourceRemote
	req.ApiFlavor = flavorName
	req.Url = providerInfo.RequestUrl
	req.AuthType = providerInfo.AuthType
	req.AuthKey = authKey
	req.Method = method

	return nil
}

// setupLocalService configures local service parameters
func setupLocalService(req *dto.CreateAIGCServiceRequest, serviceName string) {
	req.ServiceSource = types.ServiceSourceLocal
	req.ApiFlavor = types.FlavorOllama
	if serviceName == types.ServiceTextToImage || serviceName == types.ServiceSpeechToText ||
		serviceName == types.ServiceTextToSpeech || serviceName == types.ServiceSpeechToTextWS {
		req.ApiFlavor = types.FlavorOpenvino
	}
	req.AuthType = types.AuthTypeNone
}

// handleChatServicePostInstall handles post-installation for chat service
func handleChatServicePostInstall() {
	askRes := common.AskEnableRemoteService()
	if askRes {
		fmt.Println("Please visit https://platform.deepseek.com/ to apply for an API KEY.")
		apiKey := common.GetAPIKey()
		if apiKey != "" {
			fmt.Printf("\rYour entered API Key is: %s\n", apiKey)
		}

		req := &dto.CreateAIGCServiceRequest{
			ServiceName:   "chat",
			ServiceSource: "remote",
			ApiFlavor:     "deepseek",
			ProviderName:  "remote_deepseek_chat",
			Desc:          "remote deepseek model service",
			Method:        http.MethodPost,
			Url:           "https://api.lkeap.cloud.tencent.com/v1/chat/completions",
			AuthType:      "apikey",
			AuthKey:       apiKey,
			ExtraHeaders:  "{}",
			ExtraJsonBody: "{}",
			Properties:    `{"max_input_tokens": 131072,"supported_response_mode":["stream","sync"]}`,
		}

		resp := bcode.Bcode{}
		c := common.NewAOGClient()
		routerPath := fmt.Sprintf("/aog/%s/service/install", version.SpecVersion)

		err := common.DoHTTPRequest(c, http.MethodPost, routerPath, req, &resp)
		if err != nil {
			fmt.Printf("\rService install failed: %s ", err.Error())
			return
		}
	} else {
		fmt.Println("You can enable remote DeepSeek service next time by running: aog install chat -r --flavor deepseek --auth_type apikey")
	}
}

// installServiceProviderHandler handles service provider installation from file
func installServiceProviderHandler(configFile string) error {
	if err := common.ValidateFileExists(configFile); err != nil {
		return err
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	var spConf dto.CreateServiceProviderRequest
	err = json.Unmarshal(data, &spConf)
	if err != nil {
		return fmt.Errorf("please check your json format: %w", err)
	}

	if err := common.ValidateServiceProvider(spConf.ServiceName, spConf.ServiceSource, spConf.ApiFlavor, spConf.AuthType, spConf.AuthKey); err != nil {
		return err
	}

	resp := dto.CreateServiceProviderResponse{}

	err = common.ShowProgressWithMessage("Service provider installing", func() error {
		c := common.NewAOGClient()
		routerPath := fmt.Sprintf("/aog/%s/service_provider", version.SpecVersion)
		return common.DoHTTPRequest(c, http.MethodPost, routerPath, spConf, &resp)
	})
	if err != nil {
		fmt.Printf("\rService provider install failed: %s", err.Error())
		return err
	}

	if resp.HTTPCode > 200 {
		fmt.Printf("\rService provider install failed: %s", resp.Message)
		return fmt.Errorf(resp.Message)
	}

	fmt.Println("Service provider install success!")
	return nil
}
