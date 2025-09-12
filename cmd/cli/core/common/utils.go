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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/process"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils/progress"
	"github.com/spf13/cobra"
)

// CheckAOGServer checks if AOG server is running
func CheckAOGServer(cmd *cobra.Command, args []string) {
	if !IsServerRunning() {
		fmt.Println("⚠️  AOG server is not running. Please start it first:")
		fmt.Println("   aog server start")
		fmt.Println()
		fmt.Println("💡 Tip: Use 'aog server start -v' for verbose output during development")
		os.Exit(1)
	}
}

// IsServerRunning checks if the AOG server is running
func IsServerRunning() bool {
	// 优先使用HTTP健康检查，这样可以检测到任何方式启动的AOG服务器
	if isServerRunningHTTP() {
		return true
	}

	// 备用方案：检查进程管理器跟踪的进程
	manager, err := process.GetAOGProcessManager()
	if err != nil {
		return false
	}
	return manager.IsProcessRunning()
}

// isServerRunningHTTP performs HTTP health check fallback
func isServerRunningHTTP() bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:16688/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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

// NewAOGClient creates a new AOG client with context
func NewAOGClient() *config.AOGClient {
	return config.NewAOGClient()
}

// DoHTTPRequest performs HTTP request with proper error handling
func DoHTTPRequest(client *config.AOGClient, method, path string, req, resp interface{}) error {
	return client.Client.Do(context.Background(), method, path, req, resp)
}

func DoHTTPRequestStream(client *config.AOGClient, method, path string, req interface{}, fn types.PullProgressFunc) error {
	ctx := context.Background()
	reqHeader := make(map[string]string)
	reqHeader["Content-Type"] = "application/json"
	reqHeader["Accept"] = "application/json"
	dataCh, errCh := client.Client.StreamResponse(ctx, method, path, req, reqHeader)
	for {
		select {
		case data, ok := <-dataCh:
			if !ok {
				return nil
			}
			dataStr := string(data)
			if strings.HasPrefix(dataStr, "data:") {
				dataStr = dataStr[len("data:"):]
			}
			data = []byte(dataStr)
			var resp types.ProgressResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				logger.LogicLogger.Error(fmt.Sprintf("Error unmarshaling response: %v, %v", err, string(data)))
				continue
			}
			if resp.Status == "error" {
				return errors.New(dataStr)
			}

			err := fn(resp)
			if err != nil {
				return err
			}

		case err, _ := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
