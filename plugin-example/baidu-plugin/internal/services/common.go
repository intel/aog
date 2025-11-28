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

package services

import (
	"context"

	"github.com/intel/aog/plugin-sdk/client"
)

// ServiceHandler defines the unary handler contract
type ServiceHandler interface {
	HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error)
}

// StreamingHandler defines the streaming handler contract
type StreamingHandler interface {
	HandleStreaming(ctx context.Context, authInfo string, request []byte, ch chan<- client.StreamChunk)
}

// ServiceRequest is the common request structure
type ServiceRequest struct {
	Service string                 `json:"service"`
	Data    map[string]interface{} `json:"data"`
}

// ServiceResponse is the common response structure
type ServiceResponse struct {
	Data  map[string]interface{} `json:"data"`
	Error string                 `json:"error,omitempty"`
}

// ClientInterface abstracts the HTTP client for testing/decoupling
type ClientInterface interface {
	Do(ctx context.Context, method, service, authInfo string, reqData interface{}, respData interface{}) error
	StreamResponse(ctx context.Context, method, service, authInfo string, reqData interface{}) (chan []byte, chan error)
}
