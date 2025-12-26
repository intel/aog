package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// ServiceHandler defines the interface for service handlers
type ServiceHandler interface {
	Handle(ctx context.Context, request []byte) ([]byte, error)
	HandleStream(ctx context.Context, request []byte) (<-chan []byte, error)
}

// BaseHandler provides common functionality for all service handlers
type BaseHandler struct {
	ServiceName string
	OVMSHost    string
	OVMSPort    int
	Endpoint    string // API endpoint from manifest (e.g., /v3/chat/completions)
	Client      *http.Client
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(serviceName, ovmsHost string, ovmsPort int, endpoint string) *BaseHandler {
	return &BaseHandler{
		ServiceName: serviceName,
		OVMSHost:    ovmsHost,
		OVMSPort:    ovmsPort,
		Endpoint:    endpoint,
		Client:      &http.Client{
			// No timeout - inference services (e.g., text-to-image) may take long time
		},
	}
}

// GetOVMSURL returns the OVMS service URL
func (h *BaseHandler) GetOVMSURL(endpoint string) string {
	return fmt.Sprintf("http://%s:%d%s", h.OVMSHost, h.OVMSPort, endpoint)
}

// SendRequest sends a request to OVMS and returns the response with retry logic
func (h *BaseHandler) SendRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	url := h.GetOVMSURL(endpoint)
	log.Printf("[%s] Sending %s request to %s", h.ServiceName, method, url)

	// Serialize request body if provided
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Retry logic for transient failures
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[%s] Retry attempt %d/%d", h.ServiceName, attempt, maxRetries-1)
		}

		// Create request with fresh body reader for each attempt
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := h.Client.Do(req)
		if err != nil {
			lastErr = err
			// Check if error is retryable (timeout, connection refused, etc.)
			if isRetryableError(err) && attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusOK {
			// Don't retry on client errors (4xx)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, fmt.Errorf("OVMS returned status %d", resp.StatusCode)
			}
			// Retry on server errors (5xx)
			if resp.StatusCode >= 500 && attempt < maxRetries-1 {
				lastErr = fmt.Errorf("OVMS returned status %d", resp.StatusCode)
				continue
			}
			return nil, fmt.Errorf("OVMS returned status %d", resp.StatusCode)
		}

		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SendStreamRequest sends a streaming request to OVMS and returns channels for data and errors
func (h *BaseHandler) SendStreamRequest(ctx context.Context, method, endpoint string, body interface{}) (chan []byte, chan error) {
	dataChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)

		url := h.GetOVMSURL(endpoint)
		log.Printf("[%s] Sending streaming %s request to %s", h.ServiceName, method, url)

		// Serialize request body
		var bodyBytes []byte
		if body != nil {
			var err error
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				errChan <- fmt.Errorf("failed to marshal request body: %w", err)
				return
			}
		}

		// Create request
		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		// Send request (no timeout for streaming)
		resp, err := h.Client.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to send request: %w", err)
			return
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("OVMS returned status %d: %s", resp.StatusCode, string(body))
			return
		}

		// Read SSE stream (Server-Sent Events format)
		// OVMS returns data in SSE format: "data: {...}\n\n"
		buffer := make([]byte, 0, 4096)
		readBuffer := make([]byte, 4096)

		for {
			n, err := resp.Body.Read(readBuffer)
			if n > 0 {
				buffer = append(buffer, readBuffer[:n]...)

				// Process complete SSE messages (ending with \n\n)
				for {
					// Look for SSE message boundary
					idx := bytes.Index(buffer, []byte("\n\n"))
					if idx == -1 {
						break
					}

					// Extract one complete SSE message
					message := buffer[:idx]
					buffer = buffer[idx+2:] // Skip \n\n

					// Parse SSE format: "data: {...}"
					if bytes.HasPrefix(message, []byte("data: ")) {
						jsonData := bytes.TrimPrefix(message, []byte("data: "))
						jsonData = bytes.TrimSpace(jsonData)

						// Skip empty data or [DONE] marker
						if len(jsonData) == 0 || bytes.Equal(jsonData, []byte("[DONE]")) {
							continue
						}

						// Send chunk to channel
						select {
						case dataChan <- jsonData:
						case <-ctx.Done():
							return
						}
					}
				}
			}

			if err != nil {
				if err == io.EOF {
					// Stream ended normally
					break
				}
				errChan <- fmt.Errorf("failed to read stream: %w", err)
				return
			}

			// Check context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return dataChan, errChan
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Connection errors, timeouts, etc. are retryable
	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"no such host",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}

// ParseRequest parses the incoming request
func (h *BaseHandler) ParseRequest(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse request: %w", err)
	}
	return nil
}

// MarshalResponse marshals the response
func (h *BaseHandler) MarshalResponse(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return data, nil
}

// LogError logs an error with service context
func (h *BaseHandler) LogError(err error) {
	log.Printf("[%s] Error: %v", h.ServiceName, err)
}

// LogInfo logs info with service context
func (h *BaseHandler) LogInfo(msg string) {
	log.Printf("[%s] %s", h.ServiceName, msg)
}
