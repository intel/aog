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
	"time"

	"github.com/intel/aog/cmd/cli/core/common"
	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// ensureServerRunning starts AOG server if not running
func ensureServerRunning(cmd *cobra.Command, args []string) {
	manager, err := process.GetAOGProcessManager()
	if err != nil {
		fmt.Printf("Failed to initialize AOG process manager: %v\n", err)
		os.Exit(1)
	}

	if manager.IsProcessRunning() {
		return
	}

	fmt.Println("Starting AOG server...")
	if err := manager.StartProcessDaemon(); err != nil {
		fmt.Printf("Failed to start AOG server: %v\n", err)
		os.Exit(1)
	}

	if err := manager.WaitForReady(10 * time.Second); err != nil {
		fmt.Printf("Server failed to become ready: %v\n", err)
		os.Exit(1)
	}
}

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
		Use:   "install <service>",
		Short: "Install and configure AI services",
		Long: `Install and configure AI services such as chat, embed, or generate.
		
Examples:
  # Install local chat service with default model
  aog install chat

  # Install remote chat service with DeepSeek API
  aog install chat --remote --flavor deepseek --auth_type apikey

  # Install local embedding service
  aog install embed

For more information about supported services and providers, visit: https://intel.github.io/aog/`,
		Args:   cobra.ExactArgs(1),
		PreRun: ensureServerRunning,
		Run:    InstallServiceHandler,
	}

	installServiceCmd.Flags().BoolVarP(&remoteFlag, "remote", "r", false, "Install remote AI service instead of local")
	installServiceCmd.Flags().StringVar(&providerName, "name", "", "Custom name for the service provider")
	installServiceCmd.Flags().StringVar(&remoteURL, "remote_url", "", "Remote API endpoint URL")
	installServiceCmd.Flags().StringVar(&authType, "auth_type", "none", "Authentication method: apikey, token, or none")
	installServiceCmd.Flags().StringVar(&method, "method", "POST", "HTTP method for API requests")
	installServiceCmd.Flags().StringVar(&authKey, "auth_key", "", "Authentication credentials in JSON format")
	installServiceCmd.Flags().StringVar(&flavor, "flavor", "", "API provider: ollama, deepseek, openai, tencent")
	installServiceCmd.Flags().BoolVar(&skipModelFlag, "skip_model", false, "Skip automatic model download during installation")
	installServiceCmd.Flags().StringVar(&model, "model", "", "Specific model name to install")
	installServiceCmd.Flags().StringP("file", "f", "", "Path to the service provider file (required for service_provider)")

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
		fmt.Println("❌ Error: failed to get remote flag")
		return
	}
	providerName, err := cmd.Flags().GetString("name")
	if err != nil {
		fmt.Println("❌ Error: failed to get provider name")
		return
	}

	if err := common.ValidateServiceName(serviceName); err != nil {
		fmt.Printf("❌ %s\n", err.Error())
		fmt.Println("💡 Supported services: chat, embed, generate")
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
		if err := setupLocalService(&req, serviceName); err != nil {
			fmt.Printf("❌ Error: %s\n", err.Error())
			return
		}
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

	fmt.Printf("✅ Service %s installed successfully!\n", serviceName)
	fmt.Println()
	fmt.Println("⏳ Model is downloading in the background, please wait...")
	fmt.Println("📊 Check download progress:")
	fmt.Println("   aog get models")
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
func setupLocalService(req *dto.CreateAIGCServiceRequest, serviceName string) error {
	req.ServiceSource = types.ServiceSourceLocal
	req.ApiFlavor = types.FlavorOllama

	if serviceName == types.ServiceImageToVideo || serviceName == types.ServiceImageToImage {
		return fmt.Errorf("local %s service is not supported yet, please use remote services instead (e.g., aliyun)", serviceName)
	}

	if serviceName == types.ServiceTextToImage || serviceName == types.ServiceSpeechToText ||
		serviceName == types.ServiceTextToSpeech || serviceName == types.ServiceSpeechToTextWS {
		req.ApiFlavor = types.FlavorOpenvino
	}
	req.AuthType = types.AuthTypeNone

	return nil
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

	fmt.Println("✅ Service provider installed successfully!")
	return nil
}
