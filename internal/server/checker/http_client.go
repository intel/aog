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

package checker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/schedule"
	"github.com/intel/aog/internal/types"
)

// ExecuteServerRequest performs the actual HTTP request and checks the response
func ExecuteServerRequest(req *http.Request, serviceProvider types.ServiceProvider, reqBodyString string) bool {
	// Handle special case for models service which doesn't need request body
	if req.Body == nil && serviceProvider.ServiceName == types.ServiceModels {
		req, _ = http.NewRequest(serviceProvider.Method, serviceProvider.URL, nil)
	}

	transport := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}

	// Set extra headers if provided
	if serviceProvider.ExtraHeaders != "{}" && serviceProvider.ExtraHeaders != "" {
		if err := setExtraHeaders(req, serviceProvider.ExtraHeaders); err != nil {
			logger.LogicLogger.Error("[ServiceChecker] Failed to set extra headers", "error", err)
			return false
		}
	}

	client := &http.Client{Transport: transport}
	req.Header.Set("Content-Type", "application/json")

	// Handle authentication
	if serviceProvider.AuthType != "none" {
		if err := handleAuthentication(req, serviceProvider, reqBodyString); err != nil {
			logger.LogicLogger.Error("[ServiceChecker] Authentication failed", "error", err)
			return false
		}
	}

	// Execute request
	logger.LogicLogger.Info("[ServiceChecker] Executing request", "url", req.URL.String(), "method", req.Method, "body", reqBodyString)
	resp, err := client.Do(req)
	if err != nil {
		logger.LogicLogger.Error("[ServiceChecker] Request failed", "error", err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	if resp.StatusCode != http.StatusOK {
		logger.LogicLogger.Error("[ServiceChecker] Request returned error status", "status", resp.StatusCode)
		return false
	}
	return true
}

// setExtraHeaders parses and sets extra headers from JSON string
func setExtraHeaders(req *http.Request, extraHeaders string) error {
	var headerMap map[string]interface{}
	logger.LogicLogger.Info("[ServiceChecker] Setting extra headers", "headers", extraHeaders)
	err := json.Unmarshal([]byte(extraHeaders), &headerMap)
	if err != nil {
		return fmt.Errorf("failed to parse extra headers JSON: %w", err)
	}

	for key, value := range headerMap {
		if strValue, ok := value.(string); ok {
			req.Header.Set(key, strValue)
		} else {
			logger.LogicLogger.Warn("[ServiceChecker] Skipping non-string header value", "key", key, "value", value)
		}
	}
	return nil
}

// handleAuthentication applies authentication to the request
func handleAuthentication(req *http.Request, serviceProvider types.ServiceProvider, reqBodyString string) error {
	authParams := &schedule.AuthenticatorParams{
		Request:      req,
		ProviderInfo: &serviceProvider,
		RequestBody:  reqBodyString,
	}

	authenticator := schedule.ChooseProviderAuthenticator(authParams)
	if authenticator == nil {
		return fmt.Errorf("no authenticator found for auth type: %s", serviceProvider.AuthType)
	}

	return authenticator.Authenticate()
}
