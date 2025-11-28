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
)

// EmbedService implements embedding service, only supports non-streaming
type EmbedService struct {
	client ClientInterface
}

// NewEmbedService creates a new Embed service instance
func NewEmbedService(client ClientInterface) *EmbedService {
	return &EmbedService{client: client}
}

// HandleUnary handles embedding request
func (s *EmbedService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	// Parse directly to map, AOG Core sends raw request format
	var req map[string]interface{}
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Build Ollama request
	ollamaReq := map[string]interface{}{}

	// Extract fields directly from request
	if model, ok := req["model"]; ok {
		ollamaReq["model"] = model
	}
	// AOG uses "input" field, ollama uses "prompt" field
	if input, ok := req["input"]; ok {
		ollamaReq["prompt"] = input
	}

	// Call Ollama API
	var ollamaResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "/api/embeddings", authInfo, ollamaReq, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ollama embed failed: %w", err)
	}

	// Return ollama raw response directly, handled by AOG Core conversion rules
	return json.Marshal(ollamaResp)
}
