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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/intel/aog/plugin-sdk/client"
)

// ChatService implements chat service, supports both streaming and non-streaming modes
type ChatService struct {
	client ClientInterface
}

// NewChatService creates a new Chat service instance
func NewChatService(client ClientInterface) *ChatService {
	return &ChatService{client: client}
}

// HandleUnary handles non-streaming chat request
func (s *ChatService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	// Parse directly to map, AOG Core sends raw request format
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Build Ollama request (non-streaming)
	ollamaReq := s.buildRequestFromMap(req, false)

	var ollamaResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "/api/chat", authInfo, ollamaReq, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama chat failed: %w", err)
	}

	// Return ollama raw response directly, handled by AOG Core conversion rules
	return json.Marshal(ollamaResp)
}

// HandleStreaming handles streaming chat request
func (s *ChatService) HandleStreaming(ctx context.Context, authInfo string, request []byte, ch chan<- client.StreamChunk) {
	// Parse directly to map, AOG Core sends raw request format
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		ch <- client.StreamChunk{Error: fmt.Errorf("failed to unmarshal request: %w", err)}
		return
	}

	// Build Ollama request (streaming)
	ollamaReq := s.buildRequestFromMap(req, true)

	// Call Ollama streaming API
	dataChan, errChan := s.client.StreamResponse(ctx, http.MethodPost, "/api/chat", authInfo, ollamaReq)

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				// Channel closed, send final chunk
				ch <- client.StreamChunk{IsFinal: true}
				return
			}

			// Convert to SSE format
			sseData := fmt.Sprintf("data: %s\n\n", string(data))
			ch <- client.StreamChunk{
				Data: []byte(sseData),
				Metadata: map[string]string{
					"content-type": "text/event-stream",
				},
			}

		case err := <-errChan:
			if err != nil {
				ch <- client.StreamChunk{Error: fmt.Errorf("ollama streaming failed: %w", err)}
			}
			return

		case <-ctx.Done():
			ch <- client.StreamChunk{Error: ctx.Err()}
			return
		}
	}
}

// buildRequestFromMap builds Ollama request from raw request map
func (s *ChatService) buildRequestFromMap(req map[string]interface{}, stream bool) map[string]interface{} {
	ollamaReq := map[string]interface{}{
		"stream": stream,
	}

	// Extract fields directly from request
	if model, ok := req["model"]; ok {
		ollamaReq["model"] = model
	}
	if messages, ok := req["messages"]; ok {
		ollamaReq["messages"] = messages
	}

	// Optional parameters
	if temperature, ok := req["temperature"]; ok {
		ollamaReq["temperature"] = temperature
	}
	if topP, ok := req["top_p"]; ok {
		ollamaReq["top_p"] = topP
	}
	if maxTokens, ok := req["max_tokens"]; ok {
		ollamaReq["num_predict"] = maxTokens
	}

	return ollamaReq
}
