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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/protocol"
	"github.com/intel/aog/plugin-sdk/types"
)

// Use unified interfaces defined in the client package.
// This ensures plugins and clients use the same interface definitions to avoid type assertion failures.
type (
	PluginProvider       = client.PluginProvider
	LocalPluginProvider  = client.LocalPluginProvider
	RemotePluginProvider = client.RemotePluginProvider
)

// GRPCProviderServer is the gRPC server implementation on the plugin side.
type GRPCProviderServer struct {
	protocol.UnimplementedProviderServiceServer

	BasePlugin   PluginProvider
	LocalPlugin  LocalPluginProvider
	RemotePlugin RemotePluginProvider
}

// NewGRPCProviderServer creates a gRPC server.
//
// The parameter can be any type that implements PluginProvider-related interfaces.
func NewGRPCProviderServer(impl interface{}) *GRPCProviderServer {
	server := &GRPCProviderServer{}

	debugFile, _ := os.OpenFile("/tmp/aog_sdk_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if debugFile != nil {
		defer debugFile.Close()
		fmt.Fprintf(debugFile, "[SDK-DEBUG] NewGRPCProviderServer called: impl type = %T\n", impl)
	}

	if basePlugin, ok := impl.(PluginProvider); ok {
		server.BasePlugin = basePlugin
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ✅ BasePlugin assertion SUCCESS\n")
		}
	} else {
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ❌ BasePlugin assertion FAILED\n")
		}
	}

	if localPlugin, ok := impl.(LocalPluginProvider); ok {
		server.LocalPlugin = localPlugin
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ✅ LocalPlugin assertion SUCCESS\n")
		}
	} else {
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ❌ LocalPlugin assertion FAILED - THIS IS THE PROBLEM!\n")
		}
	}

	if remotePlugin, ok := impl.(RemotePluginProvider); ok {
		server.RemotePlugin = remotePlugin
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ✅ RemotePlugin assertion SUCCESS\n")
		}
	} else {
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[SDK-DEBUG] ℹ️  RemotePlugin assertion failed (expected for local plugins)\n")
		}
	}

	return server
}

func createSuccessResponse() (int32, string) {
	return 0, "success"
}

func createErrorResponse(err error) (int32, string) {
	if err == nil {
		return 0, "success"
	}

	if pluginErr, ok := err.(*types.PluginError); ok {
		return int32(pluginErr.Code), pluginErr.Error()
	}

	return int32(types.ErrCodeInternal), err.Error()
}

// GetManifest returns the plugin metadata.
func (s *GRPCProviderServer) GetManifest(ctx context.Context, req *protocol.GetManifestRequest) (*protocol.GetManifestResponse, error) {
	if s.BasePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("plugin not initialized"))
		return &protocol.GetManifestResponse{Code: code, Message: message}, nil
	}

	manifest := s.BasePlugin.GetManifest()
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetManifestResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.GetManifestResponse{
		Code:         code,
		Message:      message,
		ManifestJson: manifestJSON,
	}, nil
}

// InvokeService invokes a plugin service.
func (s *GRPCProviderServer) InvokeService(ctx context.Context, req *protocol.InvokeServiceRequest) (*protocol.InvokeServiceResponse, error) {
	if s.BasePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("plugin not initialized"))
		return &protocol.InvokeServiceResponse{Code: code, Message: message}, nil
	}

	respData, err := s.BasePlugin.InvokeService(ctx, req.ServiceName, req.AuthInfo, req.RequestData)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.InvokeServiceResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.InvokeServiceResponse{
		Code:         code,
		Message:      message,
		ResponseData: respData,
	}, nil
}

// HealthCheck performs a health check.
func (s *GRPCProviderServer) HealthCheck(ctx context.Context, req *protocol.HealthCheckRequest) (*protocol.HealthCheckResponse, error) {
	if s.BasePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("plugin not initialized"))
		return &protocol.HealthCheckResponse{Code: code, Message: message}, nil
	}

	err := s.BasePlugin.HealthCheck(ctx)
	code, message := createErrorResponse(err)
	return &protocol.HealthCheckResponse{Code: code, Message: message}, nil
}

// GetOperateStatus returns the operational status.
func (s *GRPCProviderServer) GetOperateStatus(ctx context.Context, req *protocol.GetOperateStatusRequest) (*protocol.GetOperateStatusResponse, error) {
	if s.BasePlugin == nil {
		return &protocol.GetOperateStatusResponse{Status: 0}, nil
	}

	status := s.BasePlugin.GetOperateStatus()
	return &protocol.GetOperateStatusResponse{Status: int32(status)}, nil
}

// SetOperateStatus sets the operational status.
func (s *GRPCProviderServer) SetOperateStatus(ctx context.Context, req *protocol.SetOperateStatusRequest) (*protocol.SetOperateStatusResponse, error) {
	if s.BasePlugin != nil {
		s.BasePlugin.SetOperateStatus(int(req.Status))
	}
	return &protocol.SetOperateStatusResponse{}, nil
}

// SetAuth sets authentication information (Remote plugin only).
func (s *GRPCProviderServer) SetAuth(ctx context.Context, req *protocol.SetAuthRequest) (*protocol.SetAuthResponse, error) {
	if s.RemotePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a remote plugin"))
		return &protocol.SetAuthResponse{Code: code, Message: message}, nil
	}
	// err := s.RemotePlugin.SetAuth(req.AuthType, req.Credentials)
	// code, message := createErrorResponse(err)
	return &protocol.SetAuthResponse{}, nil
}

// ValidateAuth validates authentication information (Remote plugin only).
func (s *GRPCProviderServer) ValidateAuth(ctx context.Context, req *protocol.ValidateAuthRequest) (*protocol.ValidateAuthResponse, error) {
	if s.RemotePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a remote plugin"))
		return &protocol.ValidateAuthResponse{Code: code, Message: message}, nil
	}

	err := s.RemotePlugin.ValidateAuth(ctx)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.ValidateAuthResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.ValidateAuthResponse{Code: code, Message: message}, nil
}

// RefreshAuth refreshes authentication information (Remote plugin only).
func (s *GRPCProviderServer) RefreshAuth(ctx context.Context, req *protocol.RefreshAuthRequest) (*protocol.RefreshAuthResponse, error) {
	if s.RemotePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a remote plugin"))
		return &protocol.RefreshAuthResponse{Code: code, Message: message}, nil
	}

	err := s.RemotePlugin.RefreshAuth(ctx)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.RefreshAuthResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.RefreshAuthResponse{Code: code, Message: message}, nil
}

// StartEngine starts the engine (Local plugin only).
// ===== Local Plugin-Specific Methods =====

// StartEngine starts the engine (Local plugin only).
func (s *GRPCProviderServer) StartEngine(ctx context.Context, req *protocol.StartEngineRequest) (*protocol.StartEngineResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.StartEngineResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.StartEngine(req.Mode)
	code, message := createErrorResponse(err)
	return &protocol.StartEngineResponse{Code: code, Message: message}, nil
}

// StopEngine stops the engine (Local plugin only).
func (s *GRPCProviderServer) StopEngine(ctx context.Context, req *protocol.StopEngineRequest) (*protocol.StopEngineResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.StopEngineResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.StopEngine()
	code, message := createErrorResponse(err)
	return &protocol.StopEngineResponse{Code: code, Message: message}, nil
}

// GetConfig returns the engine configuration (Local plugin only).
func (s *GRPCProviderServer) GetConfig(ctx context.Context, req *protocol.GetConfigRequest) (*protocol.GetConfigResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.GetConfigResponse{Code: code, Message: message}, nil
	}

	config, err := s.LocalPlugin.GetConfig(ctx)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetConfigResponse{Code: code, Message: message}, nil
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetConfigResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.GetConfigResponse{
		Code:       code,
		Message:    message,
		ConfigJson: configJSON,
	}, nil
}

// CheckEngine checks if the engine is installed (Local plugin only).
func (s *GRPCProviderServer) CheckEngine(ctx context.Context, req *protocol.CheckEngineRequest) (*protocol.CheckEngineResponse, error) {
	debugFile, _ := os.OpenFile("/tmp/aog_sdk_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if debugFile != nil {
		defer debugFile.Close()
		fmt.Fprintf(debugFile, "[CheckEngine] Called! LocalPlugin == nil: %v\n", s.LocalPlugin == nil)
		fmt.Fprintf(debugFile, "[CheckEngine] BasePlugin == nil: %v\n", s.BasePlugin == nil)
		fmt.Fprintf(debugFile, "[CheckEngine] RemotePlugin == nil: %v\n", s.RemotePlugin == nil)
	}

	if s.LocalPlugin == nil {
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[CheckEngine] ERROR: LocalPlugin is nil, returning error\n")
		}
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.CheckEngineResponse{Code: code, Message: message, Installed: false}, nil
	}

	if debugFile != nil {
		fmt.Fprintf(debugFile, "[CheckEngine] LocalPlugin is set, calling CheckEngine()\n")
	}
	installed, err := s.LocalPlugin.CheckEngine()
	if err != nil {
		if debugFile != nil {
			fmt.Fprintf(debugFile, "[CheckEngine] CheckEngine() returned error: %v\n", err)
		}
		code, message := createErrorResponse(err)
		return &protocol.CheckEngineResponse{Code: code, Message: message, Installed: false}, nil
	}

	if debugFile != nil {
		fmt.Fprintf(debugFile, "[CheckEngine] Success! Installed: %v\n", installed)
	}
	code, message := createSuccessResponse()
	return &protocol.CheckEngineResponse{
		Code:      code,
		Message:   message,
		Installed: installed,
	}, nil
}

// InstallEngine installs the engine (Local plugin only).
func (s *GRPCProviderServer) InstallEngine(ctx context.Context, req *protocol.InstallEngineRequest) (*protocol.InstallEngineResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.InstallEngineResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.InstallEngine(ctx)
	code, message := createErrorResponse(err)
	return &protocol.InstallEngineResponse{Code: code, Message: message}, nil
}

// InitEnv initializes environment variables (Local plugin only).
func (s *GRPCProviderServer) InitEnv(ctx context.Context, req *protocol.InitEnvRequest) (*protocol.InitEnvResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.InitEnvResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.InitEnv()
	code, message := createErrorResponse(err)
	return &protocol.InitEnvResponse{Code: code, Message: message}, nil
}

// UpgradeEngine upgrades the engine (Local plugin only).
func (s *GRPCProviderServer) UpgradeEngine(ctx context.Context, req *protocol.UpgradeEngineRequest) (*protocol.UpgradeEngineResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.UpgradeEngineResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.UpgradeEngine(ctx)
	code, message := createErrorResponse(err)
	return &protocol.UpgradeEngineResponse{Code: code, Message: message}, nil
}

// PullModel pulls a model (Local plugin only).
func (s *GRPCProviderServer) PullModel(ctx context.Context, req *protocol.PullModelRequest) (*protocol.PullModelResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.PullModelResponse{Code: code, Message: message}, nil
	}

	var pullReq types.PullModelRequest
	if err := json.Unmarshal(req.RequestJson, &pullReq); err != nil {
		code, message := createErrorResponse(err)
		return &protocol.PullModelResponse{Code: code, Message: message}, nil
	}

	resp, err := s.LocalPlugin.PullModel(ctx, &pullReq, nil)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.PullModelResponse{Code: code, Message: message}, nil
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.PullModelResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.PullModelResponse{
		Code:         code,
		Message:      message,
		ResponseJson: respJSON,
	}, nil
}

// DeleteModel deletes a model (Local plugin only).
func (s *GRPCProviderServer) DeleteModel(ctx context.Context, req *protocol.DeleteModelRequest) (*protocol.DeleteModelResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.DeleteModelResponse{Code: code, Message: message}, nil
	}

	var deleteReq types.DeleteModelRequest
	if err := json.Unmarshal(req.RequestJson, &deleteReq); err != nil {
		code, message := createErrorResponse(err)
		return &protocol.DeleteModelResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.DeleteModel(ctx, &deleteReq)
	code, message := createErrorResponse(err)
	return &protocol.DeleteModelResponse{Code: code, Message: message}, nil
}

// ListModels lists all models (Local plugin only).
func (s *GRPCProviderServer) ListModels(ctx context.Context, req *protocol.ListModelsRequest) (*protocol.ListModelsResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.ListModelsResponse{Code: code, Message: message}, nil
	}

	resp, err := s.LocalPlugin.ListModels(ctx)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.ListModelsResponse{Code: code, Message: message}, nil
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.ListModelsResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.ListModelsResponse{
		Code:         code,
		Message:      message,
		ResponseJson: respJSON,
	}, nil
}

// LoadModel loads a model into memory (Local plugin only).
func (s *GRPCProviderServer) LoadModel(ctx context.Context, req *protocol.LoadModelRequest) (*protocol.LoadModelResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.LoadModelResponse{Code: code, Message: message}, nil
	}

	var loadReq types.LoadModelRequest
	if err := json.Unmarshal(req.RequestJson, &loadReq); err != nil {
		code, message := createErrorResponse(err)
		return &protocol.LoadModelResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.LoadModel(ctx, &loadReq)
	code, message := createErrorResponse(err)
	return &protocol.LoadModelResponse{Code: code, Message: message}, nil
}

// UnloadModel unloads models from memory (Local plugin only).
func (s *GRPCProviderServer) UnloadModel(ctx context.Context, req *protocol.UnloadModelRequest) (*protocol.UnloadModelResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.UnloadModelResponse{Code: code, Message: message}, nil
	}

	var unloadReq types.UnloadModelRequest
	if err := json.Unmarshal(req.RequestJson, &unloadReq); err != nil {
		code, message := createErrorResponse(err)
		return &protocol.UnloadModelResponse{Code: code, Message: message}, nil
	}

	err := s.LocalPlugin.UnloadModel(ctx, &unloadReq)
	code, message := createErrorResponse(err)
	return &protocol.UnloadModelResponse{Code: code, Message: message}, nil
}

// GetRunningModels returns all running models (Local plugin only).
func (s *GRPCProviderServer) GetRunningModels(ctx context.Context, req *protocol.GetRunningModelsRequest) (*protocol.GetRunningModelsResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.GetRunningModelsResponse{Code: code, Message: message}, nil
	}

	resp, err := s.LocalPlugin.GetRunningModels(ctx)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetRunningModelsResponse{Code: code, Message: message}, nil
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetRunningModelsResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.GetRunningModelsResponse{
		Code:         code,
		Message:      message,
		ResponseJson: respJSON,
	}, nil
}

// GetVersion returns the engine version (Local plugin only).
func (s *GRPCProviderServer) GetVersion(ctx context.Context, req *protocol.GetVersionRequest) (*protocol.GetVersionResponse, error) {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return &protocol.GetVersionResponse{Code: code, Message: message}, nil
	}

	var versionReq types.EngineVersionResponse
	if len(req.RequestJson) > 0 {
		if err := json.Unmarshal(req.RequestJson, &versionReq); err != nil {
			code, message := createErrorResponse(err)
			return &protocol.GetVersionResponse{Code: code, Message: message}, nil
		}
	}

	resp, err := s.LocalPlugin.GetVersion(ctx, &versionReq)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetVersionResponse{Code: code, Message: message}, nil
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		code, message := createErrorResponse(err)
		return &protocol.GetVersionResponse{Code: code, Message: message}, nil
	}

	code, message := createSuccessResponse()
	return &protocol.GetVersionResponse{
		Code:         code,
		Message:      message,
		ResponseJson: respJSON,
	}, nil
}

// InvokeServiceStream performs server-side streaming invocation.
func (s *GRPCProviderServer) InvokeServiceStream(req *protocol.InvokeServiceRequest, stream protocol.ProviderService_InvokeServiceStreamServer) error {
	if s.BasePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("plugin not initialized"))
		return stream.Send(&protocol.InvokeServiceStreamResponse{
			Code:    code,
			Message: message,
			IsFinal: true,
		})
	}

	streamablePlugin, ok := s.BasePlugin.(client.StreamablePlugin)
	if !ok {
		code, message := createErrorResponse(fmt.Errorf("plugin does not support streaming"))
		return stream.Send(&protocol.InvokeServiceStreamResponse{
			Code:    code,
			Message: message,
			IsFinal: true,
		})
	}

	// Call plugin's streaming method
	chunkChan, err := streamablePlugin.InvokeServiceStream(stream.Context(), req.ServiceName, req.AuthInfo, req.RequestData)
	if err != nil {
		code, message := createErrorResponse(err)
		return stream.Send(&protocol.InvokeServiceStreamResponse{
			Code:    code,
			Message: message,
			IsFinal: true,
		})
	}

	// Forward streaming data
	for {
		select {
		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel closed, normal completion
				return nil
			}

			// Check for errors
			if chunk.Error != nil {
				code, message := createErrorResponse(chunk.Error)
				return stream.Send(&protocol.InvokeServiceStreamResponse{
					Code:    code,
					Message: message,
					IsFinal: true,
				})
			}

			// Send data chunk
			if err := stream.Send(&protocol.InvokeServiceStreamResponse{
				Code:      0,
				Message:   "success",
				ChunkData: chunk.Data,
				IsFinal:   chunk.IsFinal,
				Metadata:  chunk.Metadata,
			}); err != nil {
				return err
			}

			// If it's the last chunk, finish normally
			if chunk.IsFinal {
				return nil
			}

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// InvokeServiceBidi handles bidirectional streaming invocation
func (s *GRPCProviderServer) InvokeServiceBidi(stream protocol.ProviderService_InvokeServiceBidiServer) error {
	if s.BasePlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("plugin not initialized"))
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	bidiPlugin, ok := s.BasePlugin.(client.BidirectionalPlugin)
	if !ok {
		code, message := createErrorResponse(fmt.Errorf("plugin does not implement bidirectional interface"))
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	firstReq, err := stream.Recv()
	if err != nil {
		code, message := createErrorResponse(fmt.Errorf("failed to read first bidirectional message: %w", err))
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	if !firstReq.GetIsFirst() {
		code, message := createErrorResponse(fmt.Errorf("first bidirectional message must set is_first=true"))
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	serviceName := firstReq.GetServiceName()
	if serviceName == "" {
		code, message := createErrorResponse(fmt.Errorf("service_name is required in first bidirectional message"))
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	authInfo := firstReq.GetAuthInfo()
	wsConnID := ""
	if md := firstReq.GetMetadata(); md != nil {
		wsConnID = md["ws_conn_id"]
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	convertReq := func(req *protocol.InvokeServiceBidiRequest) client.BidiMessage {
		var metadata map[string]string
		if len(req.GetMetadata()) > 0 {
			metadata = make(map[string]string, len(req.GetMetadata()))
			for k, v := range req.GetMetadata() {
				metadata[k] = v
			}
		}
		return client.BidiMessage{
			Data:        req.GetData(),
			MessageType: req.GetMessageType(),
			Metadata:    metadata,
		}
	}

	inStream := make(chan client.BidiMessage, 10)
	outStream := make(chan client.BidiMessage, 10)
	errChan := make(chan error, 2)

	shouldForwardFirst := len(firstReq.GetData()) > 0 || firstReq.GetMessageType() != ""
	if shouldForwardFirst {
		select {
		case inStream <- convertReq(firstReq):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(inStream)
		for {
			req, recvErr := stream.Recv()
			if recvErr == io.EOF {
				return
			}
			if recvErr != nil {
				select {
				case errChan <- recvErr:
				default:
				}
				cancel()
				return
			}

			msg := convertReq(req)
			select {
			case inStream <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(outStream)
		for msg := range outStream {
			resp := &protocol.InvokeServiceBidiResponse{
				Code:        0,
				Message:     "success",
				Data:        msg.Data,
				MessageType: msg.MessageType,
				Metadata:    msg.Metadata,
			}
			if msg.Error != nil {
				resp.Code, resp.Message = createErrorResponse(msg.Error)
			}
			if sendErr := stream.Send(resp); sendErr != nil {
				select {
				case errChan <- sendErr:
				default:
				}
				cancel()
				return
			}
		}
	}()

	pluginErr := bidiPlugin.InvokeServiceBidirectional(ctx, serviceName, wsConnID, authInfo, inStream, outStream)
	//close(outStream)
	wg.Wait()

	for {
		select {
		case transportErr := <-errChan:
			if transportErr != nil {
				return transportErr
			}
		default:
			goto transportChecked
		}
	}

transportChecked:
	if pluginErr != nil {
		code, message := createErrorResponse(pluginErr)
		return stream.Send(&protocol.InvokeServiceBidiResponse{Code: code, Message: message})
	}

	return nil
}

// PullModelStream pulls a model with streaming (Local plugin only).
func (s *GRPCProviderServer) PullModelStream(req *protocol.PullModelStreamRequest, stream protocol.ProviderService_PullModelStreamServer) error {
	if s.LocalPlugin == nil {
		code, message := createErrorResponse(fmt.Errorf("not a local plugin"))
		return stream.Send(&protocol.PullModelStreamResponse{
			Code:    code,
			Message: message,
		})
	}

	// 1. Parse request
	var pullReq types.PullModelRequest
	if err := json.Unmarshal(req.RequestJson, &pullReq); err != nil {
		code, message := createErrorResponse(err)
		return stream.Send(&protocol.PullModelStreamResponse{
			Code:    code,
			Message: message,
		})
	}

	// 2. Call plugin's streaming method
	dataChan, errChan := s.LocalPlugin.PullModelStream(stream.Context(), &pullReq)

	// 3. Forward streaming data
	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				// Channel closed, normal completion
				return nil
			}
			// Send data chunk
			if err := stream.Send(&protocol.PullModelStreamResponse{
				Code:    0,
				Message: "success",
				Data:    data,
			}); err != nil {
				return err
			}

		case err, ok := <-errChan:
			if ok && err != nil {
				// Send error
				code, message := createErrorResponse(err)
				stream.Send(&protocol.PullModelStreamResponse{
					Code:    code,
					Message: message,
				})
				return err
			}
			return nil

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
