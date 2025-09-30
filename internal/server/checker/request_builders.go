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

package checker

import (
	"encoding/json"

	"github.com/intel/aog/internal/types"
)

// ModelsRequestBuilder builds requests for models service
type ModelsRequestBuilder struct{}

// BuildRequest implements RequestBuilder for models service
func (m *ModelsRequestBuilder) BuildRequest(modelName string) ([]byte, error) {
	// Models service doesn't need request body
	return nil, nil
}

// ChatRequestBuilder builds requests for chat service
type ChatRequestBuilder struct{}

// BuildRequest implements RequestBuilder for chat service
func (c *ChatRequestBuilder) BuildRequest(modelName string) ([]byte, error) {
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type RequestBody struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}

	requestBody := RequestBody{
		Model:  modelName,
		Stream: false,
		Messages: []Message{
			{
				Role:    "user",
				Content: "Hello!",
			},
		},
	}
	return json.Marshal(requestBody)
}

// EmbeddingRequestBuilder builds requests for embedding service
type EmbeddingRequestBuilder struct{}

// BuildRequest implements RequestBuilder for embedding service
func (e *EmbeddingRequestBuilder) BuildRequest(modelName string) ([]byte, error) {
	type RequestBody struct {
		Model          string   `json:"model"`
		Input          []string `json:"input"`
		Inputs         []string `json:"inputs"`
		Dimensions     int      `json:"dimensions"`
		EncodingFormat string   `json:"encoding_format"`
	}
	requestBody := RequestBody{
		Model:          modelName,
		Input:          []string{"test text"},
		Inputs:         []string{"test text"},
		Dimensions:     1024,
		EncodingFormat: "float",
	}
	return json.Marshal(requestBody)
}

// TextToImageRequestBuilder builds requests for text-to-image service
type TextToImageRequestBuilder struct {
	Flavor string
}

// BuildRequest implements RequestBuilder for text-to-image service
func (t *TextToImageRequestBuilder) BuildRequest(modelName string) ([]byte, error) {
	prompt := "Draw a puppy"
	switch t.Flavor {
	case types.FlavorTencent:
		type RequestBody struct {
			Model      string `json:"model"`
			Prompt     string `json:"Prompt"`
			RspImgType string `json:"RspImgType"`
		}
		requestBody := RequestBody{
			Model:      modelName,
			Prompt:     prompt,
			RspImgType: "url",
		}
		return json.Marshal(requestBody)
	case types.FlavorAliYun:
		type InputData struct {
			Prompt string `json:"prompt"`
		}
		type RequestBody struct {
			Model string    `json:"model"`
			Input InputData `json:"input"`
		}
		requestBody := RequestBody{
			Model: modelName,
			Input: InputData{Prompt: prompt},
		}
		return json.Marshal(requestBody)
	case types.FlavorBaidu:
		type RequestBody struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		requestBody := RequestBody{
			Model:  modelName,
			Prompt: prompt,
		}
		return json.Marshal(requestBody)
	default:
		type RequestBody struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		requestBody := RequestBody{
			Model:  modelName,
			Prompt: prompt,
		}
		return json.Marshal(requestBody)
	}
}

// TextToSpeechRequestBuilder builds requests for text-to-speech service
type TextToSpeechRequestBuilder struct {
	Flavor string
}

// BuildRequest implements RequestBuilder for text-to-speech service
func (t *TextToSpeechRequestBuilder) BuildRequest(modelName string) ([]byte, error) {
	prompt := "我来给大家推荐一款T恤，这款呢真的是超级好看"
	switch t.Flavor {
	case types.FlavorTencent:
		type RequestBody struct {
			Model      string `json:"model"`
			Prompt     string `json:"Prompt"`
			RspImgType string `json:"RspImgType"`
		}
		requestBody := RequestBody{
			Model:      modelName,
			Prompt:     prompt,
			RspImgType: "url",
		}
		return json.Marshal(requestBody)
	case types.FlavorAliYun:
		type InputData struct {
			Text  string `json:"text"`
			Voice string `json:"voice"`
		}
		type RequestBody struct {
			Model string    `json:"model"`
			Input InputData `json:"input"`
		}
		requestBody := RequestBody{
			Model: modelName,
			Input: InputData{
				Text:  prompt,
				Voice: "Chelsie",
			},
		}
		return json.Marshal(requestBody)
	case types.FlavorBaidu:
		type RequestBody struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		requestBody := RequestBody{
			Model:  modelName,
			Prompt: prompt,
		}
		return json.Marshal(requestBody)
	default:
		type RequestBody struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		requestBody := RequestBody{
			Model:  modelName,
			Prompt: prompt,
		}
		return json.Marshal(requestBody)
	}
}
