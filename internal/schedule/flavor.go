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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/intel/aog/internal/client"
	"github.com/intel/aog/internal/constants"
	"github.com/intel/aog/internal/convert"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider/template"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
	"github.com/intel/aog/internal/utils/bcode"
	"github.com/intel/aog/version"
	"gopkg.in/yaml.v3"
)

// APIFlavor mode is usually set to "default". And set to "stream" if it is using stream mode
type APIFlavor interface {
	Name() string
	InstallRoutes(server *gin.Engine)

	// GetStreamResponseProlog In stream mode, some flavor may ask for some packets to be send first
	// or at the end, in addition to normal contents. For example, OpenAI
	// needs to send an additional "data: [DONE]" after everything is done.
	GetStreamResponseProlog(service string) []string
	GetStreamResponseEpilog(service string) []string

	// Convert This should cover the 6 conversion methods below
	Convert(service string, conversion string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)

	ConvertRequestToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
	ConvertRequestFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
	ConvertResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
	ConvertResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
	ConvertStreamResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
	ConvertStreamResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error)
}

var allFlavors = make(map[string]APIFlavor)

func RegisterAPIFlavor(f APIFlavor) {
	allFlavors[f.Name()] = f
}

func AllAPIFlavors() map[string]APIFlavor {
	return allFlavors
}

func GetAPIFlavor(name string) (APIFlavor, error) {
	flavor, ok := allFlavors[name]
	if !ok {
		return nil, bcode.WrapError(bcode.ErrProviderNotExist, fmt.Errorf("API Flavor %s not found", name))
	}
	return flavor, nil
}

//------------------------------------------------------------

type FlavorConversionDef struct {
	Prologue   []string                  `yaml:"prologue"`
	Epilogue   []string                  `yaml:"epilogue"`
	Conversion []types.ConversionStepDef `yaml:"conversion"`
}

type ModelSelector struct {
	ModelInRequest  string `yaml:"request"`
	ModelInResponse string `yaml:"response"`
}
type FlavorServiceDef struct {
	Protocol              string              `yaml:"protocol"`
	ExposeProtocol        string              `yaml:"expose_protocol"`
	TaskType              string              `yaml:"task_type"`
	Endpoints             []string            `yaml:"endpoints"`
	InstallRawRoutes      bool                `yaml:"install_raw_routes"`
	DefaultModel          string              `yaml:"default_model"`
	RequestUrl            string              `yaml:"url"`
	RequestExtraUrl       string              `yaml:"extra_url"`
	AuthType              string              `yaml:"auth_type"`
	AuthApplyUrl          string              `yaml:"auth_apply_url"`
	RequestSegments       int                 `yaml:"request_segments"`
	ExtraHeaders          string              `yaml:"extra_headers"`
	SupportModels         []string            `yaml:"support_models"`
	ModelSelector         ModelSelector       `yaml:"model_selector"`
	RequestToAOG          FlavorConversionDef `yaml:"request_to_aog"`
	RequestFromAOG        FlavorConversionDef `yaml:"request_from_aog"`
	ResponseToAOG         FlavorConversionDef `yaml:"response_to_aog"`
	ResponseFromAOG       FlavorConversionDef `yaml:"response_from_aog"`
	StreamResponseToAOG   FlavorConversionDef `yaml:"stream_response_to_aog"`
	StreamResponseFromAOG FlavorConversionDef `yaml:"stream_response_from_aog"`
}

type FlavorDef struct {
	Version  string                      `yaml:"version"`
	Name     string                      `yaml:"name"`
	Services map[string]FlavorServiceDef `yaml:"services"`
}

var allConversions = []string{
	"request_to_aog", "request_from_aog", "response_to_aog", "response_from_aog",
	"stream_response_to_aog", "stream_response_from_aog",
}

func EnsureConversionNameValid(conversion string) {
	for _, p := range allConversions {
		if p == conversion {
			return
		}
	}
	panic("[Flavor] Invalid Conversion Name: " + conversion)
}

// Not all elements are defined in the YAML file. So need to handle and return nil
// Example: getConversionDef("chat", "request_to_aog")
func (f *FlavorDef) getConversionDef(service, conversion string) *FlavorConversionDef {
	EnsureConversionNameValid(conversion)
	if serviceDef, exists := f.Services[service]; exists {
		var def FlavorConversionDef
		switch conversion {
		case "request_to_aog":
			def = serviceDef.RequestToAOG
		case "request_from_aog":
			def = serviceDef.RequestFromAOG
		case "response_to_aog":
			def = serviceDef.ResponseToAOG
		case "response_from_aog":
			def = serviceDef.ResponseFromAOG
		case "stream_response_to_aog":
			def = serviceDef.StreamResponseToAOG
		case "stream_response_from_aog":
			def = serviceDef.StreamResponseFromAOG
		default:
			panic("[Flavor] Invalid Conversion Name: " + conversion)
		}
		return &def
	}
	return nil
}

func LoadFlavorDef(flavor string) (FlavorDef, error) {
	data, err := template.FlavorTemplateFs.ReadFile(flavor + ".yaml")
	if err != nil {
		return FlavorDef{}, bcode.WrapError(bcode.ErrReadRequestBody, err)
	}
	var def FlavorDef
	err = yaml.Unmarshal(data, &def)
	if err != nil {
		return FlavorDef{}, bcode.WrapError(bcode.ErrUnmarshalRequestBody, err)
	}
	if def.Name != flavor {
		return FlavorDef{}, bcode.WrapError(bcode.ErrParameterValidation,
			fmt.Errorf("flavor name %s does not match file name %s", def.Name, flavor))
	}
	return def, nil
}

var allFlavorDefs = make(map[string]FlavorDef)

func GetFlavorDef(flavor string) FlavorDef {
	// Force reload so changes in flavor config files take effect on the fly
	if _, exists := allFlavorDefs[flavor]; !exists {
		def, err := LoadFlavorDef(flavor)
		if err != nil {
			logger.LogicLogger.Error("[Init] Failed to load flavor config", "flavor", flavor, "error", err)
			// This shouldn't happen unless something goes wrong
			// Directly panic without recovering
			panic(err)
		}
		allFlavorDefs[flavor] = def
	}
	return allFlavorDefs[flavor]
}

//------------------------------------------------------------

func InitAPIFlavors() error {
	err := convert.InitConverters()
	if err != nil {
		return bcode.WrapError(bcode.ErrMiddlewareHandle, err)
	}
	files, err := template.FlavorTemplateFs.ReadDir(".")
	if err != nil {
		return bcode.WrapError(bcode.ErrReadRequestBody, err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" {
			baseName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			flavor, err := NewConfigBasedAPIFlavor(GetFlavorDef(baseName))
			if err != nil {
				logger.LogicLogger.Error("[Flavor] Failed to create API Flavor", "flavor", baseName, "error", err)
				return bcode.WrapError(bcode.ErrMiddlewareHandle, err)
			}
			RegisterAPIFlavor(flavor)
		}
	}
	return nil
}

// ------------------------------------------------------------

type ConfigBasedAPIFlavor struct {
	Config             FlavorDef
	converterPipelines map[string]map[string]*convert.ConverterPipeline
}

func NewConfigBasedAPIFlavor(config FlavorDef) (*ConfigBasedAPIFlavor, error) {
	flavor := ConfigBasedAPIFlavor{
		Config: config,
	}
	err := flavor.reloadConfig()
	if err != nil {
		return nil, bcode.WrapError(bcode.ErrMiddlewareHandle, err)
	}
	return &flavor, nil
}

// We need to do reload here instead of replace the entire pointer of ConfigBasedAPIFlavor
// This is because we don't want to break the existing routes which are already installed
// with the Handler using the old pointer to ConfigBasedAPIFlavor
// So we can only update most of the internal states of ConfigBasedAPIFlavor
// NOTE: as stated, the routes etc. defined in the ConfigBasedAPIFlavor are not updated
func (f *ConfigBasedAPIFlavor) reloadConfig() error {
	// Reload the config if needed
	f.Config = GetFlavorDef(f.Config.Name)
	// rebuild the pipelines
	pipelines := make(map[string]map[string]*convert.ConverterPipeline)
	for service := range f.Config.Services {
		pipelines[service] = make(map[string]*convert.ConverterPipeline)
		for _, conv := range allConversions {
			// nil PipelineDef means empty []ConversionStepDef, it still creates a pipeline but
			// its steps are empty slice too
			p, err := convert.NewConverterPipeline(f.Config.getConversionDef(service, conv).Conversion)
			if err != nil {
				return bcode.WrapError(bcode.ErrFlavorConvertRequest, err)
			}
			pipelines[service][conv] = p
		}
	}
	f.converterPipelines = pipelines
	// PPrint(">>> Rebuilt Converter Pipelines", f.converterPipelines)
	return nil
}

func (f *ConfigBasedAPIFlavor) GetConverterPipeline(service, conv string) *convert.ConverterPipeline {
	EnsureConversionNameValid(conv)
	return f.converterPipelines[service][conv]
}

func (f *ConfigBasedAPIFlavor) Name() string {
	return f.Config.Name
}

func (f *ConfigBasedAPIFlavor) InstallRoutes(gateway *gin.Engine) {
	vSpec := version.SpecVersion
	for service, serviceDef := range f.Config.Services {
		if serviceDef.Protocol == types.ProtocolGRPC || serviceDef.Protocol == types.ProtocolGRPC_STREAM {
			continue
		}

		for _, endpoint := range serviceDef.Endpoints {
			parts := strings.SplitN(endpoint, " ", 2)
			endpoint = strings.TrimSpace(endpoint)
			if len(parts) != 2 {
				logger.LogicLogger.Error("[Flavor] Invalid endpoint format", "endpoint", endpoint)
				panic("[Flavor] Invalid endpoint format: " + endpoint)
			}
			method := parts[0]
			path := parts[1]
			method = strings.TrimSpace(method)
			path = strings.TrimSpace(path)
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			handler := makeServiceRequestHandler(f, service)

			// raw routes which doesn't have any aog prefix
			if serviceDef.InstallRawRoutes {
				gateway.Handle(method, path, handler)
				logger.LogicLogger.Debug("[Flavor] Installed raw route", "flavor", f.Name(), "service", service, "route", method+" "+path)
			}
			// flavor routes in api_flavors or directly under services
			if f.Name() != constants.AppName {
				routerPath := fmt.Sprintf("/%s/%s/api_flavors/%s/%s", constants.AppName, vSpec, f.Name(), path)
				gateway.Handle(method, routerPath, handler)
				logger.LogicLogger.Debug("[Flavor] Installed flavor route", "flavor", f.Name(), "service", service, "route", method+" "+routerPath)
			} else if method == types.ExposeProtocolWEBSOCKET {
				routerPath := fmt.Sprintf("/%s/%s/services/%s", constants.AppName, vSpec, service)
				gateway.Handle("GET", routerPath, makeWebSocketHandler(f, service))
				logger.LogicLogger.Debug("[Flavor] Installed websocket route", "flavor", f.Name(), "service", service, "route", "GET "+routerPath)
			} else {
				routerPath := fmt.Sprintf("/%s/%s/services/%s", constants.AppName, vSpec, path)
				gateway.Handle(method, routerPath, makeServiceRequestHandler(f, service))
				logger.LogicLogger.Debug("[Flavor] Installed aog route", "flavor", f.Name(), "service", service, "route", method+" "+routerPath)
			}
		}
		logger.LogicLogger.Info("[Flavor] Installed routes", "flavor", f.Name(), "service", service)
	}
}

func (f *ConfigBasedAPIFlavor) GetStreamResponseProlog(service string) []string {
	return f.Config.getConversionDef(service, "stream_response_from_aog").Prologue
}

func (f *ConfigBasedAPIFlavor) GetStreamResponseEpilog(service string) []string {
	return f.Config.getConversionDef(service, "stream_response_from_aog").Epilogue
}

func (f *ConfigBasedAPIFlavor) Convert(service, conversion string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	pipeline := f.GetConverterPipeline(service, conversion)
	logger.LogicLogger.Debug("[Flavor] Converting", "flavor", f.Name(), "service", service, "conversion", conversion, "content", len(content.Body))
	return pipeline.Convert(content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertRequestToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "request_to_aog", content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertRequestFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "request_from_aog", content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "response_to_aog", content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "response_from_aog", content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertStreamResponseToAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "stream_response_to_aog", content, ctx)
}

func (f *ConfigBasedAPIFlavor) ConvertStreamResponseFromAOG(service string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	return f.Convert(service, "stream_response_from_aog", content, ctx)
}

func makeServiceRequestHandler(flavor APIFlavor, service string) func(c *gin.Context) {
	return func(c *gin.Context) {
		logger.LogicLogger.Info("[Handler] Invoking service", "flavor", flavor.Name(), "service", service)

		w := c.Writer
		taskID, ch, err := InvokeService(flavor.Name(), service, c.Request)
		if err != nil {
			logger.LogicLogger.Error("[Handler] Failed to invoke service", "flavor", flavor.Name(), "service", service, "error", err)
			// Use wrapped error to preserve context
			bcode.ReturnError(c, err)
			return
		}

		closeNotifier, ok := w.(http.CloseNotifier)
		if !ok {
			logger.LogicLogger.Error("[Handler] Not found http.CloseNotifier")
			bcode.ReturnError(c, bcode.ErrUnsupportedCloseNotifier)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			bcode.ReturnError(c, bcode.ErrUnsupportedFlusher)
			return
		}

		isHTTPCompleted := false
	outerLoop:
		for {
			select {
			case <-closeNotifier.CloseNotify():
				logger.LogicLogger.Warn("[Handler] Client connection disconnected", "taskID", taskID)
				isHTTPCompleted = true
			case data, ok := <-ch:
				if !ok {
					logger.LogicLogger.Debug("[Handler] Service task channel closed", "taskID", taskID)
					break outerLoop
				}
				logger.LogicLogger.Debug("[Handler] Received service result", "result status", data.StatusCode, "result body", len(data.HTTP.Body))
				if isHTTPCompleted {
					// skip below statements but do not quit
					// we should exhaust the channel to allow it to be closed
					continue
				}

				// Handle error results using unified bcode.ReturnError
				if data.Type == types.ServiceResultFailed && data.Error != nil {
					logger.LogicLogger.Error("[Handler] Service task failed", "taskID", taskID, "error", data.Error)
					bcode.ReturnError(c, data.Error)
					isHTTPCompleted = true
					continue
				}

				if data.Type == types.ServiceResultDone || data.Type == types.ServiceResultFailed {
					isHTTPCompleted = true
				}

				// For successful responses, use standard format processing
				if data.Type == types.ServiceResultDone && data.StatusCode == http.StatusOK {
					processSuccessResponse(c, data)
				} else {
					// Stream responses or non-standard status code responses are written back directly
					for k, v := range data.HTTP.Header {
						if len(v) > 0 {
							c.Writer.Header().Set(k, v[0])
						}
					}
					c.Writer.WriteHeader(data.StatusCode)
					c.Writer.Write(data.HTTP.Body)
				}
				flusher.Flush()
			}
		}
		logger.LogicLogger.Info("end_session", []string{flavor.Name(), service})
	}
}

// processSuccessResponse Process successful responses, using the bcode standard format
func processSuccessResponse(c *gin.Context, result *types.ServiceResult) {
	// Check if it is a JSON response
	contentType := ""
	for k, v := range result.HTTP.Header {
		if strings.ToLower(k) == "content-type" && len(v) > 0 {
			contentType = v[0]
			break
		}
	}

	// If it is a JSON response, use the bcode standard format
	if strings.Contains(contentType, "application/json") {
		var rawData interface{}
		if json.Unmarshal(result.HTTP.Body, &rawData) == nil {
			// Create response with original data source
			c.JSON(http.StatusOK, rawData)
			return
		}
	}

	// Non-JSON or parsing failed, using raw response
	for k, v := range result.HTTP.Header {
		if len(v) > 0 {
			c.Writer.Header().Set(k, v[0])
		}
	}
	c.Writer.WriteHeader(result.StatusCode)
	c.Writer.Write(result.HTTP.Body)
}

// GRPCWebSocketHandler handle GRCWebSocket
func GRPCWebSocketHandler(wsConn *client.WebSocketConnection, done chan struct{}) {
	// Monitor if a gRPC session has been created
	var session *client.GRPCStreamSession
	var isMonitoring bool

	// Constantly check if there are any available sessions
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			// WebSocket connection closed, exit goroutine
			logger.LogicLogger.Info("[Handler] WebSocket connection closed, stopping stream monitor",
				"connID", wsConn.ID)
			return

		case <-ticker.C:
			// If you are already listening to the stream, skip the check
			if isMonitoring {
				continue
			}

			// Check if a session has been created
			if session == nil {
				grpcStreamManager := client.GetGlobalGRPCStreamManager()
				session = grpcStreamManager.GetSessionByWSConnID(wsConn.ID)

				if session != nil {
					logger.LogicLogger.Info("[Handler] Found gRPC session for WebSocket, starting to monitor",
						"connID", wsConn.ID)

					// The flag has started listening
					isMonitoring = true

					// Stop ticking, no more regular checks
					ticker.Stop()

					// Start listening to the stream continuously (this will block the current goroutine until the stream closes or an error occurs).
					monitorStreamResponses(session, wsConn)

					// After listening, reset the state to restart the inspection
					session = nil
					isMonitoring = false
					ticker = time.NewTicker(100 * time.Millisecond)
				}
			}
		}
	}
}

// RemoteWebSocketHandler handle HTTPWebSocket
func RemoteWebSocketHandler(wsConn *client.WebSocketConnection, done chan struct{}) {
	logger.LogicLogger.Info("[Handler] Starting remote WebSocket handler", "connID", wsConn.ID)

	defer func() {
		// Clear connection
		types.RemoveWSRemoteConnection(wsConn.ID)
		logger.LogicLogger.Info("[Handler] Remote WebSocket handler stopped", "connID", wsConn.ID)
	}()

	// Wait for a remote WebSocket connection to be created
	var remoteWebSocketConn *websocket.Conn
	var exists bool

	// 最多等待30秒
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			// WebSocket connection closed, exit goroutine
			logger.LogicLogger.Info("[Handler] WebSocket connection closed, stopping remote handler",
				"connID", wsConn.ID)
			return
		case <-timeout:
			logger.LogicLogger.Error("[Handler] Timeout waiting for remote WebSocket connection", "connID", wsConn.ID)
			return
		case <-ticker.C:
			// Attempt to obtain a remote WebSocket connection
			remoteWebSocketConn, exists = types.GetWSRemoteConnection(wsConn.ID)
			if exists {
				logger.LogicLogger.Info("[Handler] Remote WebSocket connection found, starting to monitor", "connID", wsConn.ID)
				break
			}
			continue
		}
		break
	}

	// Continuously listen for remote WebSocket messages
	for {
		select {
		case <-done:
			// WebSocket connection closed, exit goroutine
			logger.LogicLogger.Info("[Handler] WebSocket connection closed, stopping remote handler",
				"connID", wsConn.ID)
			return
		default:
			// Set read timeout
			remoteWebSocketConn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Read message
			messageType, message, err := remoteWebSocketConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.LogicLogger.Error("[Handler] Remote WebSocket connection closed unexpectedly",
						"connID", wsConn.ID,
						"error", err)
				}
				return
			}

			logger.LogicLogger.Info("[Handler] Received message from remote WebSocket",
				"connID", wsConn.ID,
				"messageType", messageType,
				"messageSize", len(message),
				"message", string(message))

			// 处理消息
			if err := processRemoteWebSocketMessage(messageType, message, wsConn); err != nil {
				logger.LogicLogger.Error("[Handler] Failed to process remote WebSocket message",
					"connID", wsConn.ID,
					"error", err)
			}
		}
	}
}

// processRemoteWebSocketMessage Handling remote WebSocket messages
func processRemoteWebSocketMessage(messageType int, message []byte, wsConn *client.WebSocketConnection) error {
	switch messageType {
	case websocket.TextMessage:
		// Processing text messages (JSON format)
		var responseData map[string]interface{}
		if err := json.Unmarshal(message, &responseData); err != nil {
			logger.LogicLogger.Error("[Handler] Failed to unmarshal WebSocket message",
				"connID", wsConn.ID,
				"error", err,
				"message", string(message))
			return fmt.Errorf("failed to unmarshal WebSocket message: %w", err)
		}

		logger.LogicLogger.Info("[Handler] Parsed WebSocket message",
			"connID", wsConn.ID,
			"responseData", responseData)

		// Check if it is the final result.
		if header, ok := responseData["header"].(map[string]interface{}); ok {
			if event, ok := header["event"].(string); ok {
				logger.LogicLogger.Info("[Handler] Processing WebSocket event",
					"connID", wsConn.ID,
					"event", event)

				switch event {
				case types.WSEventResultGenerated:
					// This is the final result, sent to the WebSocket client side
					if err := wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to forward result to WebSocket client",
							"connID", wsConn.ID,
							"error", err)
						return fmt.Errorf("failed to forward result to WebSocket client: %w", err)
					}
					logger.LogicLogger.Info("[Handler] Forwarded final result to WebSocket client",
						"connID", wsConn.ID,
						"result", string(message))

				case types.WSEventTaskFinished:
					// Task completed, send completion event
					finishEvent := types.NewTaskFinishedEvent(wsConn.ID)
					finishData, _ := json.Marshal(finishEvent)
					if err := wsConn.WriteMessage(websocket.TextMessage, finishData); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to send finish event",
							"connID", wsConn.ID,
							"error", err)
						return fmt.Errorf("failed to send finish event: %w", err)
					}
					logger.LogicLogger.Info("[Handler] Task finished", "connID", wsConn.ID)

				case types.WSEventTaskStarted:
					// Start the task and forward it to the client side
					if err := wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to forward task-started event",
							"connID", wsConn.ID,
							"error", err)
						return fmt.Errorf("failed to forward task-started event: %w", err)
					}
					logger.LogicLogger.Info("[Handler] Forwarded task-started event",
						"connID", wsConn.ID)

				case types.WSEventTaskFailed:
					logger.LogicLogger.Error("[Handler] Task failed", "connID", wsConn.ID)
					// Forward task failure event
					if err := wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
						return fmt.Errorf("failed to forward task-failed event: %w", err)
					}
					return nil

				default:
					// Forwarding other events
					if err := wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to forward event",
							"connID", wsConn.ID,
							"error", err)
						return fmt.Errorf("failed to forward event: %w", err)
					}
					logger.LogicLogger.Info("[Handler] Forwarded event to WebSocket client",
						"connID", wsConn.ID,
						"event", event)
				}
			} else {
				logger.LogicLogger.Warn("[Handler] No event field in header",
					"connID", wsConn.ID,
					"header", header)
			}
		} else {
			logger.LogicLogger.Warn("[Handler] No header field in response",
				"connID", wsConn.ID,
				"responseData", responseData)
		}

	case websocket.BinaryMessage:
		// Processing binary messages (audio data, etc.)
		logger.LogicLogger.Info("[Handler] Received binary message",
			"connID", wsConn.ID,
			"size", len(message))

		if err := wsConn.WriteMessage(websocket.BinaryMessage, message); err != nil {
			logger.LogicLogger.Error("[Handler] Failed to forward binary message",
				"connID", wsConn.ID,
				"error", err)
			return fmt.Errorf("failed to forward binary message: %w", err)
		}

	default:
		logger.LogicLogger.Warn("[Handler] Unknown WebSocket message type",
			"connID", wsConn.ID,
			"messageType", messageType)
	}

	return nil
}

func makeWebSocketHandler(flavor APIFlavor, service string) func(c *gin.Context) {
	return func(c *gin.Context) {
		logger.LogicLogger.Info("[Handler] Invoking websocket service", "flavor", flavor.Name(), "service", service)

		// WebSocket upgrader configuration
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow WebSocket connection requests from all origins
				// In production environment, origins should be restricted based on security requirements
				return true
			},
		}

		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.LogicLogger.Error("[Handler] Failed to upgrade to websocket", "error", err)
			bcode.ReturnError(c, bcode.WrapError(bcode.ErrWebSocketUpgradeFailed, err))
			return
		}

		logger.LogicLogger.Info("[Handler] WebSocket connection established", "flavor", flavor.Name(), "service", service)

		// Save original request information for subsequent message processing
		originalRequest := c.Request

		// Register WebSocket connection to manager
		wsConn := client.GetGlobalWebSocketManager().RegisterConnection(conn, 0, flavor.Name(), service)

		// Set up the connection closure handler
		conn.SetCloseHandler(func(code int, text string) error {
			logger.LogicLogger.Info("[Handler] WebSocket connection closing",
				"connID", wsConn.ID,
				"code", code,
				"reason", text)

			// Close connection, which will also close associated GRPC stream
			wsConn.Close()

			// Call default close handler
			return nil
		})

		defer wsConn.Close() // Ensure connection is cleaned up

		// Set up completion flag and close channel
		done := make(chan struct{})

		// Goroutine to handle messages received from client
		go func() {
			defer close(done)
			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logger.LogicLogger.Error("[Handler] WebSocket read error", "error", err, "connID", wsConn.ID)
					} else {
						logger.LogicLogger.Info("[Handler] WebSocket connection closed by client", "connID", wsConn.ID)
					}
					return
				}

				logger.LogicLogger.Debug("[Handler] Received WebSocket message",
					"type", messageType,
					"message", len(string(message)),
					"connID", wsConn.ID)

				// Handle received WebSocket messages based on message type
				switch messageType {
				case websocket.TextMessage:
					// Handle text messages
					handleWebSocketMessage(wsConn, flavor.Name(), service, message, originalRequest, "application/json")
				case websocket.BinaryMessage:
					// Handle binary messages, such as audio data
					handleWebSocketMessage(wsConn, flavor.Name(), service, message, originalRequest, "application/octet-stream")
				case websocket.CloseMessage:
					// Client requests to close connection
					logger.LogicLogger.Info("[Handler] Received close message from client", "connID", wsConn.ID)
					return
				case websocket.PingMessage:
					// Received Ping message, reply with Pong
					if err := wsConn.WriteMessage(websocket.PongMessage, nil); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to send pong", "error", err, "connID", wsConn.ID)
						return
					}
				}

				time.Sleep(100 * time.Millisecond) // Avoid too fast cycles
			}
		}()
		grpcStreamManager := client.GetGlobalGRPCStreamManager()
		for {
			grpcStream := grpcStreamManager.GetSessionByWSConnID(wsConn.ID)
			if grpcStream != nil {
				logger.LogicLogger.Debug("[Handler] Received WebSocket grpc stream")
				go GRPCWebSocketHandler(wsConn, done)
				break
			}
			remoteWSManager, _ := types.GetWSRemoteConnection(wsConn.ID)
			if remoteWSManager != nil {
				logger.LogicLogger.Debug("[Handler] Received WebSocket remote ws stream")
				go RemoteWebSocketHandler(wsConn, done)
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		// Listen for connection closed
		<-done
		logger.LogicLogger.Info("[Handler] WebSocket connection closed", "connID", wsConn.ID)
	}
}

// Handle WebSocket messages without distinguishing between text or binary, just setting different Content-Types based on contentType
func handleWebSocketMessage(wsConn *client.WebSocketConnection, flavor, service string, message []byte, originalRequest *http.Request, contentType string) {
	logger.LogicLogger.Debug("[WebSocket] Processing message",
		"connID", wsConn.ID,
		"flavor", flavor,
		"service", service,
		"contentType", contentType,
		"length", len(message),
		"managerInstanceAddr", fmt.Sprintf("%p", client.GetGlobalWebSocketManager()))

	// Clone the original request to preserve key information
	clonedReq := originalRequest.Clone(originalRequest.Context())

	// Check if it is binary data
	isBinary := strings.Contains(contentType, "application/octet-stream") ||
		strings.Contains(contentType, "audio/")

	// Check if it is a JSON message, and if so, try to resolve it to an Action structure
	var actionMsg types.WebSocketActionMessage
	var updatedMessage []byte
	taskType := types.WSSTTTaskTypeUnknown

	if !isBinary && json.Unmarshal(message, &actionMsg) == nil {
		taskType = actionMsg.Action

		// Save task information to WebSocket connection
		sttParams := wsConn.GetSTTParams()
		sttParams.Action = actionMsg.Action
		if actionMsg.Parameters != nil {
			if actionMsg.Parameters.Format != "" {
				sttParams.AudioFormat = actionMsg.Parameters.Format
			}
			if actionMsg.Parameters.SampleRate > 0 {
				sttParams.SampleRate = actionMsg.Parameters.SampleRate
			}
			if actionMsg.Parameters.Language != "" {
				sttParams.Language = actionMsg.Parameters.Language
			}
			sttParams.UseVAD = actionMsg.Parameters.UseVAD
			if actionMsg.Parameters.ReturnFormat != "" {
				sttParams.ReturnFormat = actionMsg.Parameters.ReturnFormat
			}
		}

		if actionMsg.Model != "" {
			sttParams.Model = actionMsg.Model
		}

		logger.LogicLogger.Debug("[WebSocket] Updated STT parameters in connection",
			"connID", wsConn.ID,
			"format", sttParams.AudioFormat,
			"sampleRate", sttParams.SampleRate,
			"language", sttParams.Language,
			"useVAD", sttParams.UseVAD,
			"model", sttParams.Model,
			"returnFormat", sttParams.ReturnFormat)

		// Tag task started
		wsConn.SetConnectionTaskStatus(true, time.Now().Unix())

		// Directly set the WebSocket connection ID to the struct and reserialize it
		actionMsg.TaskID = wsConn.ID
		updatedMessage, _ = json.Marshal(actionMsg)

		logger.LogicLogger.Debug("[WebSocket] Updated message with connection ID",
			"connID", wsConn.ID,
			"action", actionMsg.Action)
	} else if isBinary {
		taskType = types.WSTaskTypeAudio
		updatedMessage = message

		logger.LogicLogger.Debug("[WebSocket] Processing binary data",
			"connID", wsConn.ID,
			"size", len(message))
	} else {
		// Unrecognized message format
		logger.LogicLogger.Warn("[WebSocket] Unrecognized message format",
			"connID", wsConn.ID)

		// Send error event
		errorEvent := types.NewTaskFailedEvent(wsConn.ID,
			types.WSErrorCodeClientError,
			bcode.ErrWebSocketMessageFormat.Error())
		errorJSON, _ := json.Marshal(errorEvent)
		wsConn.WriteMessage(websocket.TextMessage, errorJSON)

		return
	}

	// Replace request body with new message content
	clonedReq.Body = io.NopCloser(bytes.NewReader(updatedMessage))
	clonedReq.ContentLength = int64(len(updatedMessage))

	// Set Content-Type
	clonedReq.Header.Set("Content-Type", contentType)

	// Add WebSocket identifier for service to distinguish WebSocket messages
	clonedReq.Header.Set("X-WebSocket-ConnID", wsConn.ID)

	logger.LogicLogger.Debug("[WebSocket] Prepared HTTP request with WebSocket connection ID",
		"connID", wsConn.ID,
		"contentType", contentType,
		"contentLength", clonedReq.ContentLength)

	// Call service to process message
	msgTaskID, msgCh, err := InvokeService(flavor, service, clonedReq)
	if err != nil {
		logger.LogicLogger.Error("[WebSocket] Failed to invoke service for message",
			"error", err,
			"connID", wsConn.ID)

		// Send an error message to the client side
		errorEvent := types.NewTaskFailedEvent(wsConn.ID,
			types.WSErrorCodeServerError,
			err.Error())
		errorJSON, _ := json.Marshal(errorEvent)
		wsConn.WriteMessage(websocket.TextMessage, errorJSON)
		return
	}

	// Bind and write taskType and msgTaskID to wsConn
	wsConn.SetTaskType(msgTaskID, taskType)

	// If not a final task, add it to the active task list
	if taskType != types.WSActionFinishTask {
		wsConn.AddActiveTask(msgTaskID)
	}

	logger.LogicLogger.Debug("[WebSocket] Message processing started",
		"connID", wsConn.ID,
		"msgTaskID", msgTaskID,
		"connectionTaskID", wsConn.TaskID,
		"taskType", taskType,
		"activeTaskCount", wsConn.GetActiveTaskCount())

	// Start goroutine to process service results and send to client side
	go processServiceResult(wsConn, msgCh, msgTaskID)
}

// Process service results and send to WebSocket client side
func processServiceResult(wsConn *client.WebSocketConnection, ch chan *types.ServiceResult, msgTaskID uint64) {
	// Get task type
	taskType := wsConn.GetTaskType(msgTaskID)
	logger.LogicLogger.Debug("[WebSocket] Processing service result",
		"msgTaskID", msgTaskID,
		"connID", wsConn.ID,
		"taskType", taskType)

	// Make sure to clean up active tasks when a function exits
	defer func() {
		if taskType != types.WSActionFinishTask {
			wsConn.RemoveActiveTask(msgTaskID)
			logger.LogicLogger.Debug("[WebSocket] Task removed from active tasks",
				"msgTaskID", msgTaskID,
				"connID", wsConn.ID,
				"activeTaskCount", wsConn.GetActiveTaskCount())
		}
	}()

	for {
		data, ok := <-ch
		if !ok {
			logger.LogicLogger.Debug("[WebSocket] Message task channel closed", "taskID", msgTaskID, "connID", wsConn.ID)
			return
		}

		logger.LogicLogger.Debug("[WebSocket] Received message task result", "result", data, "connID", wsConn.ID)

		// First send the task start event
		if taskType == types.WSActionRunTask {
			startedEvent := types.NewTaskStartedEvent(wsConn.ID)
			startedJSON, _ := json.Marshal(startedEvent)
			if err := wsConn.WriteMessage(websocket.TextMessage, startedJSON); err != nil {
				logger.LogicLogger.Error("[WebSocket] Failed to send start event",
					"error", err,
					"connID", wsConn.ID)
			}
		}

		if data.Type == types.ServiceResultFailed {
			// Handle error
			errorMsg := "Service processing failed"
			if data.Error != nil {
				errorMsg = data.Error.Error()
			}

			// Create and send error events
			errorEvent := types.NewTaskFailedEvent(wsConn.ID,
				types.WSErrorCodeModelError,
				errorMsg)
			errorJSON, _ := json.Marshal(errorEvent)

			if err := wsConn.WriteMessage(websocket.TextMessage, errorJSON); err != nil {
				logger.LogicLogger.Error("[WebSocket] Failed to send error message",
					"error", err,
					"connID", wsConn.ID)
			}
			return
		}
	}
}

func ConvertBetweenFlavors(from, to APIFlavor, service string, conv string, content types.HTTPContent, ctx convert.ConvertContext) (types.HTTPContent, error) {
	if from.Name() == to.Name() {
		return content, nil
	}

	// If it's binary data, skip conversion
	if content.Body != nil && content.Header.Get("Content-Type") == "application/octet-stream" {
		logger.LogicLogger.Debug("[Flavor] Skipping conversion for binary data", "flavor", from.Name(), "service", service, "conversion", conv)
		return content, nil
	}

	// need conversion, content-length may change
	content.Header.Del("Content-Length")

	firstConv := conv + "_to_aog"
	secondConv := conv + "_from_aog"
	EnsureConversionNameValid(firstConv)
	EnsureConversionNameValid(secondConv)
	if from.Name() != types.FlavorAOG {
		var err error
		content, err = from.Convert(service, firstConv, content, ctx)
		if err != nil {
			return types.HTTPContent{}, bcode.WrapError(bcode.ErrFlavorConvertRequest, err)
		}
	}
	if from.Name() != types.FlavorAOG && to.Name() != types.FlavorAOG {
		if strings.HasPrefix(conv, "request") {
			logger.LogicLogger.Error("request_converted_to_aog", "<n/a>", "<n/a>", content.Header, content.Body)
		} else {
			logger.LogicLogger.Error("response_converted_to_aog", -1, content.Header, content.Body)
		}
	}
	if to.Name() != types.FlavorAOG {
		var err error
		content, err = to.Convert(service, secondConv, content, ctx)
		if err != nil {
			return types.HTTPContent{}, bcode.WrapError(bcode.ErrFlavorConvertResponse, err)
		}
	}
	return content, nil
}

type ServiceDefaultInfo struct {
	Endpoints       []string `json:"endpoints"`
	DefaultModel    string   `json:"default_model"`
	RequestUrl      string   `json:"url"`
	RequestExtraUrl string   `json:"request_extra_url"`
	AuthType        string   `json:"auth_type"`
	RequestSegments int      `json:"request_segments"`
	ExtraHeaders    string   `json:"extra_headers"`
	SupportModels   []string `json:"support_models"`
	AuthApplyUrl    string   `json:"auth_apply_url"`
	Protocol        string   `json:"protocol"`
	TaskType        string   `json:"task_type"`
	ExposeProtocol  string   `json:"expose_protocol"`
}

var FlavorServiceDefaultInfoMap = make(map[string]map[string]ServiceDefaultInfo)

func InitProviderDefaultModelTemplate(flavor APIFlavor) {
	def, err := LoadFlavorDef(flavor.Name())
	if err != nil {
		logger.LogicLogger.Error("[Provider]Failed to load file", "provider_name", flavor, "error", err.Error())
	}
	ServiceDefaultInfoMap := make(map[string]ServiceDefaultInfo)
	for service, serviceDef := range def.Services {
		ServiceDefaultInfoMap[service] = ServiceDefaultInfo{
			Endpoints:       serviceDef.Endpoints,
			DefaultModel:    serviceDef.DefaultModel,
			RequestUrl:      serviceDef.RequestUrl,
			RequestExtraUrl: serviceDef.RequestExtraUrl,
			RequestSegments: serviceDef.RequestSegments,
			AuthType:        serviceDef.AuthType,
			ExtraHeaders:    serviceDef.ExtraHeaders,
			SupportModels:   serviceDef.SupportModels,
			AuthApplyUrl:    serviceDef.AuthApplyUrl,
			ExposeProtocol:  serviceDef.ExposeProtocol,
			TaskType:        serviceDef.TaskType,
		}
	}
	FlavorServiceDefaultInfoMap[flavor.Name()] = ServiceDefaultInfoMap
}

func GetProviderServiceDefaultInfo(flavor string, service string) ServiceDefaultInfo {
	serviceDefaultInfo := FlavorServiceDefaultInfoMap[flavor][service]
	return serviceDefaultInfo
}

func GetProviderServices(flavor string) map[string]ServiceDefaultInfo {
	return FlavorServiceDefaultInfoMap[flavor]
}

type SignParams struct {
	SecretId      string           `json:"secret_id"`
	SecretKey     string           `json:"secret_key"`
	RequestBody   string           `json:"request_body"`
	RequestUrl    string           `json:"request_url"`
	RequestMethod string           `json:"request_method"`
	RequestHeader http.Header      `json:"request_header"`
	CommonParams  SignCommonParams `json:"common_params"`
}

type SignCommonParams struct {
	Version string `json:"version"`
	Action  string `json:"action"`
	Region  string `json:"region"`
}

func TencentSignGenerate(p SignParams, req http.Request) error {
	secretId := p.SecretId
	secretKey := p.SecretKey
	parseUrl, err := url.Parse(p.RequestUrl)
	if err != nil {
		return bcode.WrapError(bcode.ErrParameterValidation, err)
	}
	host := parseUrl.Host
	service := strings.Split(host, ".")[0]
	algorithm := "TC3-HMAC-SHA256"
	tcVersion := p.CommonParams.Version
	action := p.CommonParams.Action
	region := p.CommonParams.Region
	timestamp := time.Now().Unix()

	// step 1: build canonical request string
	httpRequestMethod := p.RequestMethod
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := ""
	signedHeaders := ""
	for k, v := range p.RequestHeader {
		if strings.ToLower(k) == "content-type" {
			signedHeaders += fmt.Sprintf("%s;", strings.ToLower(k))
			canonicalHeaders += fmt.Sprintf("%s:%s\n", strings.ToLower(k), strings.ToLower(v[0]))
		}
	}
	signedHeaders += "host"
	canonicalHeaders += fmt.Sprintf("%s:%s\n", "host", host)
	signedHeaders = strings.TrimRight(signedHeaders, ";")
	payload := p.RequestBody
	hashedRequestPayload := utils.Sha256hex(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	// step 2: build string to sign
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := utils.Sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm,
		timestamp,
		credentialScope,
		hashedCanonicalRequest)

	// step 3: sign string
	secretDate := utils.HmacSha256(date, "TC3"+secretKey)
	secretService := utils.HmacSha256(service, secretDate)
	secretSigning := utils.HmacSha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(utils.HmacSha256(string2sign, secretSigning)))

	// step 4: build authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secretId,
		credentialScope,
		signedHeaders,
		signature)

	req.Header.Add("Authorization", authorization)
	req.Header.Add("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Add("X-TC-Version", tcVersion)
	req.Header.Add("X-TC-Region", region)
	req.Header.Add("X-TC-Action", action)
	return nil
}

type SignAuthInfo struct {
	SecretId  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
}

type ApiKeyAuthInfo struct {
	ApiKey string `json:"api_key"`
}

type Authenticator interface {
	Authenticate() error
}

type APIKEYAuthenticator struct {
	AuthInfo string `json:"auth_info"`
	Req      http.Request
}

type TencentSignAuthenticator struct {
	AuthInfo     string                `json:"auth_info"`
	Req          http.Request          `json:"request"`
	ProviderInfo types.ServiceProvider `json:"provider_info"`
	ReqBody      string                `json:"req_body"`
}

func (a *APIKEYAuthenticator) Authenticate() error {
	var authInfoData ApiKeyAuthInfo
	err := json.Unmarshal([]byte(a.AuthInfo), &authInfoData)
	if err != nil {
		return bcode.WrapError(bcode.ErrAuthInfoParsing, err)
	}
	a.Req.Header.Set("Authorization", "Bearer "+authInfoData.ApiKey)
	return nil
}

func (s *TencentSignAuthenticator) Authenticate() error {
	var authInfoData SignAuthInfo
	err := json.Unmarshal([]byte(s.AuthInfo), &authInfoData)
	if err != nil {
		return bcode.WrapError(bcode.ErrAuthInfoParsing, err)
	}

	commonParams := SignParams{
		SecretId:      authInfoData.SecretId,
		SecretKey:     authInfoData.SecretKey,
		RequestUrl:    s.ProviderInfo.URL,
		RequestBody:   s.ReqBody,
		RequestHeader: s.Req.Header,
		RequestMethod: s.Req.Method,
	}
	if s.ProviderInfo.ExtraHeaders != "" {
		var serviceExtraInfo SignCommonParams
		err := json.Unmarshal([]byte(s.ProviderInfo.ExtraHeaders), &serviceExtraInfo)
		if err != nil {
			return bcode.WrapError(bcode.ErrAuthInfoParsing, err)
		}
		commonParams.CommonParams = serviceExtraInfo
	}

	err = TencentSignGenerate(commonParams, s.Req)
	if err != nil {
		return bcode.WrapError(bcode.ErrAuthenticationFailed, err)
	}
	return nil
}

type AuthenticatorParams struct {
	Request      *http.Request
	ProviderInfo *types.ServiceProvider
	RequestBody  string
}

func ChooseProviderAuthenticator(p *AuthenticatorParams) Authenticator {
	var authenticator Authenticator
	if p.ProviderInfo.AuthType == types.AuthTypeToken {
		switch p.ProviderInfo.Flavor {
		case types.FlavorTencent:
			authenticator = &TencentSignAuthenticator{
				Req:          *p.Request,
				AuthInfo:     p.ProviderInfo.AuthKey,
				ProviderInfo: *p.ProviderInfo,
				ReqBody:      p.RequestBody,
			}
		}
	} else if p.ProviderInfo.AuthType == types.AuthTypeApiKey {
		authenticator = &APIKEYAuthenticator{
			AuthInfo: p.ProviderInfo.AuthKey,
			Req:      *p.Request,
		}
	}
	return authenticator
}

// Function to monitor stream responses
func monitorStreamResponses(session *client.GRPCStreamSession, wsConn *client.WebSocketConnection) {
	grpcStreamManager := client.GetGlobalGRPCStreamManager()

	for {
		// Check if session is still valid (we no longer access internal mutex and Active fields, but check through public methods)
		if grpcStreamManager.GetSessionByWSConnID(wsConn.ID) == nil {
			logger.LogicLogger.Info("[Handler] gRPC session no longer active, stopping monitoring",
				"connID", wsConn.ID)
			return
		}

		// Receive response (this is a blocking call)
		streamResponse, err := session.Stream.Recv()
		// Handle receive errors
		if err != nil {
			if err == io.EOF {
				logger.LogicLogger.Info("[Handler] gRPC stream closed by server",
					"connID", wsConn.ID)
			} else {
				logger.LogicLogger.Error("[Handler] Error receiving from gRPC stream",
					"error", err,
					"connID", wsConn.ID)

				// Send error event to client
				errorEvent := types.NewTaskFailedEvent(wsConn.ID,
					types.WSErrorCodeModelError,
					bcode.WrapError(bcode.ErrGRPCStreamReceive, err).Error())
				errorJSON, _ := json.Marshal(errorEvent)

				wsConn.WriteMessage(websocket.TextMessage, errorJSON)
			}

			// Close session
			grpcStreamManager.CloseSessionByWSConnID(wsConn.ID)

			return
		}

		// Handle received response
		if streamResponse != nil {
			// Check if there are error messages
			if streamResponse.ErrorMessage != "" {
				logger.LogicLogger.Error("[Handler] Server returned error",
					"error", streamResponse.ErrorMessage,
					"connID", wsConn.ID)

				// Send error event to client
				errorEvent := types.NewTaskFailedEvent(wsConn.ID,
					types.WSErrorCodeModelError,
					streamResponse.ErrorMessage)
				errorJSON, _ := json.Marshal(errorEvent)

				wsConn.WriteMessage(websocket.TextMessage, errorJSON)
				continue
			}

			// Get inference response
			inferResp := streamResponse.GetInferResponse()
			if inferResp == nil {
				continue
			}

			// Parse JSON response
			if len(inferResp.RawOutputContents) > 0 {
				// Parse RawOutputContents as JSON
				rawContent := string(inferResp.RawOutputContents[0])

				var resultData struct {
					Status  string `json:"status"`
					IsFinal bool   `json:"is_final"`
					Content string `json:"content"`
					Message string `json:"message"`
				}

				err := json.Unmarshal([]byte(rawContent), &resultData)
				if err != nil {
					logger.LogicLogger.Error("[Handler] Failed to parse inference result JSON",
						"error", err,
						"raw", rawContent,
						"connID", wsConn.ID)

					// Send error event to client
					errorEvent := types.NewTaskFailedEvent(wsConn.ID,
						types.WSErrorCodeModelError,
						bcode.WrapError(bcode.ErrJSONParsing, err).Error())
					errorJSON, _ := json.Marshal(errorEvent)
					wsConn.WriteMessage(websocket.TextMessage, errorJSON)
					continue
				}

				// Use parsed content
				text := resultData.Content
				statusInfo := resultData.Status
				isFinal := resultData.IsFinal
				messageInfo := resultData.Message

				// Log status information
				logger.LogicLogger.Debug("[Handler] Received inference result",
					"status", statusInfo,
					"is_final", isFinal,
					"message", messageInfo,
					"connID", wsConn.ID)

				// If content is in SRT format, parse timestamps
				var beginTime, endTime *int
				if text != "" {
					beginTime, endTime = utils.ParseSRTTimestamps(text)

					logger.LogicLogger.Debug("[Handler] Parsed SRT timestamps",
						"beginTime", beginTime,
						"endTime", endTime,
						"connID", wsConn.ID)

					resultEvent := types.NewResultGeneratedEvent(wsConn.ID, beginTime, endTime, text)
					resultJSON, _ := json.Marshal(resultEvent)

					// Send result to WebSocket client
					if err := wsConn.WriteMessage(websocket.TextMessage, resultJSON); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to send result to WebSocket",
							"error", err,
							"connID", wsConn.ID)
					} else {
						logger.LogicLogger.Debug("[Handler] Sent recognition result to WebSocket",
							"text", text,
							"connID", wsConn.ID)
					}
				}

				if isFinal {
					finishEvent := types.NewTaskFinishedEvent(wsConn.ID)
					finishJSON, _ := json.Marshal(finishEvent)
					if err := wsConn.WriteMessage(websocket.TextMessage, finishJSON); err != nil {
						logger.LogicLogger.Error("[Handler] Failed to send finish event to WebSocket",
							"error", err,
							"connID", wsConn.ID)
					} else {
						logger.LogicLogger.Debug("[Handler] Sent finish event to WebSocket",
							"connID", wsConn.ID)
					}
				}
			}
		}
	}
}
