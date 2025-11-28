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

// ChatService handles both streaming and non-streaming chat flows
type ChatService struct {
	client ClientInterface
}

// NewChatService creates a ChatService instance
func NewChatService(client ClientInterface) *ChatService {
	return &ChatService{client: client}
}

// HandleUnary processes non-streaming chat requests
func (s *ChatService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	var tencentResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "chat", authInfo, request, &tencentResp); err != nil {
		return nil, fmt.Errorf("tencent chat failed: %w", err)
	}

	return s.buildResponse(tencentResp)
}

// HandleStreaming processes streaming chat requests
func (s *ChatService) HandleStreaming(ctx context.Context, authInfo string, request []byte, ch chan<- client.StreamChunk) {
	// Call Tencent streaming API
	dataChan, errChan := s.client.StreamResponse(ctx, http.MethodPost, "chat", authInfo, request)

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				// Channel closed, sending the last chunk
				ch <- client.StreamChunk{IsFinal: true}
				return
			}

			// Convert data to Server-Sent Events (SSE) format
			sseData := fmt.Sprintf("data: %s\n\n", string(data))
			ch <- client.StreamChunk{
				Data: []byte(sseData),
				Metadata: map[string]string{
					"content-type": "text/event-stream",
				},
			}

		case err := <-errChan:
			if err != nil {
				ch <- client.StreamChunk{Error: fmt.Errorf("tencent streaming failed: %w", err)}
			}
			return

		case <-ctx.Done():
			ch <- client.StreamChunk{Error: ctx.Err()}
			return
		}
	}
}

// buildRequest builds the Tencent payload shared by unary/streaming requests
func (s *ChatService) buildRequest(req *ServiceRequest, stream bool) map[string]interface{} {
	tencentReq := map[string]interface{}{
		"stream": stream,
	}

	// Extract required fields
	if model, ok := req.Data["model"]; ok {
		tencentReq["model"] = model
	}
	if messages, ok := req.Data["messages"]; ok {
		tencentReq["messages"] = messages
	}
	if !stream {
		// Allow overrides in non-streaming mode
		if streamParam, ok := req.Data["stream"]; ok {
			tencentReq["stream"] = streamParam
		}
	}

	return tencentReq
}

// buildResponse assembles the response (non-streaming only)
func (s *ChatService) buildResponse(tencentResp map[string]interface{}) ([]byte, error) {
	respData := map[string]interface{}{}

	// Extract message content
	if message, ok := tencentResp["message"]; ok {
		respData["message"] = message
	}

	// Extract model
	if model, ok := tencentResp["model"]; ok {
		respData["model"] = model
	}

	// Extract usage info
	if promptTokens, ok := tencentResp["prompt_eval_count"].(float64); ok {
		if evalTokens, ok := tencentResp["eval_count"].(float64); ok {
			respData["usage"] = map[string]interface{}{
				"prompt_tokens":     int(promptTokens),
				"completion_tokens": int(evalTokens),
				"total_tokens":      int(promptTokens + evalTokens),
			}
		}
	}

	// Return serialized response
	resp := ServiceResponse{Data: respData}
	return json.Marshal(resp)
}
