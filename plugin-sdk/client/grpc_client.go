//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

// Package client provides plugin gRPC client implementation
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/intel/aog/plugin-sdk/protocol"
	"github.com/intel/aog/plugin-sdk/types"
	"google.golang.org/grpc"
)

// GRPCProviderClient is the gRPC client implementation on the Host side
//
// It wraps the protobuf-generated ProviderServiceClient,
// implementing the PluginProvider, LocalPluginProvider, and RemotePluginProvider interfaces.
//
// This client is responsible for:
// - Converting interface method calls to gRPC calls
// - Handling request/response serialization and deserialization
// - Handling error codes and error messages
// - Managing streaming communication
type GRPCProviderClient struct {
	client protocol.ProviderServiceClient
	conn   *grpc.ClientConn
}

// InvokeServiceBidirectional bridges channels to the gRPC bidi stream to satisfy the BidirectionalPlugin interface.
func (c *GRPCProviderClient) InvokeServiceBidirectional(
	ctx context.Context,
	serviceName string,
	wsConnID string,
	authInfo string,
	inStream <-chan BidiMessage,
	outStream chan<- BidiMessage,
) error {
	sendChan, recvChan, err := c.InvokeServiceBidi(ctx, serviceName, authInfo)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Forward inbound messages to gRPC stream
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(sendChan)
		first := true
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case msg, ok := <-inStream:
				if !ok {
					return
				}
				if first {
					if wsConnID != "" {
						if msg.Metadata == nil {
							msg.Metadata = make(map[string]string)
						}
						msg.Metadata["ws_conn_id"] = wsConnID
					}
					first = false
				}
				select {
				case sendChan <- msg:
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				}
			}
		}
	}()

	// Forward plugin responses back to caller
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case msg, ok := <-recvChan:
				if !ok {
					return
				}
				select {
				case outStream <- msg:
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				}
			}
		}
	}()

	wg.Wait()
	close(errChan)
	for e := range errChan {
		if e != nil && e != context.Canceled {
			return e
		}
	}

	return nil
}

func (c *GRPCProviderClient) GetSupportModelList(ctx context.Context) ([]types.RecommendModelData, error) {
	manifest := c.GetManifest()
	if manifest == nil {
		return nil, fmt.Errorf("failed to get plugin manifest")
	}

	var models []types.RecommendModelData
	for _, svc := range manifest.Services {
		for _, name := range svc.SupportModels {
			m := types.RecommendModelData{
				Id:              fmt.Sprintf("%s:%s:%s", manifest.Provider.Name, svc.ServiceName, name),
				Service:         svc.ServiceName,
				ApiFlavor:       "",
				Flavor:          "",
				Method:          "POST",
				Desc:            manifest.Provider.Description,
				Url:             svc.Endpoint,
				AuthType:        svc.AuthType,
				AuthApplyUrl:    manifest.Provider.Homepage,
				AuthFields:      nil,
				Name:            name,
				ServiceProvider: manifest.Provider.Name,
				Size:            "",
				IsRecommended:   name == svc.DefaultModel,
				Status:          "",
				Avatar:          "",
				CanSelect:       true,
				Class:           nil,
				OllamaId:        "",
				ParamsSize:      0,
				InputLength:     0,
				OutputLength:    0,
				Source:          manifest.Provider.Type,
				IsDefault:       fmt.Sprintf("%v", name == svc.DefaultModel),
				Think:           false,
				ThinkSwitch:     false,
				Tools:           false,
				Context:         0,
			}
			models = append(models, m)
		}
	}

	return models, nil
}

// NewGRPCProviderClient creates a gRPC client
//
// Parameters:
//   - client: protobuf-generated gRPC client stub
//
// Returns:
//   - *GRPCProviderClient: wrapped client implementing all plugin interfaces
func NewGRPCProviderClientFromStub(client protocol.ProviderServiceClient) *GRPCProviderClient {
	return &GRPCProviderClient{
		client: client,
		conn:   nil,
	}
}

// NewGRPCProviderClient creates a client from gRPC connection
func NewGRPCProviderClient(conn *grpc.ClientConn) *GRPCProviderClient {
	return &GRPCProviderClient{
		client: protocol.NewProviderServiceClient(conn),
		conn:   conn,
	}
}

// Close closes the client connection
func (c *GRPCProviderClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ===== PluginProvider Interface Implementation =====

// InvokeService invokes a plugin service (core method)
func (c *GRPCProviderClient) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	resp, err := c.client.InvokeService(ctx, &protocol.InvokeServiceRequest{
		ServiceName: serviceName,
		RequestData: request,
		AuthInfo:    authInfo,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil, protocol.NewPluginError(resp.Code, resp.Message)
	}

	return resp.ResponseData, nil
}

// InvokeServiceStream performs server-side streaming invocation
func (c *GRPCProviderClient) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string,
	request []byte,
) (<-chan StreamChunk, error) {
	stream, err := c.client.InvokeServiceStream(ctx, &protocol.InvokeServiceRequest{
		ServiceName: serviceName,
		AuthInfo:    authInfo,
		RequestData: request,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	ch := make(chan StreamChunk, 10)

	go func() {
		defer close(ch)

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				ch <- StreamChunk{
					Error: fmt.Errorf("stream recv error: %w", err),
				}
				return
			}

			if !protocol.IsSuccess(resp.Code) {
				ch <- StreamChunk{
					Error: protocol.NewPluginError(resp.Code, resp.Message),
				}
				return
			}

			ch <- StreamChunk{
				Data:     resp.ChunkData,
				IsFinal:  resp.IsFinal,
				Metadata: resp.Metadata,
			}

			if resp.IsFinal {
				return
			}
		}
	}()

	return ch, nil
}

// InvokeServiceBidi bidirectional streaming call
func (c *GRPCProviderClient) InvokeServiceBidi(
	ctx context.Context,
	serviceName string,
	authInfo string,
) (chan<- BidiMessage, <-chan BidiMessage, error) {
	stream, err := c.client.InvokeServiceBidi(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	sendChan := make(chan BidiMessage, 10)
	recvChan := make(chan BidiMessage, 10)

	var wg sync.WaitGroup
	closeRecvChan := func() {
		close(recvChan)
	}
	safeSend := func(msg BidiMessage) {
		recvChan <- msg
	}

	// Send coroutine（AOG → gRPC → plugin）
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stream.CloseSend()

		isFirst := true
		for msg := range sendChan {
			err := stream.Send(&protocol.InvokeServiceBidiRequest{
				ServiceName: func() string {
					if isFirst {
						return serviceName
					}
					return ""
				}(),
				Data:        msg.Data,
				MessageType: msg.MessageType,
				Metadata:    msg.Metadata,
				IsFirst:     isFirst,
				AuthInfo: func() string {
					if isFirst {
						return authInfo
					}
					return ""
				}(),
			})
			if err != nil {
				// Unable to log, error can only be returned through the receive channel
				safeSend(BidiMessage{
					Error: fmt.Errorf("failed to send: %w", err),
				})
				return
			}

			isFirst = false
		}
	}()

	// receive coroutine（plugin → gRPC → AOG）
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				safeSend(BidiMessage{
					Error: fmt.Errorf("stream recv error: %w", err),
				})
				return
			}

			if !protocol.IsSuccess(resp.Code) {
				safeSend(BidiMessage{
					Error: protocol.NewPluginError(resp.Code, resp.Message),
				})
				return
			}

			safeSend(BidiMessage{
				Data:        resp.Data,
				MessageType: resp.MessageType,
				Metadata:    resp.Metadata,
			})
		}
	}()

	go func() {
		wg.Wait()
		closeRecvChan()
	}()

	return sendChan, recvChan, nil
}

// HealthCheck Health Check
func (c *GRPCProviderClient) HealthCheck(ctx context.Context) error {
	resp, err := c.client.HealthCheck(ctx, &protocol.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// GetManifest Get plugin metadata
func (c *GRPCProviderClient) GetManifest() *types.PluginManifest {
	resp, err := c.client.GetManifest(context.Background(), &protocol.GetManifestRequest{})
	if err != nil {
		// SDK should not log, return nil directly
		return nil
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil
	}

	var manifest types.PluginManifest
	if err := json.Unmarshal(resp.ManifestJson, &manifest); err != nil {
		return nil
	}

	return &manifest
}

// GetOperateStatus Get running status
func (c *GRPCProviderClient) GetOperateStatus() int {
	resp, err := c.client.GetOperateStatus(context.Background(), &protocol.GetOperateStatusRequest{})
	if err != nil {
		return 0
	}
	return int(resp.Status)
}

// SetOperateStatus Set the running state
func (c *GRPCProviderClient) SetOperateStatus(status int) {
	_, _ = c.client.SetOperateStatus(context.Background(), &protocol.SetOperateStatusRequest{
		Status: int32(status),
	})
}

// ===== RemotePluginProvider interface implementation =====

// SetAuth Set authentication information
func (c *GRPCProviderClient) SetAuth(req *http.Request, authType string, credentials map[string]string) error {
	resp, err := c.client.SetAuth(context.Background(), &protocol.SetAuthRequest{
		AuthType:    authType,
		Credentials: credentials,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// ValidateAuth Verify authentication information
func (c *GRPCProviderClient) ValidateAuth(ctx context.Context) error {
	resp, err := c.client.ValidateAuth(ctx, &protocol.ValidateAuthRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// RefreshAuth Refresh authentication information
func (c *GRPCProviderClient) RefreshAuth(ctx context.Context) error {
	resp, err := c.client.RefreshAuth(ctx, &protocol.RefreshAuthRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// ===== LocalPluginProvider interface implementation =====

// StartEngine Start the engine
func (c *GRPCProviderClient) StartEngine(mode string) error {
	resp, err := c.client.StartEngine(context.Background(), &protocol.StartEngineRequest{Mode: mode})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// StopEngine Stop the engine
func (c *GRPCProviderClient) StopEngine() error {
	resp, err := c.client.StopEngine(context.Background(), &protocol.StopEngineRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// CheckEngine checks if the engine is installed
func (c *GRPCProviderClient) CheckEngine() (bool, error) {
	resp, err := c.client.CheckEngine(context.Background(), &protocol.CheckEngineRequest{})
	if err != nil {
		return false, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return false, protocol.NewPluginError(resp.Code, resp.Message)
	}

	return resp.Installed, nil
}

// GetConfig gets engine configuration
func (c *GRPCProviderClient) GetConfig(ctx context.Context) (*types.EngineRecommendConfig, error) {
	resp, err := c.client.GetConfig(ctx, &protocol.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil, protocol.NewPluginError(resp.Code, resp.Message)
	}

	var config types.EngineRecommendConfig
	if err := json.Unmarshal(resp.ConfigJson, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// InstallEngine installs the engine
func (c *GRPCProviderClient) InstallEngine(ctx context.Context) error {
	resp, err := c.client.InstallEngine(ctx, &protocol.InstallEngineRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// InitEnv initializes environment
func (c *GRPCProviderClient) InitEnv() error {
	resp, err := c.client.InitEnv(context.Background(), &protocol.InitEnvRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// UpgradeEngine upgrades the engine
func (c *GRPCProviderClient) UpgradeEngine(ctx context.Context) error {
	resp, err := c.client.UpgradeEngine(ctx, &protocol.UpgradeEngineRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// PullModel pulls a model
func (c *GRPCProviderClient) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.PullModel(ctx, &protocol.PullModelRequest{RequestJson: reqJSON})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil, protocol.NewPluginError(resp.Code, resp.Message)
	}

	var progressResp types.ProgressResponse
	if err := json.Unmarshal(resp.ResponseJson, &progressResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &progressResp, nil
}

// PullModelStream pulls a model with streaming
func (c *GRPCProviderClient) PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error) {
	dataChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(dataChan)
		defer close(errChan)

		reqJSON, err := json.Marshal(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		stream, err := c.client.PullModelStream(ctx, &protocol.PullModelStreamRequest{RequestJson: reqJSON})
		if err != nil {
			errChan <- fmt.Errorf("gRPC call failed: %w", err)
			return
		}

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errChan <- fmt.Errorf("stream recv error: %w", err)
				return
			}

			if !protocol.IsSuccess(resp.Code) {
				errChan <- protocol.NewPluginError(resp.Code, resp.Message)
				return
			}

			dataChan <- resp.Data
		}
	}()

	return dataChan, errChan
}

// DeleteModel deletes a model
func (c *GRPCProviderClient) DeleteModel(ctx context.Context, req *types.DeleteRequest) error {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.DeleteModel(ctx, &protocol.DeleteModelRequest{RequestJson: reqJSON})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// ListModels lists all models
func (c *GRPCProviderClient) ListModels(ctx context.Context) (*types.ListResponse, error) {
	resp, err := c.client.ListModels(ctx, &protocol.ListModelsRequest{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil, protocol.NewPluginError(resp.Code, resp.Message)
	}

	var listResp types.ListResponse
	if err := json.Unmarshal(resp.ResponseJson, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &listResp, nil
}

// LoadModel loads a model
func (c *GRPCProviderClient) LoadModel(ctx context.Context, req *types.LoadRequest) error {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.LoadModel(ctx, &protocol.LoadModelRequest{RequestJson: reqJSON})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// UnloadModel unloads models
func (c *GRPCProviderClient) UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.UnloadModel(ctx, &protocol.UnloadModelRequest{RequestJson: reqJSON})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return protocol.NewPluginError(resp.Code, resp.Message)
	}

	return nil
}

// GetRunningModels gets running models
func (c *GRPCProviderClient) GetRunningModels(ctx context.Context) (*types.ListResponse, error) {
	resp, err := c.client.GetRunningModels(ctx, &protocol.GetRunningModelsRequest{})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(resp.Code) {
		return nil, protocol.NewPluginError(resp.Code, resp.Message)
	}

	var listResp types.ListResponse
	if err := json.Unmarshal(resp.ResponseJson, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &listResp, nil
}

// GetVersion gets engine version
func (c *GRPCProviderClient) GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error) {
	reqJSON, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	grpcResp, err := c.client.GetVersion(ctx, &protocol.GetVersionRequest{RequestJson: reqJSON})
	if err != nil {
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	if !protocol.IsSuccess(grpcResp.Code) {
		return nil, protocol.NewPluginError(grpcResp.Code, grpcResp.Message)
	}

	var versionResp types.EngineVersionResponse
	if err := json.Unmarshal(grpcResp.ResponseJson, &versionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &versionResp, nil
}
