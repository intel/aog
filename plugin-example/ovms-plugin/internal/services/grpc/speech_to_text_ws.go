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
)

// SpeechToTextWSHandler is a placeholder for speech-to-text-ws service
// Note: speech-to-text-ws uses bidirectional communication and is handled
// entirely through provider.InvokeServiceBidirectional() using GRPCBidiStreamHandler.
// This handler exists only to satisfy the service handler registry.
type SpeechToTextWSHandler struct {
	serviceName string
}

// NewSpeechToTextWSHandler creates a new WebSocket speech-to-text handler
func NewSpeechToTextWSHandler(ovmsHost string, ovmsPort int, endpoint string) *SpeechToTextWSHandler {
	return &SpeechToTextWSHandler{
		serviceName: "speech-to-text-ws",
	}
}

// Handle is not used for bidirectional services
// All bidirectional communication is handled through InvokeServiceBidirectional
func (h *SpeechToTextWSHandler) Handle(ctx context.Context, request []byte) ([]byte, error) {
	// This should never be called for bidirectional services
	// Return error to indicate misconfiguration
	return nil, nil
}

// HandleStream is not used for bidirectional services
// All bidirectional communication is handled through InvokeServiceBidirectional
func (h *SpeechToTextWSHandler) HandleStream(ctx context.Context, request []byte) (<-chan []byte, error) {
	// This should never be called for bidirectional services
	// Return empty channel to indicate misconfiguration
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}
