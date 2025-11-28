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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

// OllamaClient wraps Ollama API operations
type OllamaClient struct {
	config           *Config
	baseURL          *url.URL
	httpClient       *http.Client // For regular requests (with timeout)
	streamHTTPClient *http.Client // For long-running streaming requests (no timeout)
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(config *Config) (*OllamaClient, error) {
	baseURL, err := url.Parse(fmt.Sprintf("%s://%s", config.Scheme, config.Host))
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	return &OllamaClient{
		config:  config,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout, // Regular requests with 30-second timeout
		},
		streamHTTPClient: &http.Client{
			Timeout: 0, // Streaming requests with no timeout (for long operations like model downloads)
		},
	}, nil
}

// Do executes an HTTP request
// Automatically selects whether timeout is needed based on API path:
// - AI inference APIs (chat, generate, embeddings) use no-timeout client
// - Query/management APIs (tags, version, ps, etc.) use timeout client
func (c *OllamaClient) Do(ctx context.Context, method, path, authInfo string, reqBody, respBody interface{}) error {
	url := c.baseURL.String() + path
	log.Printf("[ollama-plugin] [DEBUG] HTTP Request: %s %s", method, url)

	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to marshal request: %v", err)
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] Failed to create HTTP request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Select HTTP client based on API path
	client := c.selectHTTPClient(path)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[ollama-plugin] [ERROR] HTTP request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[ollama-plugin] [DEBUG] HTTP Response: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[ollama-plugin] [ERROR] HTTP error %d: %s", resp.StatusCode, string(body))
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to decode response: %v", err)
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	log.Printf("[ollama-plugin] [DEBUG] HTTP request completed successfully")
	return nil
}

// selectHTTPClient selects the appropriate HTTP client based on API path
func (c *OllamaClient) selectHTTPClient(path string) *http.Client {
	// AI inference and model download APIs use no-timeout client
	longRunningAPIs := []string{
		"/api/chat",       // Chat inference (may take a long time)
		"/api/generate",   // Generate inference (may take a long time)
		"/api/embeddings", // Embedding generation (may take a long time)
		"/api/pull",       // Model download (takes minutes to tens of minutes)
	}

	for _, api := range longRunningAPIs {
		if path == api {
			log.Printf("[ollama-plugin] [DEBUG] Using no-timeout client for: %s", path)
			return c.streamHTTPClient // No timeout
		}
	}

	// Other APIs (query, management operations) use timeout client
	log.Printf("[ollama-plugin] [DEBUG] Using timeout client (30s) for: %s", path)
	return c.httpClient // 30-second timeout
}

// StreamResponse executes a streaming HTTP request
func (c *OllamaClient) StreamResponse(ctx context.Context, method, path, authInfo string, reqBody interface{}) (chan []byte, chan error) {
	url := c.baseURL.String() + path
	log.Printf("[ollama-plugin] [DEBUG] HTTP Streaming Request: %s %s", method, url)

	dataChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)
		defer log.Printf("[ollama-plugin] [DEBUG] HTTP streaming completed")

		var bodyReader io.Reader
		if reqBody != nil {
			jsonBody, err := json.Marshal(reqBody)
			if err != nil {
				log.Printf("[ollama-plugin] [ERROR] Failed to marshal streaming request: %v", err)
				errChan <- fmt.Errorf("failed to marshal request: %w", err)
				return
			}
			bodyReader = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Failed to create streaming request: %v", err)
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		// Use no-timeout HTTP client for streaming requests (e.g., model downloads)
		resp, err := c.streamHTTPClient.Do(req)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Streaming request failed: %v", err)
			errChan <- fmt.Errorf("request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		log.Printf("[ollama-plugin] [DEBUG] Streaming response status: %d", resp.StatusCode)

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		// Read response line by line
		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk map[string]interface{}
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					break
				}
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
