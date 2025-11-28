// internal/aliyun_client.go
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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin/examples/aliyun-plugin/internal/utils"
)

// AliyunClient wraps aliyun API operations
type AliyunClient struct {
	config           *Config
	baseURL          *url.URL
	httpClient       *http.Client // Used for normal requests (with timeout)
	streamHTTPClient *http.Client // Used for long-running streaming requests (no timeout)
}

const (
	taskStartedTimeout      = 45 * time.Second
	taskFinishedGracePeriod = 10 * time.Second
	outStreamSendTimeout    = 5 * time.Second
)

// NewAliyunClient creates a new aliyun client
func NewAliyunClient(config *Config) (*AliyunClient, error) {
	baseURL, err := url.Parse(config.Provider.EngineHost)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}

	return &AliyunClient{
		config:  config,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout, // Timeout for non-streaming requests
		},
		streamHTTPClient: &http.Client{
			Timeout: 0, // No timeout for streaming requests (e.g., downloading models)
		},
	}, nil
}

// Do executes an HTTP request
func (c *AliyunClient) Do(ctx context.Context, method, service, authInfo string, reqBody, respBody interface{}) error {
	var reqUrl string
	serviceConfig := c.getServiceConf(service)
	if serviceConfig.SpecialUrl != "" {
		reqUrl = serviceConfig.SpecialUrl
	} else {
		reqUrl = c.baseURL.String() + serviceConfig.Endpoint
	}
	LogicLogger.Info("[aliyun-plugin] [DEBUG] HTTP Request: %s %s", method, reqUrl)

	var bodyReader io.Reader
	var jsonBody []byte
	var err error
	if reqBody != nil {
		if _, ok := reqBody.([]byte); !ok {
			jsonBody, err = json.Marshal(reqBody)
			if err != nil {
				LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to marshal request: %v", err)
				return fmt.Errorf("failed to marshal request: %w", err)
			}
		} else {
			jsonBody = reqBody.([]byte)
		}
		bodyReader = bytes.NewReader(jsonBody)

	}

	req, err := http.NewRequestWithContext(ctx, method, reqUrl, bodyReader)
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to create HTTP request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	if reqBody != nil && req.Header.Get("Content-Encoding") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	slog.Error("------------------" + serviceConfig.ExtraHeaders)
	err = c.setHeaders(req, serviceConfig.ExtraHeaders)
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to set headers: %v", err)
	}

	err = c.SetAuth(ctx, req, serviceConfig.AuthType, authInfo)
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to set auth: %v", err)
		return fmt.Errorf("failed to set auth: %w, %s", err, authInfo)
	}

	// Select the proper HTTP client based on API path characteristics
	client := c.selectHTTPClient(serviceConfig.Endpoint)

	resp, err := client.Do(req)
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] HTTP request failed: %v", err)
		return fmt.Errorf("request failed: %w", err)
	}

	LogicLogger.Info("[aliyun-plugin] [DEBUG] HTTP Response: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		newBody, err := c.readResponseBody(resp)
		LogicLogger.Info("[aliyun-plugin] [ERROR] HTTP error %d: %s", resp.StatusCode, err)
		return fmt.Errorf("API returned %w (status= %s)(status=%d)", err, req.URL.String()+req.Header.Get("Authorization")+req.Header.Get("X-DashScope-Async")+"-"+string(len(jsonBody))+"-"+string(newBody), resp.StatusCode)
	}

	if serviceConfig.ExtraUrl != "" {
		return c.handleSegmentedRequest(client, resp, serviceConfig, authInfo, respBody)
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to decode response: %v", err)
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	LogicLogger.Info("[aliyun-plugin] [DEBUG] HTTP request completed successfully")
	return nil
}

// selectHTTPClient Select the appropriate HTTP client side based on the API path
func (c *AliyunClient) selectHTTPClient(path string) *http.Client {
	// AI inference and model download APIs use the no-timeout client
	longRunningAPIs := []string{}
	for _, service := range c.config.Services {
		longRunningAPIs = append(longRunningAPIs, service.Endpoint)
	}

	for _, api := range longRunningAPIs {
		if path == api {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Using no-timeout client for: %s", path)
			return c.streamHTTPClient // No timeout
		}
	}

	// Other APIs (query/management) use the timeout-enabled client
	LogicLogger.Info("[aliyun-plugin] [DEBUG] Using timeout client for: %s", path)
	return c.httpClient
}

// StreamResponse executes a streaming HTTP request
func (c *AliyunClient) StreamResponse(ctx context.Context, method, service, authInfo string, reqBody interface{}) (chan []byte, chan error) {
	var reqUrl string
	serviceConfig := c.getServiceConf(service)
	if serviceConfig.SpecialUrl != "" {
		reqUrl = serviceConfig.SpecialUrl
	} else {
		reqUrl = c.baseURL.String() + serviceConfig.Endpoint
	}

	LogicLogger.Info("[aliyun-plugin] [DEBUG] HTTP Streaming Request: %s %s", method, reqUrl)

	dataChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)
		defer LogicLogger.Info("[aliyun-plugin] [DEBUG] HTTP streaming completed")

		var bodyReader io.Reader
		var jsonBody []byte
		var err error
		if reqBody != nil {
			if _, ok := reqBody.([]byte); !ok {
				jsonBody, err = json.Marshal(reqBody)
				if err != nil {
					LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to marshal request: %v", err)
					errChan <- fmt.Errorf("failed to marshal request: %w", err)
					return
				}
			} else {
				jsonBody = reqBody.([]byte)
			}

			bodyReader = bytes.NewReader(jsonBody)

		}

		req, err := http.NewRequestWithContext(ctx, method, reqUrl, bodyReader)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to create streaming request: %v", err)
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		err = c.setHeaders(req, serviceConfig.ExtraHeaders)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to set headers: %v", err)
		}

		err = c.SetAuth(ctx, req, serviceConfig.AuthType, authInfo)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to set auth: %v", err)
			errChan <- fmt.Errorf("failed to set auth: %w", err)
			return
		}

		// Use the no-timeout HTTP client for streaming operations (e.g., downloads)
		resp, err := c.streamHTTPClient.Do(req)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Streaming request failed: %v", err)
			errChan <- fmt.Errorf("request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		LogicLogger.Info("[aliyun-plugin] [DEBUG] Streaming response status: %d", resp.StatusCode)

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

func (c *AliyunClient) Websocket(ctx context.Context, service string, connID string, authInfo string, inStream <-chan client.BidiMessage, outStream chan<- client.BidiMessage) error {
	// Retrieve service configuration
	defer func() {
		if err := recover(); err != nil {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Invoke service Websocket", err)
		}
	}()
	serviceConf := c.getServiceConf(service)
	if serviceConf == nil {
		return fmt.Errorf("service configuration not found for: %s", service)
	}

	// Parse auth payload
	var authInfoData map[string]interface{}
	err := json.Unmarshal([]byte(authInfo), &authInfoData)
	if err != nil {
		return fmt.Errorf("unmarshal auth info failed: %v", err)
	}

	// Resolve API key
	apiKey, ok := authInfoData["api_key"].(string)
	if !ok {
		return fmt.Errorf("api key or access key id not found in auth info")
	}

	// Connect to the Aliyun WebSocket endpoint
	LogicLogger.Info("[aliyun-plugin] [DEBUG] Connecting to WebSocket: %s", serviceConf.SpecialUrl)
	conn, err := utils.ConnectWebSocket(serviceConf.SpecialUrl, apiKey)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	LogicLogger.Info("[aliyun-plugin] [DEBUG] WebSocket connection established successfully")

	// Store the remote connection reference
	utils.SetWSRemoteConnection(connID, conn)

	// Create control channels for the streaming task
	taskStarted := make(chan bool, 1)
	taskDone := make(chan bool, 1)

	// Spawn result receiver (never closes caller-provided channels)
	c.startResultReceiver(conn, taskStarted, taskDone, outStream)

	// Send run-task command
	taskID, err := c.sendRunTaskCmd(conn)
	if err != nil {
		return fmt.Errorf("failed to send run-task command: %w", err)
	}

	// Wait for task-started event
	select {
	case <-taskStarted:
		log.Println("[aliyun-plugin] Task started successfully")
	case <-time.After(taskStartedTimeout):
		// If task-started never arrives, close connection and return error
		_ = conn.Close()
		return fmt.Errorf("timeout waiting for task-started event")
	case <-ctx.Done():
		_ = conn.Close()
		return fmt.Errorf("context cancelled while waiting for task-started")
	}

	// Launch goroutine to process inbound data
	go func() {
		defer func() {
			if err := recover(); err != nil {
				LogicLogger.Info("[aliyun-plugin] [DEBUG] Invoke service process inbound data", err)
			}
		}()
		finishRequested := false
		defer func() {
			// Allow upstream consumer to handle outStream closure
			// taskStarted/taskDone remain caller-owned
			LogicLogger.Info("[aliyun-plugin] [DEBUG] input-processing goroutine exiting")
		}()

		for {
			select {
			case msg, ok := <-inStream:
				if !ok {
					// Input channel closed, ensure finish-task is sent once
					if !finishRequested {
						finishRequested = true
						if finishErr := c.sendFinishTaskCmd(conn, taskID); finishErr != nil {
							LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to send finish-task command: %v", finishErr)
						} else {
							LogicLogger.Info("[aliyun-plugin] [DEBUG] Sent finish-task after input stream closed")
						}
					}
					// Wait for remote confirmation or context cancellation
					select {
					case <-taskDone:
						LogicLogger.Info("[aliyun-plugin] [DEBUG] Received task-done notification after finish-task")
						return
					case <-time.After(taskFinishedGracePeriod):
						LogicLogger.Info("[aliyun-plugin] [WARN] Timed out waiting for task-done after finish-task")
						return
					case <-ctx.Done():
						return
					}
				} else {
					// Handle different inbound message types
					switch msg.MessageType {
					case utils.WSSTTTaskTypeAudio, "binary":
						// Forward audio data to remote endpoint
						if err := conn.WriteMessage(websocket.BinaryMessage, msg.Data); err != nil {
							LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to send audio data: %v", err)
							sendBidiMessage(outStream, client.BidiMessage{
								MessageType: utils.WSErrorCodeClientError,
								Error:       err,
							})
							return
						}
					case "text":
						action, err := parseClientWSAction(msg.Data)
						if err != nil {
							LogicLogger.Info("[aliyun-plugin] [WARN] Failed to parse WS text action: %v", err)
							continue
						}
						switch action {
						case utils.WSActionFinishTask:
							if finishRequested {
								LogicLogger.Info("[aliyun-plugin] [DEBUG] Duplicate finish-task ignored")
								continue
							}
							finishRequested = true
							if finishErr := c.sendFinishTaskCmd(conn, taskID); finishErr != nil {
								LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to send finish-task command: %v", finishErr)
							} else {
								LogicLogger.Info("[aliyun-plugin] [DEBUG] Sent finish-task due to client request")
							}
							select {
							case <-taskDone:
								LogicLogger.Info("[aliyun-plugin] [DEBUG] Received task-done notification after client finish-task")
							case <-time.After(taskFinishedGracePeriod):
								LogicLogger.Info("[aliyun-plugin] [WARN] Timed out waiting for task completion after client finish-task")
							case <-ctx.Done():
								LogicLogger.Info("[aliyun-plugin] [WARN] Context cancelled while waiting for task completion")
							}
							return
						case utils.WSActionRunTask:
							LogicLogger.Info("[aliyun-plugin] [DEBUG] Received client run-task command; remote task already initialized")
						default:
							LogicLogger.Info("[aliyun-plugin] [WARN] Unsupported client action: %s", action)
						}
					default:
						LogicLogger.Info("[aliyun-plugin] [WARN] Unknown message type: %s", msg.MessageType)
					}
				}
			}
		}
	}()
	return nil
}

type clientWSAction struct {
	Action string `json:"action"`
}

func parseClientWSAction(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty payload")
	}
	var action clientWSAction
	if err := json.Unmarshal(data, &action); err != nil {
		return "", err
	}
	return action.Action, nil
}

// startResultReceiver launches a goroutine that asynchronously receives WebSocket messages
// Fix: never close caller-provided channels; use non-blocking writes to avoid stalls or duplicates
func (c *AliyunClient) startResultReceiver(conn *websocket.Conn, taskStarted chan<- bool, taskDone chan<- bool, outStream chan<- client.BidiMessage) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				LogicLogger.Info("[aliyun-plugin] [DEBUG] Invoke service startResultReceiver", err)
				return
			}
		}()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to read WebSocket message: %v", err)
				// 将错误传递给上层 consumer
				select {
				case outStream <- client.BidiMessage{
					MessageType: utils.WSErrorCodeClientError,
					Error:       err,
				}:
				default:
				}
				select {
				case taskDone <- true:
				default:
				}
				return
			}

			var event utils.Event
			err = json.Unmarshal(message, &event)
			if err != nil {
				LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to unmarshal event: %v; raw=%s", err, string(message))
				continue
			}

			// The handler handles events; the handler writes messages to outStream and controls taskStarted/taskDone when needed.
			if c.handleEvent(conn, event, taskStarted, taskDone, outStream) {
				// handler returns true to indicate that the read loop should end (task finished/failed)
				// Do not close any external channels
				return
			}
		}
	}()
}

func (c *AliyunClient) sendRunTaskCmd(conn *websocket.Conn) (string, error) {
	runTaskCmd, taskID, err := c.generateRunTaskCmd()
	if err != nil {
		return "", err
	}
	err = conn.WriteMessage(websocket.TextMessage, []byte(runTaskCmd))
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to write run-task command: %v", err)
	}
	return taskID, err
}

// generateRunTaskCmd builds the run-task payload
func (c *AliyunClient) generateRunTaskCmd() (string, string, error) {
	taskID := uuid.New().String()
	runTaskCmd := utils.Event{
		Header: utils.Header{
			Action:    utils.WSActionRunTask,
			TaskID:    taskID,
			Streaming: "duplex",
		},
		Payload: utils.Payload{
			TaskGroup: "audio",
			Task:      "asr",
			Function:  "recognition",
			Model:     "paraformer-realtime-v2",
			Parameters: utils.Params{
				Format:     "pcm",
				SampleRate: 16000,
			},
			Input: utils.Input{},
		},
	}
	runTaskCmdJSON, err := json.Marshal(runTaskCmd)
	return string(runTaskCmdJSON), taskID, err
}

// sendFinishTaskCmd issues the finish-task command
func (c *AliyunClient) sendFinishTaskCmd(conn *websocket.Conn, taskID string) error {
	finishTaskCmd, err := c.generateFinishTaskCmd(taskID)
	if err != nil {
		return err
	}
	err = conn.WriteMessage(websocket.TextMessage, []byte(finishTaskCmd))
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to write finish-task command: %v", err)
	}
	return err
}

// generateFinishTaskCmd builds the finish-task payload
func (c *AliyunClient) generateFinishTaskCmd(taskID string) (string, error) {
	finishTaskCmd := utils.Event{
		Header: utils.Header{
			Action:    utils.WSActionFinishTask,
			TaskID:    taskID,
			Streaming: "duplex",
		},
		Payload: utils.Payload{
			Input: utils.Input{},
		},
	}
	finishTaskCmdJSON, err := json.Marshal(finishTaskCmd)
	return string(finishTaskCmdJSON), err
}

// handleEvent processes incoming events
func (c *AliyunClient) handleEvent(conn *websocket.Conn, event utils.Event, taskStarted chan<- bool, taskDone chan<- bool, outStream chan<- client.BidiMessage) bool {
	switch event.Header.Event {
	case utils.WSEventTaskStarted:
		log.Println("[aliyun-plugin] Received task-started event")
		// Non-blocking write to avoid duplicate sends and blocking
		select {
		case taskStarted <- true:
		default:
		}
	case utils.WSEventResultGenerated:
		// Marshal recognition result and push to output stream
		resultData, err := json.Marshal(event)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to marshal result: %v", err)
			return false
		}

		// Deliver result to upstream consumer
		sendBidiMessage(outStream, client.BidiMessage{
			MessageType: "text",
			Data:        resultData,
		})
	case utils.WSEventTaskFinished:
		LogicLogger.Info("[aliyun-plugin] Task finished")
		// Notify upstream (non-blocking)
		resultData, err := json.Marshal(event)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to marshal result: %v", err)
			return false
		}
		sendBidiMessage(outStream, client.BidiMessage{
			MessageType: "text",
			Data:        resultData,
		})
		select {
		case taskDone <- true:
		default:
		}
		return true
	case utils.WSEventTaskFailed:
		errorMsg := "Unknown error"
		if event.Header.ErrorMessage != "" {
			errorMsg = event.Header.ErrorMessage
		}
		LogicLogger.Info("[aliyun-plugin] Task failed: %s", errorMsg)

		// Propagate failure upstream
		sendBidiMessage(outStream, client.BidiMessage{
			MessageType: utils.WSEventTaskFailed,
			Error:       errors.New(errorMsg),
		})

		// Signal completion
		select {
		case taskDone <- true:
		default:
		}
		return true
	default:
		// Unexpected event: log and forward raw payload upstream
		LogicLogger.Info("[aliyun-plugin] Unexpected event: %v", event)
		raw, _ := json.Marshal(event)
		sendBidiMessage(outStream, client.BidiMessage{
			MessageType: "unknown",
			Data:        raw,
		})
	}
	return false
}

func sendBidiMessage(outStream chan<- client.BidiMessage, msg client.BidiMessage) {
	select {
	case outStream <- msg:
	case <-time.After(outStreamSendTimeout):
		LogicLogger.Info("[aliyun-plugin] [WARN] Timed out sending message type %s to outStream", msg.MessageType)
	}
}

func (c *AliyunClient) getServiceConf(serviceName string) *YamlConfigService {
	for i := range c.config.Services {
		if c.config.Services[i].ServiceName == serviceName {
			return &c.config.Services[i]
		}
	}
	return nil
}

// Populate authentication headers based on AuthType; authInfo comes from aggregated config data
func (c *AliyunClient) SetAuth(ctx context.Context, req *http.Request, authType string, authInfo string) error {
	var credentials map[string]string
	err := json.Unmarshal([]byte(authInfo), &credentials)
	if err != nil {
		LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to unmarshal request credentials: %v", err)
		return fmt.Errorf("failed to unmarshal request credentials: %w, %s", err, authInfo)
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
	case "token":
		appkey := credentials["app_key"]
		reqUrl, _ := url.Parse(req.URL.String() + "?appkey=" + appkey)
		req.URL = reqUrl
		accessKeyId := credentials["access_key_id"]
		accessKeySecret := credentials["access_key_secret"]
		if accessKeyId == "" || accessKeySecret == "" {
			return fmt.Errorf("miss auth info: %s", "access_key_id or access_key_secret")
		}
		token, err := utils.GetToken(accessKeyId, accessKeySecret)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Failed to get token: %v", err)
		}
		req.Header.Set("X-NLS-Token", token)
	default:
		// Fallback: if api_key exists, use Bearer
		if v := credentials["api_key"]; v != "" {
			req.Header.Set("Authorization", "Bearer "+v)
		}
	}
	return nil
}

func (c *AliyunClient) setHeaders(req *http.Request, extraHeaders string) error {
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

// isTaskComplete Check if the task is complete
func (c *AliyunClient) isTaskComplete(status string) bool {
	return status == "FAILED" || status == "SUCCEEDED" || status == "UNKNOWN"
}

func (c *AliyunClient) handleErrorResponse(resp *http.Response) error {
	var sbody string
	newBody, err := c.readResponseBody(resp)
	// b, err := io.ReadAll(resp.Body)
	if err != nil {
		sbody = string(newBody)
	}
	resp.Body.Close()
	return errors.New(sbody)
}

// parseTaskStatusResponse Parse the task status response
func (c *AliyunClient) parseTaskStatusResponse(resp *http.Response) (string, []byte, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var respData struct {
		Output struct {
			TaskStatus string `json:"task_status"`
		} `json:"output"`
	}

	if err := json.Unmarshal(body, &respData); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return respData.Output.TaskStatus, body, nil
}

// getTaskStatus Get the task status
func (c *AliyunClient) getTaskStatus(client *http.Client, serviceConfig *YamlConfigService, authInfo string, taskID string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", serviceConfig.ExtraUrl, taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}

	if err := c.SetAuth(context.Background(), req, serviceConfig.AuthType, authInfo); err != nil {
		return nil, fmt.Errorf("failed to authenticate status request: %w", err)
	}
	if err = c.setHeaders(req, serviceConfig.ExtraHeaders); err != nil {
		return nil, fmt.Errorf("failed to set headers: %w", err)
	}

	return client.Do(req)
}

// pollTaskStatus Poll the task status
func (c *AliyunClient) pollTaskStatus(client *http.Client, serviceConfig *YamlConfigService, authInfo string, taskID string, respBody interface{}) error {
	const (
		pollInterval = 1000 * time.Millisecond
		maxRetries   = 100 // Add maximum retry count to prevent infinite loops
	)

	retryCount := 0
	for retryCount < maxRetries {
		resp, err := c.getTaskStatus(client, serviceConfig, authInfo, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return c.handleErrorResponse(resp)
		}

		status, body, err := c.parseTaskStatusResponse(resp)
		if err != nil {
			return fmt.Errorf("failed to parse task status response: %w", err)
		}

		if c.isTaskComplete(status) {
			err = json.Unmarshal(body, respBody)
			if err != nil {
				return fmt.Errorf("failed to unmarshal task status response: %w", err)
			}
			return nil
		}

		time.Sleep(pollInterval)
		retryCount++
	}

	return fmt.Errorf("exceeded maximum number of retries (%d) while polling task status", maxRetries)
}

func (c *AliyunClient) readResponseBody(resp *http.Response) ([]byte, error) {
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

func (c *AliyunClient) parseInitialResponse(resp *http.Response) (string, error) {
	body, err := c.readResponseBody(resp)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var submitRespData struct {
		Output struct {
			TaskId string `json:"task_id"`
		} `json:"output"`
	}

	if err := json.Unmarshal(body, &submitRespData); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return submitRespData.Output.TaskId, nil
}

func (c *AliyunClient) handleSegmentedRequest(client *http.Client, resp *http.Response, serviceConfig *YamlConfigService, authInfo string, respBody interface{}) error {
	defer resp.Body.Close()

	// 1. Read and parse initial response
	taskID, err := c.parseInitialResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to parse initial response: %w", err)
	}

	// 2. Poll task status
	return c.pollTaskStatus(client, serviceConfig, authInfo, taskID, respBody)
}

// ValidateAuth Verify that the authentication information is valid
func (c *AliyunClient) ValidateAuth(ctx context.Context) error {
	return nil
}

// RefreshAuth refresh authentication information
// Some authentication methods (e.g. OAuth) require periodic token refresh
func (c *AliyunClient) RefreshAuth(ctx context.Context) error {
	return nil
}

// InvokeService is the core plugin entry point
// serviceName: service identifier (e.g., "chat", "embed")
// request: serialized request payload
// Returns: serialized response payload
func (c *AliyunClient) InvokeService(ctx context.Context, serviceName string, request []byte) ([]byte, error) {
	return nil, nil
}
