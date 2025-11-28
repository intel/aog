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

// GenerateService implements text generation service, supports both streaming and non-streaming
type GenerateService struct {
	client ClientInterface
}

// NewGenerateService creates a new Generate service instance
func NewGenerateService(client ClientInterface) *GenerateService {
	return &GenerateService{client: client}
}

// HandleUnary handles non-streaming generation request
func (s *GenerateService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Build Ollama request (non-streaming)
	ollamaReq := s.buildRequestFromMap(req, false)

	// Call Ollama Generate API - return raw response directly
	var ollamaResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "/api/generate", authInfo, ollamaReq, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama generate failed: %w", err)
	}

	// Return ollama raw response directly, handled by AOG Core conversion rules
	return json.Marshal(ollamaResp)
}

// HandleStreaming handles streaming generation request
func (s *GenerateService) HandleStreaming(ctx context.Context, authInfo string, request []byte, ch chan<- client.StreamChunk) {
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		ch <- client.StreamChunk{Error: fmt.Errorf("failed to unmarshal request: %w", err)}
		return
	}

	// Build Ollama request (streaming)
	ollamaReq := s.buildRequestFromMap(req, true)

	// Call Ollama streaming Generate API
	dataChan, errChan := s.client.StreamResponse(ctx, http.MethodPost, "/api/generate", authInfo, ollamaReq)

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				ch <- client.StreamChunk{IsFinal: true}
				return
			}

			// 转换为SSE格式
			sseData := fmt.Sprintf("data: %s\n\n", string(data))
			ch <- client.StreamChunk{
				Data: []byte(sseData),
				Metadata: map[string]string{
					"content-type": "text/event-stream",
				},
			}

		case err := <-errChan:
			if err != nil {
				ch <- client.StreamChunk{Error: fmt.Errorf("ollama generate streaming failed: %w", err)}
			}
			return

		case <-ctx.Done():
			ch <- client.StreamChunk{Error: ctx.Err()}
			return
		}
	}
}

// buildRequestFromMap builds Ollama Generate request from raw request map
func (s *GenerateService) buildRequestFromMap(req map[string]interface{}, stream bool) map[string]interface{} {
	ollamaReq := map[string]interface{}{
		"stream": stream,
	}

	// Extract fields directly from request
	if model, ok := req["model"]; ok {
		ollamaReq["model"] = model
	}
	if prompt, ok := req["prompt"]; ok {
		ollamaReq["prompt"] = prompt
	}

	// Optional parameters
	if temperature, ok := req["temperature"]; ok {
		ollamaReq["temperature"] = temperature
	}
	if topP, ok := req["top_p"]; ok {
		ollamaReq["top_p"] = topP
	}

	return ollamaReq
}
