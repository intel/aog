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

package grpc

import (
	"context"
	"fmt"

	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/grpc_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCBaseHandler provides common functionality for gRPC-based service handlers
type GRPCBaseHandler struct {
	ServiceName string
	OVMSHost    string
	OVMSPort    int // gRPC port (9000)
}

// NewGRPCBaseHandler creates a new gRPC base handler
func NewGRPCBaseHandler(serviceName string, ovmsHost string, ovmsPort int) *GRPCBaseHandler {
	return &GRPCBaseHandler{
		ServiceName: serviceName,
		OVMSHost:    ovmsHost,
		OVMSPort:    ovmsPort,
	}
}

// CreateGRPCConnection establishes a gRPC connection to OVMS
func (h *GRPCBaseHandler) CreateGRPCConnection(ctx context.Context) (*grpc.ClientConn, error) {
	addr := fmt.Sprintf("%s:%d", h.OVMSHost, h.OVMSPort)
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OVMS gRPC at %s: %w", addr, err)
	}
	return conn, nil
}

// SendGRPCRequest sends a unary gRPC request to OVMS
func (h *GRPCBaseHandler) SendGRPCRequest(
	ctx context.Context,
	modelName string,
	inputs []*grpc_client.ModelInferRequest_InferInputTensor,
	rawInputs [][]byte,
	outputs []*grpc_client.ModelInferRequest_InferRequestedOutputTensor,
) (*grpc_client.ModelInferResponse, error) {
	// 1. Create gRPC connection
	conn, err := h.CreateGRPCConnection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 2. Create gRPC client
	gClient := grpc_client.NewGRPCInferenceServiceClient(conn)

	// 3. Prepare request
	req := &grpc_client.ModelInferRequest{
		ModelName:        modelName,
		Inputs:           inputs,
		RawInputContents: rawInputs,
	}

	// Add requested outputs if specified
	if len(outputs) > 0 {
		req.Outputs = outputs
	}

	// 4. Send request
	resp, err := gClient.ModelInfer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC ModelInfer failed: %w", err)
	}

	return resp, nil
}

// LogInfo logs an info message (to be implemented by embedding struct)
func (h *GRPCBaseHandler) LogInfo(msg string) {
	fmt.Printf("[%s] INFO: %s\n", h.ServiceName, msg)
}

// LogDebug logs a debug message
func (h *GRPCBaseHandler) LogDebug(msg string) {
	fmt.Printf("[%s] DEBUG: %s\n", h.ServiceName, msg)
}

// LogError logs an error message
func (h *GRPCBaseHandler) LogError(msg string, err error) {
	fmt.Printf("[%s] ERROR: %s: %v\n", h.ServiceName, msg, err)
}
