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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/plugin/registry"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
	sdkclient "github.com/intel/aog/plugin-sdk/client"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

// PluginInvoker implements plugin service invocation with strict capability-based routing.
// Core principle: Route based on capabilities declared in manifest without implicit degradation.
type PluginInvoker struct {
	task *ServiceTask
}

// NewPluginInvoker creates a new plugin invoker
func NewPluginInvoker(task *ServiceTask) *PluginInvoker {
	return &PluginInvoker{
		task: task,
	}
}

// Invoke implements plugin service invocation with capability-based routing.
func (p *PluginInvoker) Invoke(sp *types.ServiceProvider, content types.HTTPContent) (resp *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.LogicLogger.Error("[Plugin] Plugin panicked during invocation",
				"provider", sp.ProviderName,
				"service", sp.ServiceName,
				"panic", r,
				"stack", string(debug.Stack()))
			err = fmt.Errorf("plugin panicked: %v", r)
		}
	}()

	logger.LogicLogger.Debug("[Plugin] Invoking plugin service",
		"provider", sp.ProviderName,
		"service", sp.ServiceName,
		"taskid", p.task.Schedule.Id)

	serviceDef, err := p.getPluginServiceDef(sp)
	if err != nil {
		logger.LogicLogger.Error("[Plugin] Failed to get service definition",
			"provider", sp.ProviderName,
			"service", sp.ServiceName,
			"error", err)
		return nil, fmt.Errorf("service definition not found: %w", err)
	}

	clientRequestMode := p.getRequestModeFromTarget()

	if err := p.validateCapabilityMatch(clientRequestMode, serviceDef); err != nil {
		logger.LogicLogger.Error("[Plugin] Capability mismatch",
			"provider", sp.ProviderName,
			"service", sp.ServiceName,
			"error", err)
		return nil, err
	}
	// 4. 获取Provider实例
	var providerInst provider.ModelServiceProvider
	if sp.ServiceSource == types.ServiceSourceLocal {
		providerInst, err = provider.GetModelEngine(sp.Flavor)
		if err != nil {
			logger.LogicLogger.Error("[Plugin] Failed to get provider",
				"provider", sp.ProviderName,
				"error", err)
			return nil, fmt.Errorf("failed to get plugin provider: %w", err)
		}
		logger.LogicLogger.Debug("[Plugin] Got provider instance",
			"provider", sp.ProviderName,
			"flavor", sp.Flavor,
			"type", fmt.Sprintf("%T", providerInst))
	} else if sp.ServiceSource == types.ServiceSourceRemote {
		// Get remote plugin
		pluginRegistry := registry.GetGlobalPluginRegistry()
		if pluginRegistry == nil {
			return nil, fmt.Errorf("plugin registry not initialized")
		}

		remotePlugin, err := pluginRegistry.GetRemotePluginProvider(sp.Flavor)
		if err != nil {
			logger.LogicLogger.Error("[Plugin] Failed to get remote plugin",
				"provider", sp.ProviderName,
				"flavor", sp.Flavor,
				"error", err)
			return nil, fmt.Errorf("failed to get remote plugin provider: %w", err)
		}

		// Convert to ModelServiceProvider
		providerInst = registry.NewRemotePluginAdapter(remotePlugin)
	} else {
		return nil, fmt.Errorf("unsupported service source: %s", sp.ServiceSource)
	}

	switch clientRequestMode {
	case "websocket":
		return p.invokeWebSocket(providerInst, sp, serviceDef, content)
	case "streaming":
		return p.invokeStream(providerInst, sp, serviceDef, content)
	default:
		return p.invokeUnary(providerInst, sp, serviceDef, content)
	}
}

// getRequestModeFromTarget get request mode from Task.Target
// Target has been determined by the dispatch phase based on client side requests and service capabilities
func (p *PluginInvoker) getRequestModeFromTarget() string {
	// Check if it is a WebSocket.
	if p.task.Target.ExposeProtocol == types.ExposeProtocolWEBSOCKET {
		return "websocket"
	}

	// Check if it is a streaming request
	if p.task.Target.Stream {
		return "streaming"
	}

	return "unary"
}

// validateCapabilityMatch Verify that client side requests match service capabilities
func (p *PluginInvoker) validateCapabilityMatch(requestMode string, serviceDef *sdktypes.ServiceDef) error {
	switch requestMode {
	case "websocket":
		if !serviceDef.SupportsBidirectional() {
			return fmt.Errorf(
				"service %s does not support WebSocket (bidirectional streaming). "+
					"Please check the service manifest capabilities declaration",
				serviceDef.ServiceName)
		}
	case "streaming":
		if !serviceDef.SupportsStreaming() {
			return fmt.Errorf(
				"service %s does not support streaming response. "+
					"Please check the service manifest capabilities declaration",
				serviceDef.ServiceName)
		}
	}
	return nil
}

// invokeUnary Unary calls (non-streaming)
func (p *PluginInvoker) invokeUnary(
	providerInst provider.ModelServiceProvider,
	sp *types.ServiceProvider,
	serviceDef *sdktypes.ServiceDef,
	content types.HTTPContent,
) (*http.Response, error) {
	logger.LogicLogger.Debug("[Plugin] Using unary invocation",
		"provider", sp.ProviderName,
		"service", sp.ServiceName)

	timeout := p.getTimeout(serviceDef, 60*time.Second)
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	} else {
		// No timeout for long-running services (e.g., text-to-image, model pull)
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	// call plugin
	respData, err := providerInst.InvokeService(ctx, sp.ServiceName, sp.AuthKey, content.Body)
	if err != nil {
		return nil, fmt.Errorf("plugin invoke failed: %w", err)
	}

	return &http.Response{
		StatusCode:    200,
		Status:        "200 OK",
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(bytes.NewReader(respData)),
		ContentLength: int64(len(respData)),
	}, nil
}

// invokeStream streaming call
func (p *PluginInvoker) invokeStream(
	providerInst provider.ModelServiceProvider,
	sp *types.ServiceProvider,
	serviceDef *sdktypes.ServiceDef,
	content types.HTTPContent,
) (*http.Response, error) {
	logger.LogicLogger.Debug("[Plugin] Using streaming invocation",
		"provider", sp.ProviderName,
		"service", sp.ServiceName)

	logger.LogicLogger.Debug("[Plugin] Before type assertion",
		"provider_type", fmt.Sprintf("%T", providerInst))

	// ⚠️ Strict check: Plugins must implement StreamablePlugin
	streamablePlugin, ok := providerInst.(sdkclient.StreamablePlugin)
	if !ok {
		logger.LogicLogger.Error("[Plugin] Type assertion failed",
			"provider_type", fmt.Sprintf("%T", providerInst),
			"expected", "sdkclient.StreamablePlugin")
		return nil, fmt.Errorf("plugin does not implement StreamablePlugin interface, but manifest declares support_streaming=true")
	}

	logger.LogicLogger.Debug("[Plugin] Type assertion succeeded",
		"streamable_type", fmt.Sprintf("%T", streamablePlugin))

	timeout := p.getTimeout(serviceDef, 300*time.Second)
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		// No timeout for long-running streaming services
		ctx, cancel = context.WithCancel(context.Background())
	}

	// Invoke plug-in streaming interface
	chunkChan, err := streamablePlugin.InvokeServiceStream(ctx, sp.ServiceName, sp.AuthKey, content.Body)
	if err != nil {
		cancel() // Initialization failed, cancel context
		return nil, fmt.Errorf("failed to initialize stream: %w", err)
	}

	// Create pipeline
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()
		defer cancel() // Cancel the context after the streaming read is complete

		for chunk := range chunkChan {
			if chunk.Error != nil {
				pipeWriter.CloseWithError(chunk.Error)
				return
			}

			if len(chunk.Data) > 0 {
				if _, err := pipeWriter.Write(chunk.Data); err != nil {
					logger.LogicLogger.Error("[Plugin] Failed to write chunk", "error", err)
					return
				}
			}

			if chunk.IsFinal {
				break
			}
		}
	}()

	return &http.Response{
		StatusCode:    200,
		Status:        "200 OK",
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:          pipeReader,
		ContentLength: -1,
	}, nil
}

// invokeWebSocket WebSocket Bidirectional Calling
func (p *PluginInvoker) invokeWebSocket(
	providerInst provider.ModelServiceProvider,
	sp *types.ServiceProvider,
	serviceDef *sdktypes.ServiceDef,
	content types.HTTPContent,
) (*http.Response, error) {
	logger.LogicLogger.Debug("[Plugin] Using WebSocket invocation",
		"provider", sp.ProviderName,
		"service", sp.ServiceName)

	// Get WebSocket Connection
	wsConnID := content.Header.Get("X-WebSocket-ConnID")
	if wsConnID == "" {
		return nil, fmt.Errorf("missing WebSocket connection ID")
	}

	wsManager := client.GetGlobalWebSocketManager()
	wsConn, exists := wsManager.GetConnection(wsConnID)
	if !exists {
		return nil, fmt.Errorf("WebSocket connection not found: %s", wsConnID)
	}

	// Check if the plugin implements the BidirectionalPlugin interface
	// If implemented, the native implementation is preferred
	if bidiPlugin, ok := providerInst.(sdkclient.BidirectionalPlugin); ok {
		logger.LogicLogger.Debug("[Plugin] Using native BidirectionalPlugin implementation",
			"provider", sp.ProviderName,
			"service", sp.ServiceName)

		ctx, cancel := context.WithCancel(context.Background())
		var closeOnce sync.Once
		closeBridge := func(reason string, closeErr error) {
			closeOnce.Do(func() {
				logger.LogicLogger.Debug("[Plugin] Closing WebSocket bridge",
					"connID", wsConnID,
					"reason", reason,
					"error", closeErr)
				cancel()
				wsConn.Close()
			})
		}

		// Initialize bidirectional flow channel
		inStream := make(chan sdkclient.BidiMessage, 10)
		outStream := make(chan sdkclient.BidiMessage, 10)
		isFirst := false

		if wsConn.InputStream == nil {
			wsConn.InputStream = inStream
		}
		if wsConn.OutputStream == nil {
			wsConn.OutputStream = outStream
			isFirst = true
		}

		// Ensure model is loaded before starting bidirectional communication
		if isFirst && sp.ServiceName == "speech-to-text-ws" {
			sttParams := wsConn.GetSTTParams()
			if sttParams.Model != "" {
				logger.LogicLogger.Debug("[Plugin] Ensuring model is loaded before WebSocket communication",
					"model", sttParams.Model,
					"provider", sp.ProviderName)

				loadReq := &sdktypes.LoadRequest{
					Model: sttParams.Model,
				}

				// Load model and wait for it to be ready
				if err := providerInst.LoadModel(ctx, loadReq); err != nil {
					logger.LogicLogger.Error("[Plugin] Failed to load model for WebSocket service",
						"model", sttParams.Model,
						"provider", sp.ProviderName,
						"error", err)
					closeBridge("model_load_failed", err)
					return nil, fmt.Errorf("failed to load model %s: %w", sttParams.Model, err)
				}

				logger.LogicLogger.Info("[Plugin] Model loaded successfully for WebSocket service",
					"model", sttParams.Model,
					"provider", sp.ProviderName)
			}
		}

		// Start plug-in processing coroutine
		if isFirst {
			go func() {
				if err := bidiPlugin.InvokeServiceBidirectional(ctx, sp.ServiceName, wsConnID, sp.AuthKey, wsConn.InputStream, wsConn.OutputStream); err != nil {

					logger.LogicLogger.Error("[Plugin] Bidirectional stream error", "error", err)
					closeBridge("plugin_stream", err)
				}
			}()
		}

		// WebSocket → plugin
		inputMsgType := "text"
		isBinary := strings.Contains(content.Header.Get("Content-Type"), "application/octet-stream") ||
			strings.Contains(content.Header.Get("Content-Type"), "audio/")
		if isBinary {
			inputMsgType = "binary"
		}
		if inputMsgType == "text" {
			fmt.Printf(string(content.Body))
		}

		// Prepare metadata for bidirectional message
		metadata := make(map[string]string)
		if sp.ServiceName == "speech-to-text-ws" {
			// For speech-to-text-ws, include model and other parameters from session
			sttParams := wsConn.GetSTTParams()
			if sttParams.Model != "" {
				metadata["model"] = sttParams.Model
			}
			if sttParams.Language != "" {
				metadata["language"] = sttParams.Language
			}
			if sttParams.AudioFormat != "" {
				metadata["format"] = sttParams.AudioFormat
			}
			if sttParams.SampleRate > 0 {
				metadata["sample_rate"] = fmt.Sprintf("%d", sttParams.SampleRate)
			}
		}

		wsConn.InputStream <- sdkclient.BidiMessage{
			Data:        content.Body,
			MessageType: inputMsgType,
			Metadata:    metadata,
		}

		// plugin → WebSocket
		if isFirst {
			go func() {
				logger.LogicLogger.Debug("[Plugin] Started listening for plugin responses",
					"connID", wsConnID)
				for msg := range wsConn.OutputStream {
					logger.LogicLogger.Debug("[Plugin] Received message from plugin",
						"connID", wsConnID,
						"dataLen", len(msg.Data),
						"hasError", msg.Error != nil)
					if msg.Error != nil {
						logger.LogicLogger.Error("[Plugin] Received error from plugin", "error", msg.Error)
						// Send error message to client side
						errorMsg := map[string]interface{}{
							"error": msg.Error.Error(),
						}
						if errorData, err := json.Marshal(errorMsg); err == nil {
							_ = wsConn.Conn.WriteMessage(websocket.TextMessage, errorData)
						}
						close(wsConn.OutputStream)
						closeBridge("plugin_message", msg.Error)
						return
					}

					// Send the data returned by the plugin to the client side
					wsType := p.convertToWSMessageType(msg.MessageType)
					if err := wsConn.Conn.WriteMessage(wsType, msg.Data); err != nil {
						close(wsConn.OutputStream)
						logger.LogicLogger.Error("[Plugin] Failed to write WebSocket message", "error", err)
						closeBridge("ws_write", err)
						return
					}
					logger.LogicLogger.Debug("[Plugin] Successfully sent message to WebSocket client",
						"connID", wsConnID,
						"dataLen", len(msg.Data))
				}
				logger.LogicLogger.Debug("[Plugin] Output stream closed, listener exiting",
					"connID", wsConnID)
			}()
		}

		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
		}, nil
	}

	// If the plugin does not implement the BidirectionalPlugin interface, the default processing is used
	logger.LogicLogger.Warn("[Plugin] Plugin does not implement BidirectionalPlugin interface",
		"provider", sp.ProviderName,
		"service", sp.ServiceName)

	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
}

// getTimeout Get timeout
// Returns 0 if no timeout should be set (serviceDef.Timeout < 0)
func (p *PluginInvoker) getTimeout(serviceDef *sdktypes.ServiceDef, defaultTimeout time.Duration) time.Duration {
	if serviceDef.Timeout > 0 {
		return time.Duration(serviceDef.Timeout) * time.Second
	}
	if serviceDef.Timeout < 0 {
		return 0 // Negative value means no timeout
	}
	return defaultTimeout
}

// convertWSMessageType Convert WebSocket message type to string
func (p *PluginInvoker) convertWSMessageType(wsType int) string {
	switch wsType {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	case websocket.CloseMessage:
		return "close"
	default:
		return "binary"
	}
}

// convertToWSMessageType Convert string to WebSocket message type
func (p *PluginInvoker) convertToWSMessageType(msgType string) int {
	switch msgType {
	case "text":
		return websocket.TextMessage
	case "binary":
		return websocket.BinaryMessage
	case "ping":
		return websocket.PingMessage
	case "pong":
		return websocket.PongMessage
	case "close":
		return websocket.CloseMessage
	default:
		return websocket.BinaryMessage
	}
}

// getPluginServiceDef Get service definition from plugin manifest
func (p *PluginInvoker) getPluginServiceDef(sp *types.ServiceProvider) (*sdktypes.ServiceDef, error) {
	pluginRegistry := registry.GetGlobalPluginRegistry()
	if pluginRegistry == nil {
		return nil, fmt.Errorf("plugin registry not initialized")
	}

	manifest, err := pluginRegistry.GetPluginManifest(sp.Flavor)
	if err != nil {
		return nil, fmt.Errorf("plugin manifest not found for provider: %s", sp.Flavor)
	}

	serviceDef, err := manifest.GetServiceByName(sp.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("service %s not found in plugin %s: %w", sp.ServiceName, sp.Flavor, err)
	}

	return serviceDef, nil
}
