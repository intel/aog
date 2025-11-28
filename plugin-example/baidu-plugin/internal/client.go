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

package internal

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// BaiduClient wraps baidu API operations
type BaiduClient struct {
	config           *Config
	baseURL          *url.URL
	httpClient       *http.Client // Used for regular requests (with timeout)
	streamHTTPClient *http.Client // Used for long-running streaming requests (no timeout)
}

// NewBaiduClient creates a new baidu client
func NewBaiduClient(config *Config) (*BaiduClient, error) {
	baseURL, err := url.Parse(config.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	return &BaiduClient{
		config:  config,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout, // Regular request timeout
		},
		streamHTTPClient: &http.Client{
			Timeout: 0, // No timeout for streaming/long operations
		},
	}, nil
}

// Do executes an HTTP request
func (c *BaiduClient) Do(ctx context.Context, method, service, authInfo string, reqBody, respBody interface{}) error {
	var reqUrl string
	serviceConfig := c.getServiceConf(service)
	if serviceConfig.SpecialUrl != "" {
		reqUrl = serviceConfig.SpecialUrl
	} else {
		reqUrl = c.baseURL.String() + serviceConfig.Endpoint
	}
	log.Printf("[baidu-plugin] [DEBUG] HTTP Request: %s %s", method, reqUrl)

	var bodyReader io.Reader
	var jsonBody []byte
	var err error
	if reqBody != nil {
		if _, ok := reqBody.([]byte); !ok {
			jsonBody, err = json.Marshal(reqBody)
			if err != nil {
				log.Printf("[baidu-plugin] [ERROR] Failed to marshal request: %v", err)
				return fmt.Errorf("failed to marshal request: %w", err)
			}
		} else {
			jsonBody = reqBody.([]byte)
		}

		bodyReader = bytes.NewReader(jsonBody)

	}

	req, err := http.NewRequestWithContext(ctx, method, reqUrl, bodyReader)
	if err != nil {
		log.Printf("[baidu-plugin] [ERROR] Failed to create HTTP request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	err = c.setHeaders(req, serviceConfig.ExtraHeaders)
	if err != nil {
		log.Printf("[baidu-plugin] [ERROR] Failed to set headers: %v", err)
	}

	err = c.SetAuth(ctx, req, serviceConfig.AuthType, authInfo)
	if err != nil {
		log.Printf("[baidu-plugin] [ERROR] Failed to set auth: %v", err)
		return fmt.Errorf("failed to set auth: %w", err)
	}

	// Select appropriate HTTP client based on API path characteristics
	client := c.selectHTTPClient(serviceConfig.Endpoint)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[baidu-plugin] [ERROR] HTTP request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}

	log.Printf("[baidu-plugin] [DEBUG] HTTP Response: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		err = c.handleErrorResponse(resp)
		log.Printf("[baidu-plugin] [ERROR] HTTP error %d: %s", resp.StatusCode, err)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, err)
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			log.Printf("[baidu-plugin] [ERROR] Failed to decode response: %v", err)
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	log.Printf("[baidu-plugin] [DEBUG] HTTP request completed successfully")
	return nil
}

// selectHTTPClient chooses between timeout and non-timeout clients depending on the API path
func (c *BaiduClient) selectHTTPClient(path string) *http.Client {
	// AI inference / model download APIs use the no-timeout client
	longRunningAPIs := []string{}
	for _, service := range c.config.Services {
		longRunningAPIs = append(longRunningAPIs, service.Endpoint)
	}

	for _, api := range longRunningAPIs {
		if path == api {
			log.Printf("[baidu-plugin] [DEBUG] Using no-timeout client for: %s", path)
			return c.streamHTTPClient // No timeout
		}
	}

	// Other APIs (management, metadata) use the timeout-enabled client
	log.Printf("[baidu-plugin] [DEBUG] Using timeout client for: %s", path)
	return c.httpClient
}

// StreamResponse executes a streaming HTTP request
func (c *BaiduClient) StreamResponse(ctx context.Context, method, service, authInfo string, reqBody interface{}) (chan []byte, chan error) {
	var reqUrl string
	var serviceConfig YamlConfigService
	for _, s := range c.config.Services {
		if s.ServiceName == service {
			serviceConfig = s
			break
		}
	}
	if serviceConfig.SpecialUrl != "" {
		reqUrl = serviceConfig.SpecialUrl
	} else {
		reqUrl = c.baseURL.String() + serviceConfig.Endpoint
	}
	log.Printf("[baidu-plugin] [DEBUG] HTTP Streaming Request: %s %s", method, reqUrl)

	dataChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)
		defer log.Printf("[baidu-plugin] [DEBUG] HTTP streaming completed")

		var bodyReader io.Reader
		if reqBody != nil {
			jsonBody, err := json.Marshal(reqBody)
			if err != nil {
				log.Printf("[baidu-plugin] [ERROR] Failed to marshal streaming request: %v", err)
				errChan <- fmt.Errorf("failed to marshal request: %w", err)
				return
			}
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqUrl, bodyReader)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Failed to create streaming request: %v", err)
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		err = c.setHeaders(req, serviceConfig.ExtraHeaders)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Failed to set headers: %v", err)
		}

		err = c.SetAuth(ctx, req, serviceConfig.AuthType, authInfo)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Failed to set auth: %v", err)
			errChan <- fmt.Errorf("failed to set auth: %w", err)
			return
		}
		// Use the no-timeout client for streaming scenarios
		resp, err := c.streamHTTPClient.Do(req)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Streaming request failed: %v", err)
			errChan <- fmt.Errorf("request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		log.Printf("[baidu-plugin] [DEBUG] Streaming response status: %d", resp.StatusCode)

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		// Read response line by line
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				break
			}
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				errChan <- err
				return
			}

			// Re-encode and send
			chunkBytes, err := json.Marshal(chunk)
			if err != nil {
				errChan <- err
				return
			}

			select {
			case dataChan <- chunkBytes:
			case <-ctx.Done():
				return
			}
		}
	}()

	return dataChan, errChan
}

func (c *BaiduClient) getServiceConf(serviceName string) *YamlConfigService {
	for i := range c.config.Services {
		if c.config.Services[i].ServiceName == serviceName {
			return &c.config.Services[i]
		}
	}
	return nil
}

// Populate authorization headers according to AuthType; authInfo comes from aggregated config data
func (c *BaiduClient) SetAuth(ctx context.Context, req *http.Request, authType string, authInfo string) error {
	var credentials map[string]string
	err := json.Unmarshal([]byte(authInfo), &credentials)
	if err != nil {
		log.Printf("[baidu-plugin] [ERROR] Failed to unmarshal request credentials: %v", err)
		return fmt.Errorf("failed to unmarshal request credentials: %w", err)
	}
	if authType == "" || authType == "none" {
		return fmt.Errorf("invalid auth type: %s", authType)
	}

	switch authType {
	case "apikey":
		// Default to Bearer
		if v := credentials["api_key"]; v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
	default:
		// Fallback: if api_key exists, also use Bearer
		if v := credentials["api_key"]; v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
	}
	return nil
}

func (c *BaiduClient) setHeaders(req *http.Request, extraHeaders string) error {
	if extraHeaders != "{}" {
		var extraHeader map[string]interface{}
		if err := json.Unmarshal([]byte(extraHeaders), &extraHeader); err != nil {
			return fmt.Errorf("failed to parse extra headers: %w", err)
		}
		for k, v := range extraHeader {
			if strVal, ok := v.(string); ok {
				req.Header.Set(k, strVal)
			}
		}
	}
	return nil
}

func (c *BaiduClient) handleErrorResponse(resp *http.Response) error {
	var sbody string
	newBody, err := c.readResponseBody(resp)
	// b, err := io.ReadAll(resp.Body)
	if err != nil {
		sbody = string(newBody)
	}
	resp.Body.Close()
	return errors.New(sbody)
}

func (c *BaiduClient) readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.ReadCloser
	var err error

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	return io.ReadAll(reader)
}

// ValidateAuth verifies authentication credentials
func (c *BaiduClient) ValidateAuth(ctx context.Context) error {
	return nil
}

// RefreshAuth refreshes authentication credentials
// Some auth types (e.g. OAuth) require periodic token refresh
func (c *BaiduClient) RefreshAuth(ctx context.Context) error {
	return nil
}

// InvokeService is the core plugin entry-point
// serviceName: e.g. "chat", "embed"
// request: serialized request payload
// Returns: serialized response payload
func (c *BaiduClient) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
	return nil, nil
}
