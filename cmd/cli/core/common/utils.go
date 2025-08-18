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

package common

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/progress"
	"github.com/spf13/cobra"
)

// CheckAOGServer checks if AOG server is running
func CheckAOGServer(cmd *cobra.Command, args []string) {
	if !utils.IsServerRunning() {
		fmt.Println("AOG server is not running, Please run 'aog server start' first")
		os.Exit(1)
		return
	}
}

// StartAOGServer starts AOG server if not running
func StartAOGServer(cmd *cobra.Command, args []string) {
	if utils.IsServerRunning() {
		return
	}

	fmt.Println("AOG server is not running. Starting the server...")
	if err := startAogServer(); err != nil {
		log.Fatalf("Failed to start AOG server: %s \n", err.Error())
		return
	}

	time.Sleep(6 * time.Second)

	if !utils.IsServerRunning() {
		log.Fatal("Failed to start AOG server.")
		return
	}

	fmt.Println("AOG server start successfully.")
}

// startAogServer starts the AOG server
func startAogServer() error {
	logPath := config.GlobalEnvironment.ConsoleLog
	rootDir := config.GlobalEnvironment.RootDir
	err := utils.StartAOGServer(logPath, rootDir)
	if err != nil {
		fmt.Printf("AOG server start failed: %s", err.Error())
		return err
	}
	return nil
}

// ShowProgressWithMessage shows loading animation with custom message
func ShowProgressWithMessage(message string, fn func() error) error {
	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	wg.Add(1)
	go progress.ShowLoadingAnimation(stopChan, &wg, message)

	err := fn()

	close(stopChan)
	wg.Wait()

	return err
}

// GetAPIKey gets the API Key entered by the user
func GetAPIKey() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Please enter your applied API Key: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return ""
	}
	return strings.TrimSpace(input)
}

// AskEnableRemoteService asks the user whether to enable remote services
func AskEnableRemoteService() bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Do you want to enable remote collaborative DeepSeek service? (y/n): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" {
			return true
		} else if input == "n" {
			return false
		} else {
			fmt.Println("Invalid input, please enter 'y' or 'n'.")
		}
	}
}

// NewAOGClient creates a new AOG client with context
func NewAOGClient() *config.AOGClient {
	return config.NewAOGClient()
}

// DoHTTPRequest performs HTTP request with proper error handling
func DoHTTPRequest(client *config.AOGClient, method, path string, req, resp interface{}) error {
	return client.Client.Do(context.Background(), method, path, req, resp)
}
