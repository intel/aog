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

package schedule

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/client/grpc/grpc_client"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
	"google.golang.org/grpc"
)

// Invoker defines the interface for service invocation
type Invoker interface {
	Invoke(sp *types.ServiceProvider, content types.HTTPContent) (*http.Response, error)
}

// GRPCInvoker implements the invocation of GRPC services
type GRPCInvoker struct {
	task *ServiceTask
}

// HTTPInvoker implements the invocation of HTTP services
type HTTPInvoker struct {
	task *ServiceTask
}

// 定义结构体来表示JSON数据
type Header struct {
	Action       string                 `json:"action"`
	TaskID       string                 `json:"task_id"`
	Streaming    string                 `json:"streaming"`
	Event        string                 `json:"event"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Attributes   map[string]interface{} `json:"attributes"`
}

type Output struct {
	Sentence struct {
		BeginTime int64  `json:"begin_time"`
		EndTime   *int64 `json:"end_time"`
		Text      string `json:"text"`
		Words     []struct {
			BeginTime   int64  `json:"begin_time"`
			EndTime     *int64 `json:"end_time"`
			Text        string `json:"text"`
			Punctuation string `json:"punctuation"`
		} `json:"words"`
	} `json:"sentence"`
	Usage interface{} `json:"usage"`
}

type Payload struct {
	TaskGroup  string `json:"task_group"`
	Task       string `json:"task"`
	Function   string `json:"function"`
	Model      string `json:"model"`
	Parameters Params `json:"parameters"`
	// Resources  []Resource `json:"resources"`
	Input  Input  `json:"input"`
	Output Output `json:"output,omitempty"`
}

type Params struct {
	Format                   string   `json:"format"`
	SampleRate               int      `json:"sample_rate"`
	VocabularyID             string   `json:"vocabulary_id"`
	DisfluencyRemovalEnabled bool     `json:"disfluency_removal_enabled"`
	LanguageHints            []string `json:"language_hints"`
}

type Input struct{}

type Event struct {
	Header  Header  `json:"header"`
	Payload Payload `json:"payload"`
}

// connect WebSocket service
func connectWebSocket(wsURL, apiKey string) (*websocket.Conn, error) {
	header := make(http.Header)
	header.Add("X-DashScope-DataInspection", "enable")
	header.Add("Authorization", fmt.Sprintf("bearer %s", apiKey))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	return conn, err
}

// NewInvoker creates the corresponding invoker
func NewInvoker(task *ServiceTask, providerType string) Invoker {
	switch providerType {
	case types.ProtocolGRPC:
		return &GRPCInvoker{task: task}
	case types.ProtocolHTTP:
		return &HTTPInvoker{task: task}
	default:
		return nil
	}
}

// Invoke implements GRPC service invocation
func (g *GRPCInvoker) Invoke(sp *types.ServiceProvider, content types.HTTPContent) (*http.Response, error) {
	// Get the corresponding service handler
	handler := NewGRPCServiceHandler(sp.ServiceName)
	if handler == nil {
		return nil, fmt.Errorf("unsupported service type: %s", sp.ServiceName)
	}

	// Check if it is a streaming service
	if g.task.Target.Protocol == types.ProtocolGRPC_STREAM {
		logger.LogicLogger.Debug("[Service] Invoking GRPC streaming service",
			"service", sp.ServiceName,
			"model", g.task.Target.Model,
			"taskid", g.task.Schedule.Id)
		return g.invokeStreamService(handler, content)
	}

	// non-streaming service processing
	logger.LogicLogger.Debug("[Service] Invoking GRPC non-streaming service",
		"service", sp.ServiceName,
		"model", g.task.Target.Model,
		"taskid", g.task.Schedule.Id)
	return g.invokeNonStreamService(handler, content)
}

// 处理非流式服务
func (g *GRPCInvoker) invokeNonStreamService(handler GRPCServiceHandler, content types.HTTPContent) (*http.Response, error) {
	// Establish a gRPC connection
	conn, err := grpc.Dial(g.task.Target.ServiceProvider.URL, grpc.WithInsecure())
	if err != nil {
		logger.LogicLogger.Error("Couldn't connect to endpoint %s: %v", g.task.Target.ServiceProvider.URL, err)
		return nil, err
	}
	defer conn.Close()

	// Create a gRPC client
	grpcClient := grpc_client.NewGRPCInferenceServiceClient(conn)

	// Prepare the request
	grpcReq, err := handler.PrepareRequest(content, g.task.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	// Send the request
	inferResponse, err := grpcClient.ModelInfer(context.Background(), grpcReq)
	if err != nil {
		logger.LogicLogger.Error("[Service] Error processing InferRequest", "taskid", g.task.Schedule.Id, "error", err)
		return nil, err
	}

	// Process the response
	resp, err := handler.ProcessResponse(inferResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to process response: %w", err)
	}

	logger.LogicLogger.Debug("[Service] Response Receiving", "taskid", g.task.Schedule.Id, "header",
		fmt.Sprintf("%+v", resp.Header), "task", g.task)

	return resp, nil
}

// 处理流式服务
func (g *GRPCInvoker) invokeStreamService(handler GRPCServiceHandler, content types.HTTPContent) (*http.Response, error) {
	// 1. Parsing WebSocket Connection ID and Task Type
	wsConnID := content.Header.Get("X-WebSocket-ConnID")
	if wsConnID == "" {
		return nil, fmt.Errorf("missing WebSocket connection ID")
	}

	wsManager := client.GetGlobalWebSocketManager()
	wsConn, _ := wsManager.GetConnection(wsConnID)
	taskType := wsConn.GetTaskType(g.task.Schedule.Id)

	// 2. Handle separately according to task type
	switch taskType {
	case types.WSActionFinishTask:
		// Processing end-of-task requests
		return g.handleFinishTask(handler, content, wsConnID)

	case types.WSActionRunTask:
		// Processing start task requests
		return g.handleRunTask(handler, content, wsConnID)

	default:
		// Processing audio data or other requests
		return g.handleStreamData(handler, content, wsConnID)
	}
}

// handleFinishTask Processing end-of-task requests
func (g *GRPCInvoker) handleFinishTask(handler GRPCServiceHandler, content types.HTTPContent, wsConnID string) (*http.Response, error) {
	// Close the gRPC stream session associated with the WebSocket connection
	grpcStreamManager := client.GetGlobalGRPCStreamManager()
	session := grpcStreamManager.GetSessionByWSConnID(wsConnID)

	// Notify the WebSocket Manager to close the streaming session
	wsManager := client.GetGlobalWebSocketManager()
	if wsConn, exists := wsManager.GetConnection(wsConnID); exists {
		wsConn.SetTaskFinished(g.task.Schedule.Id)
	}

	// Send empty audio data to ovms, which means the sending is complete.
	grpcReq, err := handler.PrepareStreamRequest(content, g.task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stream request: %w", err)
	}

	err = session.Stream.Send(grpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send empty audio data: %w", err)
	}

	// Successfully sent data, return confirmation immediately
	ackResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"processing"}`))),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
	return ackResponse, nil
}

// handleRunTask Processing start task requests
func (g *GRPCInvoker) handleRunTask(handler GRPCServiceHandler, content types.HTTPContent, wsConnID string) (*http.Response, error) {
	// Create a new gRPC stream session
	_, err := g.createStreamSession(handler, content, wsConnID)
	if err != nil {
		return nil, err
	}

	// Create Task Start Incident Response
	startedEvent := types.NewTaskStartedEvent(wsConnID)
	return createWebSocketEventResponse(startedEvent), nil
}

// handleStreamData Processing audio data or other streaming requests
func (g *GRPCInvoker) handleStreamData(handler GRPCServiceHandler, content types.HTTPContent, wsConnID string) (*http.Response, error) {
	// Attempt to fetch an existing gRPC streaming session
	grpcStreamManager := client.GetGlobalGRPCStreamManager()
	session := grpcStreamManager.GetSessionByWSConnID(wsConnID)

	// If no session is found, create a new session
	if session == nil {
		var err error
		session, err = g.createStreamSession(handler, content, wsConnID)
		if err != nil {
			return nil, err
		}

		// Immediately returns a startup success response
		startEvent := types.NewTaskStartedEvent(wsConnID)
		return createWebSocketEventResponse(startEvent), nil
	} else {
		// Prepare a request for an existing session
		grpcReq, err := handler.PrepareStreamRequest(content, g.task)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare stream request: %w", err)
		}

		// Send to an existing stream
		if err := session.Stream.Send(grpcReq); err != nil {
			grpcStreamManager.CloseSessionByWSConnID(wsConnID)
			logger.LogicLogger.Error("[Service] Error sending to existing stream",
				"taskid", g.task.Schedule.Id,
				"wsConnID", wsConnID,
				"error", err)
			return nil, err
		}

		// Successfully sent data, return confirmation immediately
		ackResponse := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"processing"}`))),
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}
		return ackResponse, nil
	}
}

// createStreamSession Create a new gRPC stream session
func (g *GRPCInvoker) createStreamSession(handler GRPCServiceHandler, content types.HTTPContent, wsConnID string) (*client.GRPCStreamSession, error) {
	logger.LogicLogger.Info("[Service] Creating new GRPC stream for WebSocket connection",
		"wsConnID", wsConnID,
		"taskID", g.task.Schedule.Id)

	// Establish a gRPC connection
	conn, err := grpc.Dial(g.task.Target.ServiceProvider.URL, grpc.WithInsecure())
	if err != nil {
		logger.LogicLogger.Error("[Service] Couldn't connect to endpoint",
			"url", g.task.Target.ServiceProvider.URL,
			"error", err)
		return nil, err
	}

	// Creating the gRPC client side
	gClient := grpc_client.NewGRPCInferenceServiceClient(conn)

	// Prepare a streaming request
	grpcReq, err := handler.PrepareStreamRequest(content, g.task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stream request: %w", err)
	}

	// Create context and cancel functions
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)

	// Create a two-way flow
	stream, err := gClient.ModelStreamInfer(ctx)
	if err != nil {
		cancel()
		logger.LogicLogger.Error("[Service] Error creating stream",
			"taskid", g.task.Schedule.Id,
			"wsConnID", wsConnID,
			"error", err)
		return nil, err
	}

	// Create a new session
	grpcStreamManager := client.GetGlobalGRPCStreamManager()
	session := grpcStreamManager.CreateSession(
		wsConnID,
		gClient,
		stream,
		ctx,
		cancel,
		g.task.Request.Service,
		g.task.Target.Model,
	)

	// Send initial request
	if err := stream.Send(grpcReq); err != nil {
		grpcStreamManager.CloseSessionByWSConnID(wsConnID)
		logger.LogicLogger.Error("[Service] Error sending stream request",
			"taskid", g.task.Schedule.Id,
			"wsConnID", wsConnID,
			"error", err)
		return nil, err
	}

	return session, nil
}

// createWebSocketEventResponse Create a WebSocket event response
func createWebSocketEventResponse(event types.WebSocketEventMessage) *http.Response {
	jsonData, _ := json.Marshal(event)
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}
}

// Invoke implements HTTP service invocation
func (h *HTTPInvoker) Invoke(sp *types.ServiceProvider, content types.HTTPContent) (*http.Response, error) {
	// 1. Build the invocation URL
	var err error
	var resp *http.Response
	if h.task.Target.ExposeProtocol == types.ExposeProtocolWEBSOCKET {
		resp, err = h.invokeWSService(sp, content)
	} else {
		resp, err = h.invokeNonWSService(sp, content)
	}
	return resp, err
}

func (h *HTTPInvoker) invokeWSService(sp *types.ServiceProvider, content types.HTTPContent) (*http.Response, error) {
	// 1. Parsing WebSocket Connection ID and Task Type
	wsConnID := content.Header.Get("X-WebSocket-ConnID")
	if wsConnID == "" {
		return nil, fmt.Errorf("missing WebSocket connection ID")
	}

	wsManager := client.GetGlobalWebSocketManager()
	wsConn, exists := wsManager.GetConnection(wsConnID)
	if !exists {
		return nil, fmt.Errorf("WebSocket connection not found: %s", wsConnID)
	}

	taskType := wsConn.GetTaskType(h.task.Schedule.Id)
	if taskType == "" {
		return nil, fmt.Errorf("task type not found for task ID: %d", h.task.Schedule.Id)
	}

	// 2. Obtain or create a WebSocket connection
	var conn *websocket.Conn
	var err error

	conn, exists = types.GetWSRemoteConnection(wsConnID)
	if !exists {
		// Create a new WebSocket connection
		invokeURL := sp.URL
		var authInfoData ApiKeyAuthInfo
		err = json.Unmarshal([]byte(sp.AuthKey), &authInfoData)
		if err != nil {
			return nil, bcode.WrapError(bcode.ErrAuthInfoParsing, err)
		}
		apiKey := authInfoData.ApiKey

		conn, err = connectWebSocket(invokeURL, apiKey)
		if err != nil {
			logger.LogicLogger.Error("[Service] Failed to connect to WebSocket",
				"url", invokeURL,
				"wsConnID", wsConnID,
				"error", err)
			return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
		}

		// restore remote connection
		types.SetWSRemoteConnection(wsConnID, conn)
	} else {
		logger.LogicLogger.Debug("[Service] Using existing WebSocket connection",
			"wsConnID", wsConnID)
	}

	// 3. Processing requests based on task type
	switch taskType {
	case types.WSActionFinishTask:
		return h.handleFinishTaskWS(sp, content, wsConnID, conn)
	case types.WSActionRunTask:
		return h.handleRunTaskWS(conn, wsConnID)
	default:
		return h.handleStreamDataWS(content, conn, wsConnID)
	}
}

// handleRunTaskWS Handling WebSocket run-task requests
func (h *HTTPInvoker) handleRunTaskWS(conn *websocket.Conn, wsConnID string) (*http.Response, error) {
	// Building run-task messages in Alibaba Cloud format
	runTaskCmd := Event{
		Header: Header{
			Action:    "run-task",
			TaskID:    wsConnID,
			Streaming: "duplex",
		},
		Payload: Payload{
			TaskGroup: "audio",
			Task:      "asr",
			Function:  "recognition",
			Model:     h.task.Target.Model,
			Parameters: Params{
				Format:     "wav",
				SampleRate: 16000,
			},
			Input: Input{},
		},
	}

	runTaskCmdJSON, err := json.Marshal(runTaskCmd)
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to marshal run-task command",
			"wsConnID", wsConnID,
			"error", err)
		return nil, fmt.Errorf("failed to marshal run-task command: %w", err)
	}

	// Send a run-task command to a remote service
	if err := conn.WriteMessage(websocket.TextMessage, runTaskCmdJSON); err != nil {
		logger.LogicLogger.Error("[Service] Failed to send run-task command",
			"wsConnID", wsConnID,
			"model", h.task.Target.Model,
			"error", err)
		return nil, fmt.Errorf("failed to send run-task command: %w", err)
	}

	logger.LogicLogger.Info("[Service] Successfully sent run-task command",
		"wsConnID", wsConnID,
		"model", h.task.Target.Model,
		"command", string(runTaskCmdJSON))

	// Returns a response that a task has been started
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(fmt.Sprintf(`{"status":"started","task_id":"%s"}`, wsConnID)))),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// handleStreamDataWS Processing WebSocket audio stream data
func (h *HTTPInvoker) handleStreamDataWS(content types.HTTPContent, conn *websocket.Conn, wsConnID string) (*http.Response, error) {
	// Direct forwarding of audio data to remote services
	if err := conn.WriteMessage(websocket.BinaryMessage, content.Body); err != nil {
		logger.LogicLogger.Error("[Service] Failed to send audio data to WebSocket",
			"wsConnID", wsConnID,
			"dataSize", len(content.Body),
			"error", err)
		return nil, fmt.Errorf("failed to send audio data: %w", err)
	}

	logger.LogicLogger.Debug("[Service] Successfully sent audio data to WebSocket",
		"wsConnID", wsConnID,
		"dataSize", len(content.Body))

	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"processing"}`))),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// handleFinishTaskWS Handling WebSocket find-task requests
func (h *HTTPInvoker) handleFinishTaskWS(sp *types.ServiceProvider, content types.HTTPContent, wsConnID string, conn *websocket.Conn) (*http.Response, error) {
	// prepare finish-task message
	finishTaskCmd := Event{
		Header: Header{
			Action:    "finish-task",
			TaskID:    wsConnID,
			Streaming: "duplex",
		},
		Payload: Payload{
			Input: Input{},
		},
	}

	finishTaskCmdJSON, err := json.Marshal(finishTaskCmd)
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to marshal finish-task command",
			"wsConnID", wsConnID,
			"error", err)
		return nil, fmt.Errorf("failed to marshal finish-task command: %w", err)
	}

	// Send finish-task action
	if err := conn.WriteMessage(websocket.TextMessage, finishTaskCmdJSON); err != nil {
		logger.LogicLogger.Error("[Service] Failed to send finish-task command",
			"wsConnID", wsConnID,
			"error", err)
		return nil, fmt.Errorf("failed to send finish-task command: %w", err)
	}

	logger.LogicLogger.Info("[Service] Successfully sent finish-task command",
		"wsConnID", wsConnID,
		"command", string(finishTaskCmdJSON))

	// Return a completed response
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"finished"}`))),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func (h *HTTPInvoker) invokeNonWSService(sp *types.ServiceProvider, content types.HTTPContent) (*http.Response, error) {
	invokeURL := sp.URL
	if strings.ToUpper(sp.Method) == http.MethodGet && len(content.Body) > 0 {
		u, err := utils.BuildGetRequestURL(sp.URL, content.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to build GET request URL: %w", err)
		}
		invokeURL = u
		content.Body = nil
	}

	// 2. Create the HTTP request
	req, err := http.NewRequest(sp.Method, invokeURL, bytes.NewReader(content.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 3. Set the request headers
	if err := h.setRequestHeaders(req, sp, content); err != nil {
		return nil, fmt.Errorf("failed to set request headers: %w", err)
	}

	if req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		var jsonData map[string]string
		err := json.NewDecoder(req.Body).Decode(&jsonData)
		if err != nil {
			return nil, err
		}

		// 2. Build multipart/form-data request body
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		for key, val := range jsonData {
			_ = writer.WriteField(key, val)
		}

		writer.Close()
		req.Body = io.NopCloser(&buf)
	}

	// 4. Handle authentication
	if sp.AuthType != types.AuthTypeNone {
		if err := h.handleAuthentication(req, sp, content); err != nil {
			return nil, fmt.Errorf("failed to handle authentication: %w", err)
		}
	}

	// 5. Create HTTP client and send request
	httpClient := h.createHTTPClient()

	// 6. Log the request
	h.logRequest(req, content)

	// 7. Send the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 8. Handle error response
	if resp.StatusCode != http.StatusOK {
		return h.handleErrorResponse(resp)
	}

	// 9. Handle segmented request
	serviceDefaultInfo := GetProviderServiceDefaultInfo(h.task.Target.ToFavor, h.task.Request.Service)
	if serviceDefaultInfo.RequestSegments > 1 {
		return h.handleSegmentedRequest(httpClient, resp, sp, serviceDefaultInfo)
	}
	newBody, err := h.readResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	// 10. Log the response
	logger.LogicLogger.Debug("[Service] Response Receiving",
		"taskid", h.task.Schedule.Id,
		"header", fmt.Sprintf("%+v", resp.Header),
		"task", h.task)

	return resp, nil
}

func (h *HTTPInvoker) invokeStreamService(sp *types.ServiceProvider, content types.HTTPContent, wsConnID string) (*http.Response, error) {
	wsManager := client.GetGlobalWebSocketManager()
	wsConn, exists := wsManager.GetConnection(wsConnID)
	if !exists {
		return nil, fmt.Errorf("WebSocket connection not found: %s", wsConnID)
	}

	invokeURL := sp.URL
	if strings.ToUpper(sp.Method) == http.MethodGet && len(content.Body) > 0 {
		u, err := utils.BuildGetRequestURL(sp.URL, content.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to build GET request URL: %w", err)
		}
		invokeURL = u
		content.Body = nil
	}

	req, err := http.NewRequest(sp.Method, invokeURL, bytes.NewReader(content.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	if err := h.setRequestHeaders(req, sp, content); err != nil {
		return nil, fmt.Errorf("failed to set request headers: %w", err)
	}

	if sp.AuthType != types.AuthTypeNone {
		if err := h.handleAuthentication(req, sp, content); err != nil {
			return nil, fmt.Errorf("failed to handle authentication: %w", err)
		}
	}

	httpClient := h.createHTTPClient()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			// Push to WebSocket client side
			if err := wsConn.WriteMessage(1, line); err != nil {
				logger.LogicLogger.Error("[HTTP WS] Failed to send message to WebSocket", "error", err, "connID", wsConnID)
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.LogicLogger.Error("[HTTP WS] Error reading HTTP stream", "error", err, "connID", wsConnID)
			break
		}
	}

	return nil, nil
}

// handleAuthentication Handle authentication
func (h *HTTPInvoker) handleAuthentication(req *http.Request, sp *types.ServiceProvider, content types.HTTPContent) error {
	authParams := &AuthenticatorParams{
		Request:      req,
		ProviderInfo: sp,
		RequestBody:  string(content.Body),
	}

	authenticator := ChooseProviderAuthenticator(authParams)
	if authenticator == nil {
		logger.LogicLogger.Error("[Service] Failed to choose authenticator")
		return fmt.Errorf("failed to choose authenticator")
	}

	if err := authenticator.Authenticate(); err != nil {
		logger.LogicLogger.Error("[Service] Failed to authenticate",
			"taskid", h.task.Schedule.Id,
			"error", err)
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	return nil
}

func (h *HTTPInvoker) handleSegmentedRequest(client *http.Client, resp *http.Response, sp *types.ServiceProvider, serviceDefaultInfo ServiceDefaultInfo) (*http.Response, error) {
	defer resp.Body.Close()
	logger.LogicLogger.Info("[Service] Handling segmented request", "taskid", h.task.Schedule.Id)

	// 1. Read and parse initial response
	taskID, err := h.parseInitialResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse initial response: %w", err)
	}

	// 2. Poll task status
	return h.pollTaskStatus(client, sp, serviceDefaultInfo, taskID)
}

func (h *HTTPInvoker) parseInitialResponse(resp *http.Response) (string, error) {
	body, err := h.readResponseBody(resp)
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

func (h *HTTPInvoker) readResponseBody(resp *http.Response) ([]byte, error) {
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

// pollTaskStatus Poll the task status
func (h *HTTPInvoker) pollTaskStatus(client *http.Client, sp *types.ServiceProvider, serviceDefaultInfo ServiceDefaultInfo, taskID string) (*http.Response, error) {
	const (
		pollInterval = 500 * time.Millisecond
		maxRetries   = 100 // Add maximum retry count to prevent infinite loops
	)

	retryCount := 0
	for retryCount < maxRetries {
		resp, err := h.getTaskStatus(client, sp, serviceDefaultInfo, taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to get task status: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return h.handleErrorResponse(resp)
		}

		status, body, err := h.parseTaskStatusResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse task status response: %w", err)
		}

		if h.isTaskComplete(status) {
			logger.LogicLogger.Info("[Service] Segmented request completed", "taskid", h.task.Schedule.Id)
			return &http.Response{
				StatusCode: resp.StatusCode,
				Header:     resp.Header.Clone(),
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		}

		time.Sleep(pollInterval)
		retryCount++
	}

	return nil, fmt.Errorf("exceeded maximum number of retries (%d) while polling task status", maxRetries)
}

// getTaskStatus Get the task status
func (h *HTTPInvoker) getTaskStatus(client *http.Client, sp *types.ServiceProvider, serviceDefaultInfo ServiceDefaultInfo, taskID string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", serviceDefaultInfo.RequestExtraUrl, taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create status request: %w", err)
	}

	if err := h.authenticateStatusRequest(req, sp); err != nil {
		return nil, fmt.Errorf("failed to authenticate status request: %w", err)
	}

	return client.Do(req)
}

// authenticateStatusRequest Authenticate the status request
func (h *HTTPInvoker) authenticateStatusRequest(req *http.Request, sp *types.ServiceProvider) error {
	authParams := &AuthenticatorParams{
		Request:      req,
		ProviderInfo: sp,
	}

	authenticator := ChooseProviderAuthenticator(authParams)
	if authenticator == nil {
		return fmt.Errorf("failed to choose authenticator for status request")
	}

	return authenticator.Authenticate()
}

// parseTaskStatusResponse Parse the task status response
func (h *HTTPInvoker) parseTaskStatusResponse(resp *http.Response) (string, []byte, error) {
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

// isTaskComplete Check if the task is complete
func (h *HTTPInvoker) isTaskComplete(status string) bool {
	return status == "FAILED" || status == "SUCCEEDED" || status == "UNKNOWN"
}

// logRequest Log the request
func (h *HTTPInvoker) logRequest(req *http.Request, content types.HTTPContent) {
	logger.LogicLogger.Info("[Service] Request Sending to Service Provider ...",
		"taskid", h.task.Schedule.Id,
		"url", req.URL.String())

	logger.LogicLogger.Debug("[Service] Request Sending to Service Provider ...",
		"taskid", h.task.Schedule.Id,
		"method", req.Method,
		"url", req.URL.String(),
		"header", fmt.Sprintf("%+v", req.Header),
		"body", string(content.Body))
}

func (h *HTTPInvoker) handleErrorResponse(resp *http.Response) (*http.Response, error) {
	var sbody string
	newBody, err := h.readResponseBody(resp)
	// b, err := io.ReadAll(resp.Body)
	if err != nil {
		sbody = string(newBody)
	}
	logger.LogicLogger.Warn("[Service] Service Provider returns Error", "taskid", h.task.Schedule.Id,
		"status_code", resp.StatusCode, "body", sbody)
	resp.Body.Close()
	return nil, &types.HTTPErrorResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       newBody,
	}
}

// createHTTPClient Create an HTTP client
func (h *HTTPInvoker) createHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		},
	}
}

// setRequestHeaders Set request headers
func (h *HTTPInvoker) setRequestHeaders(req *http.Request, sp *types.ServiceProvider, content types.HTTPContent) error {
	// set content headers
	for k, v := range content.Header {
		if k != "Content-Length" {
			req.Header.Set(k, v[0])
		}
	}

	// set extra headers
	if sp.ExtraHeaders != "{}" {
		var extraHeader map[string]interface{}
		if err := json.Unmarshal([]byte(sp.ExtraHeaders), &extraHeader); err != nil {
			logger.LogicLogger.Error("Error parsing extra headers:", err)
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

// GRPCServiceHandler define the interface for handling GRPC service requests
type GRPCServiceHandler interface {
	PrepareRequest(content types.HTTPContent, target *types.ServiceTarget) (*grpc_client.ModelInferRequest, error)
	ProcessResponse(inferResponse *grpc_client.ModelInferResponse) (*http.Response, error)

	PrepareStreamRequest(content types.HTTPContent, st *ServiceTask) (*grpc_client.ModelInferRequest, error)
	ProcessStreamResponse(streamResponse *grpc_client.ModelStreamInferResponse, wsConnID string) (*http.Response, bool, error)
}

type BaseGRPCHandler struct{}

func (h *BaseGRPCHandler) PrepareStreamRequest(content types.HTTPContent, st *ServiceTask) (*grpc_client.ModelInferRequest, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

func (h *BaseGRPCHandler) ProcessStreamResponse(streamResponse *grpc_client.ModelStreamInferResponse, wsConnID string) (*http.Response, bool, error) {
	return nil, true, fmt.Errorf("streaming not implemented")
}

// TextToImageHandler handles text-to-image service
type TextToImageHandler struct {
	BaseGRPCHandler
}

func NewTextToImageHandler() *TextToImageHandler {
	return &TextToImageHandler{
		BaseGRPCHandler: BaseGRPCHandler{},
	}
}

// SpeechToTextHandler handles speech-to-text service
type SpeechToTextHandler struct {
	BaseGRPCHandler
}

func NewSpeechToTextHandler() *SpeechToTextHandler {
	return &SpeechToTextHandler{
		BaseGRPCHandler: BaseGRPCHandler{},
	}
}

type SpeechToTextWSHandler struct {
	BaseGRPCHandler
}

func NewSpeechToTextWSHandler() *SpeechToTextWSHandler {
	return &SpeechToTextWSHandler{
		BaseGRPCHandler: BaseGRPCHandler{},
	}
}

type SpeechToTextWSRemoteHandler struct {
	BaseGRPCHandler
}

func NewSpeechToTextWSRemoteHandler() *SpeechToTextWSRemoteHandler {
	return &SpeechToTextWSRemoteHandler{
		BaseGRPCHandler: BaseGRPCHandler{},
	}
}

// PrepareRequest Implementing non-streaming request preparation
func (h *SpeechToTextWSHandler) PrepareRequest(content types.HTTPContent, target *types.ServiceTarget) (*grpc_client.ModelInferRequest, error) {
	return nil, fmt.Errorf("speech-to-text stream handler only supports streaming")
}

// ProcessResponse Implementing non-streaming response processing
func (h *SpeechToTextWSHandler) ProcessResponse(inferResponse *grpc_client.ModelInferResponse) (*http.Response, error) {
	return nil, fmt.Errorf("speech-to-text stream handler only supports streaming")
}

// PrepareStreamRequest Prepare a streaming request
func (h *SpeechToTextWSHandler) PrepareStreamRequest(content types.HTTPContent, st *ServiceTask) (*grpc_client.ModelInferRequest, error) {
	var actionMessage types.WebSocketActionMessage

	// First try to get the task type from the WebSocket connection
	wsConnID := content.Header.Get("X-WebSocket-ConnID")
	wsManager := client.GetGlobalWebSocketManager()
	wsConn, _ := wsManager.GetConnection(wsConnID)
	taskType := wsConn.GetTaskType(st.Schedule.Id)

	isBinary := taskType == types.WSTaskTypeAudio
	sttParams := wsConn.GetSTTParams()
	actionMessage.Parameters = &types.WebSocketParameters{
		Service:    st.Target.ServiceProvider.ServiceName,
		Format:     sttParams.AudioFormat,
		SampleRate: sttParams.SampleRate,
		Language:   sttParams.Language,
		UseVAD:     sttParams.UseVAD,
	}
	actionMessage.Model = st.Target.Model
	actionMessage.Action = taskType
	actionMessage.Task = st.Request.Service
	actionMessage.TaskID = wsConnID

	// Prepare parameter object
	params := h.prepareParams(content, isBinary, taskType, actionMessage)
	// Get audio data
	audioBytes := h.getAudioData(content, isBinary, taskType, actionMessage)
	// Build and return model inference requests
	return h.buildModelInferRequest(st.Target.Model, audioBytes, params), nil
}

// prepareParams Prepare all parameters
func (h *SpeechToTextWSHandler) prepareParams(content types.HTTPContent, isBinary bool, taskType string, actionMessage types.WebSocketActionMessage) *types.SpeechToTextParams {
	params := types.NewSpeechToTextParams()
	if actionMessage.Parameters.Format != "" {
		params.AudioFormat = actionMessage.Parameters.Format
	}
	if actionMessage.Parameters.SampleRate > 0 {
		params.SampleRate = actionMessage.Parameters.SampleRate
	}
	if actionMessage.Parameters.Language != "" {
		params.Language = actionMessage.Parameters.Language
	}
	if actionMessage.Parameters.ReturnFormat != "" {
		params.ReturnFormat = actionMessage.Parameters.ReturnFormat
	}
	if actionMessage.TaskID != "" {
		params.TaskID = actionMessage.TaskID
	}
	if actionMessage.Model != "" {
		params.Model = actionMessage.Model
	}
	if actionMessage.Parameters.Service != "" {
		params.Service = actionMessage.Parameters.Service
	}
	params.UseVAD = actionMessage.Parameters.UseVAD
	params.Action = actionMessage.Action

	return params
}

func (h *SpeechToTextWSHandler) ProcessStreamResponse(streamResponse *grpc_client.ModelStreamInferResponse, wsConnID string) (*http.Response, bool, error) {
	// Check if the response contains error messages
	if streamResponse.ErrorMessage != "" {
		return nil, false, fmt.Errorf("stream error from model: %s", streamResponse.ErrorMessage)
	}

	// Check if there is an inference response
	inferResponse := streamResponse.GetInferResponse()
	if inferResponse == nil || len(inferResponse.RawOutputContents) == 0 {
		return nil, true, nil // Returns a nil response, but continues to receive the stream
	}

	// Parsing text in SRT format
	srtText := string(inferResponse.RawOutputContents[0])
	lines := strings.Split(strings.TrimSpace(srtText), "\n")

	// Make sure there are enough rows to obtain valid data
	if len(lines) < 3 {
		return nil, true, nil // Incomplete data, continue to receive stream
	}

	// parse ID
	id := 0
	fmt.Sscanf(lines[0], "%d", &id)

	// Parse timestamp
	timeRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})`)
	matches := timeRegex.FindStringSubmatch(lines[1])
	if len(matches) != 3 {
		return nil, true, nil // Timestamp format is incorrect, continue receiving stream
	}

	// Get text content
	text := strings.Join(lines[2:], " ")

	// Building a single paragraph response (not an array)
	segment := map[string]interface{}{
		"id":    id,
		"start": matches[1],
		"end":   matches[2],
		"text":  text,
	}

	// Create a custom response message
	resultMsg := map[string]interface{}{
		"header": map[string]string{
			"task_id": wsConnID,
			"event":   types.WSEventResultGenerated,
		},
		"payload": segment,
	}

	// serialized response
	jsonData, err := json.Marshal(resultMsg)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal response to JSON: %v", err)
	}

	// Create an HTTP response
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	// Determine whether to continue receiving the stream (depending on whether there is an EOS flag)
	continueStream := true
	if inferResponse.Parameters != nil {
		if eosParam, exists := inferResponse.Parameters["eos"]; exists {
			if eosParam.GetBoolParam() {
				continueStream = false // 如果收到EOS标记，不再继续接收
			}
		}
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}, continueStream, nil
}

// getAudioData Get audio data
func (h *SpeechToTextWSHandler) getAudioData(content types.HTTPContent, isBinary bool, taskType string, actionMessage types.WebSocketActionMessage) []byte {
	// Processing binary audio data
	if isBinary {
		logger.LogicLogger.Debug("[Service] Processing binary audio data",
			"contentType", content.Header.Get("Content-Type"),
			"dataSize", len(content.Body))
		return content.Body
	}

	return []byte{}
}

// buildModelInferRequest 构建模型推理请求
func (h *SpeechToTextWSHandler) buildModelInferRequest(modelName string, audioBytes []byte, params *types.SpeechToTextParams) *grpc_client.ModelInferRequest {
	paramsJSON, err := params.ToJSON()
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to marshal params to JSON", "error", err)
		paramsJSON = []byte("{}")
	}

	rawContents := [][]byte{
		audioBytes,
		paramsJSON,
	}

	inferTensorInputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "audio",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "params",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
	}

	inferOutputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{
			Name: "result",
		},
	}

	return &grpc_client.ModelInferRequest{
		ModelName:        modelName,
		Inputs:           inferTensorInputs,
		Outputs:          inferOutputs,
		RawInputContents: rawContents,
	}
}

func (h *TextToImageHandler) PrepareRequest(content types.HTTPContent, target *types.ServiceTarget) (*grpc_client.ModelInferRequest, error) {
	var requestMap map[string]interface{}
	if err := json.Unmarshal(content.Body, &requestMap); err != nil {
		logger.LogicLogger.Error("[Service] Failed to unmarshal request body", "error", err)
		return nil, err
	}

	// Parse parameters
	prompt, ok := requestMap["prompt"].(string)
	if !ok {
		logger.LogicLogger.Error("[Service] Failed to get prompt from request body")
		return nil, fmt.Errorf("failed to get prompt from request body")
	}

	batch, height, width := h.parseImageParams(requestMap)

	// Prepare input data
	rawContents := [][]byte{
		[]byte(prompt),
		[]byte(fmt.Sprintf("%d", int(batch))),
		[]byte(strconv.Itoa(height)),
		[]byte(strconv.Itoa(width)),
	}

	// Prepare input tensors
	inferTensorInputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "prompt",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "batch",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "height",
			Datatype: "BYTES",
		},
		{
			Name:     "width",
			Datatype: "BYTES",
		},
	}

	// Prepare output tensors
	inferOutputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{
			Name: "image",
		},
	}

	return &grpc_client.ModelInferRequest{
		ModelName:        target.Model,
		Inputs:           inferTensorInputs,
		Outputs:          inferOutputs,
		RawInputContents: rawContents,
	}, nil
}

func (h *TextToImageHandler) parseImageParams(requestMap map[string]interface{}) (batch float64, height, width int) {
	batch = 1
	height = 1024
	width = 1024

	if batchVal, ok := requestMap["batch"].(float64); ok {
		batch = batchVal
	}

	if size, ok := requestMap["size"].(string); ok {
		sizeStr := strings.Split(size, "*")
		if len(sizeStr) == 2 {
			if num, err := strconv.Atoi(sizeStr[0]); err == nil {
				height = num
			}
			if num, err := strconv.Atoi(sizeStr[1]); err == nil {
				width = num
			}
		}
	}

	return batch, height, width
}

func (h *TextToImageHandler) ProcessResponse(inferResponse *grpc_client.ModelInferResponse) (*http.Response, error) {
	imageList, err := utils.ParseImageData(inferResponse.RawOutputContents[0])
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to parse image data", "error", err)
		return nil, err
	}

	outputList := make([]string, 0)
	for i, imageData := range imageList {
		imagePath, err := h.saveImage(imageData, i)
		if err != nil {
			logger.LogicLogger.Error("[Service] Failed to write image file", "error", err)
			continue
		}
		outputList = append(outputList, imagePath)
	}

	return h.createResponse(outputList)
}

func (h *TextToImageHandler) saveImage(imageData []byte, index int) (string, error) {
	now := time.Now()
	randNum := rand.Intn(10000)
	downloadPath, _ := utils.GetDownloadDir()
	imageName := fmt.Sprintf("%d%02d%02d%02d%02d%02d%04d%01d.png",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		randNum, index)
	imagePath := fmt.Sprintf("%s/%s", downloadPath, imageName)

	if err := os.WriteFile(imagePath, imageData, 0o644); err != nil {
		return "", err
	}
	return imagePath, nil
}

func (h *TextToImageHandler) createResponse(outputList []string) (*http.Response, error) {
	respHeader := make(http.Header)
	respHeader.Set("Content-Type", "application/json")

	respBody := map[string]interface{}{
		"local_path": outputList,
	}
	respBodyBytes, err := json.Marshal(respBody)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     respHeader,
		Body:       io.NopCloser(strings.NewReader(string(respBodyBytes))),
	}, nil
}

func (h *SpeechToTextHandler) PrepareRequest(content types.HTTPContent, target *types.ServiceTarget) (*grpc_client.ModelInferRequest, error) {
	var requestMap map[string]interface{}
	if err := json.Unmarshal(content.Body, &requestMap); err != nil {
		logger.LogicLogger.Error("[Service] Failed to unmarshal request body", "error", err)
		return nil, err
	}

	// 解析音频文件路径
	audioPath, ok := requestMap["audio"].(string)
	if !ok {
		logger.LogicLogger.Error("[Service] Failed to get audio path from request body")
		return nil, fmt.Errorf("failed to get audio path from request body")
	}

	// Parsing language parameters (optional)
	language := "<|zh|>"
	if lang, ok := requestMap["language"].(string); ok {
		language = fmt.Sprintf("<|%s|>", lang)
	}

	params := &types.SpeechToTextParams{
		Service:      target.ServiceProvider.ServiceName,
		Language:     language,
		ReturnFormat: "text",
	}

	// Serialize parameters as JSON strings
	paramsJSON, err := params.ToJSON()
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to marshal params to JSON", "error", err)
		paramsJSON = []byte("{}")
	}

	// Read audio files
	audioBytes, err := os.ReadFile(audioPath)
	if err != nil {
		logger.LogicLogger.Error("[Service] Failed to read audio file", "error", err)
		return nil, err
	}

	// Ready to enter data
	rawContents := [][]byte{
		audioBytes,
		paramsJSON,
	}

	// prepare input tensor
	inferTensorInputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "audio",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "params",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
	}

	// prepare output tensor
	inferOutputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{
			Name: "result",
		},
	}

	return &grpc_client.ModelInferRequest{
		ModelName:        target.Model,
		Inputs:           inferTensorInputs,
		Outputs:          inferOutputs,
		RawInputContents: rawContents,
	}, nil
}

func (h *SpeechToTextHandler) ProcessResponse(inferResponse *grpc_client.ModelInferResponse) (*http.Response, error) {
	if len(inferResponse.RawOutputContents) == 0 {
		return nil, fmt.Errorf("empty response from model")
	}

	// Parsing text in timestamp format
	srtText := string(inferResponse.RawOutputContents[0])
	lines := strings.Split(strings.TrimSpace(srtText), "\n")

	segments := make([]map[string]interface{}, 0, len(lines))
	// Match format: [start time, end time] Text content
	timeRegex := regexp.MustCompile(`^\[(\d+\.\d+),\s*(\d+\.\d+)\]\s*(.+)$`)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parsing timestamps and text
		matches := timeRegex.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		startTime := matches[1]
		endTime := matches[2]
		text := strings.TrimSpace(matches[3])

		// Convert seconds to hours: minutes: seconds, milliseconds format
		startTimeFormatted := utils.FormatSecondsToSRT(startTime)
		endTimeFormatted := utils.FormatSecondsToSRT(endTime)

		segment := map[string]interface{}{
			"id":    i + 1,
			"start": startTimeFormatted,
			"end":   endTimeFormatted,
			"text":  text,
		}

		segments = append(segments, segment)
	}

	// Building JSON responses
	response := map[string]interface{}{
		"segments": segments,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response to JSON: %v", err)
	}

	// Create an HTTP response
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}, nil
}

func NewGRPCServiceHandler(serviceType string) GRPCServiceHandler {
	switch serviceType {
	case types.ServiceTextToImage:
		return NewTextToImageHandler()
	case types.ServiceSpeechToText:
		return NewSpeechToTextHandler()
	case types.ServiceSpeechToTextWS:
		return NewSpeechToTextWSHandler()
	case types.ServiceTextToSpeech:
		return NewTextToSpeechHandler()
	default:
		return nil
	}
}

// TextToSpeechHandler handles text-to-speech service
type TextToSpeechHandler struct {
	BaseGRPCHandler
}

func NewTextToSpeechHandler() *TextToSpeechHandler {
	return &TextToSpeechHandler{
		BaseGRPCHandler: BaseGRPCHandler{},
	}
}

func (h *TextToSpeechHandler) PrepareRequest(content types.HTTPContent, target *types.ServiceTarget) (*grpc_client.ModelInferRequest, error) {
	var requestMap map[string]interface{}
	if err := json.Unmarshal(content.Body, &requestMap); err != nil {
		logger.LogicLogger.Error("[Service] Failed to unmarshal request body", "error", err)
		return nil, err
	}

	// Parse input text
	text, ok := requestMap["text"].(string)
	if !ok {
		logger.LogicLogger.Error("[Service] Failed to get text from request body")
		return nil, fmt.Errorf("failed to get text from request body")
	}

	// parse timbre
	voice := types.VoiceMale // 默认男声
	if _, ok := requestMap["voice"].(string); ok {
		voice = requestMap["voice"].(string)
	}

	params := []byte("{}")

	// Ready to enter data
	rawContents := [][]byte{
		[]byte(text),
		[]byte(voice),
		params,
	}

	// prepare input tensor
	inferTensorInputs := []*grpc_client.ModelInferRequest_InferInputTensor{
		{
			Name:     "text",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "voice",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
		{
			Name:     "params",
			Datatype: "BYTES",
			Shape:    []int64{1},
		},
	}

	// prepare output tensor
	inferOutputs := []*grpc_client.ModelInferRequest_InferRequestedOutputTensor{
		{
			Name: "audio",
		},
	}

	return &grpc_client.ModelInferRequest{
		ModelName:        target.Model,
		Inputs:           inferTensorInputs,
		Outputs:          inferOutputs,
		RawInputContents: rawContents,
	}, nil
}

func (h *TextToSpeechHandler) ProcessResponse(inferResponse *grpc_client.ModelInferResponse) (*http.Response, error) {
	if len(inferResponse.RawOutputContents) == 0 {
		return nil, fmt.Errorf("empty response from model")
	}

	audioData := inferResponse.RawOutputContents[0]
	// Write audioData to wav file
	if len(inferResponse.RawOutputContents) == 0 {
		return nil, fmt.Errorf("empty response from model")
	}

	now := time.Now()
	randNum := rand.Intn(10000)
	DownloadPath, _ := utils.GetDownloadDir()
	audioName := fmt.Sprintf("%d%02d%02d%02d%02d%02d%04d.wav", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), randNum)
	audioPath := fmt.Sprintf("%s/%s", DownloadPath, audioName)
	err := os.WriteFile(audioPath, audioData, 0o644)
	if err != nil {
		return nil, fmt.Errorf("write file failed")
	}

	// Building JSON responses
	response := map[string]interface{}{
		"url": audioPath,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(response, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response to JSON: %v", err)
	}

	// Create an HTTP response
	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(jsonData)),
	}, nil
}

func (h *TextToSpeechHandler) PrepareStreamRequest(content types.HTTPContent, st *ServiceTask) (*grpc_client.ModelInferRequest, error) {
	// Here you can implement text-to-speech streaming request preparation logic
	return nil, fmt.Errorf("text-to-speech streaming request preparation not implemented")
}

func (h *TextToSpeechHandler) ProcessStreamResponse(streamResponse *grpc_client.ModelStreamInferResponse, wsConnID string) (*http.Response, bool, error) {
	// Here you can implement text-to-speech streaming response processing logic
	return nil, false, fmt.Errorf("text-to-speech streaming response  not implemented")
}
