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

// TextToImageService handles text-to-image generation (non-streaming)
type TextToImageService struct {
	client ClientInterface
}

// NewTextToImageService creates a new TextToImageService
func NewTextToImageService(client ClientInterface) *TextToImageService {
	return &TextToImageService{client: client}
}

// HandleUnary processes text-to-image requests
func (s *TextToImageService) HandleUnary(ctx context.Context, authInfo string, request []byte) ([]byte, error) {
	var req ServiceRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Call the Aliyun API
	var aliyunResp map[string]interface{}
	if err := s.client.Do(ctx, http.MethodPost, "text-to-image", authInfo, request, &aliyunResp); err != nil {
		return nil, fmt.Errorf("aliyun text-to-image failed: %w, authInfo: %s", err, authInfo)
	}

	return json.Marshal(aliyunResp)
}
