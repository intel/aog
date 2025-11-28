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

// ChatService Implement chat services that support both streaming and non-streaming modes
type ChatService struct {
	client ClientInterface
}

// NewChatService Create a new chat service instance
func NewChatService(client ClientInterface) *ChatService {
	return &ChatService{client: client}
}

// HandleUnary Handling non-streaming chat requests
func (s *ChatService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	var deepseekResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "chat", authInfo, request, &deepseekResp); err != nil {
		return nil, fmt.Errorf("deepseek chat failed: %w", err)
	}

	return json.Marshal(deepseekResp)
}

// HandleStreaming Processing streaming chat requests
func (s *ChatService) HandleStreaming(ctx context.Context, authInfo string, request []byte, ch chan<- client.StreamChunk) {
	// Call the deepseek streaming API
	dataChan, errChan := s.client.StreamResponse(ctx, http.MethodPost, "chat", authInfo, request)

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				// Channel closed, sending the last chunk
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
				ch <- client.StreamChunk{Error: fmt.Errorf("deepseek streaming failed: %w", err)}
			}
			return

		case <-ctx.Done():
			ch <- client.StreamChunk{Error: ctx.Err()}
			return
		}
	}
}
