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

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/intel/aog/plugin-sdk/types"
)

// =============== ModelManager Implementation ===============

// PullModel downloads a model
func (p *OllamaProvider) PullModel(ctx context.Context, req *types.PullModelRequest, fn types.PullProgressFunc) (*types.ProgressResponse, error) {
	// Prefer Model field, if empty use Name (backward compatible)
	modelName := req.Model
	if modelName == "" {
		modelName = req.Name
	}

	log.Printf("[ollama-plugin] [INFO] Pulling model: %s", modelName)

	pullReq := map[string]interface{}{
		"name": modelName,
	}

	dataChan, errChan := p.client.StreamResponse(ctx, http.MethodPost, "/api/pull", "", pullReq)

	// Process streaming response
	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				log.Printf("[ollama-plugin] [INFO] ✅ Model %s pulled successfully", modelName)
				return &types.ProgressResponse{
					Status: "success",
				}, nil
			}

			// Parse progress data
			var progress types.ProgressResponse
			if err := json.Unmarshal(data, &progress); err == nil {
				// Call progress function if provided
				if fn != nil {
					fn(progress)
				}
				// Log progress at DEBUG level
				if progress.Status != "" {
					log.Printf("[ollama-plugin] [DEBUG] Pull progress: %s", progress.Status)
				}
			}

		case err := <-errChan:
			if err != nil {
				log.Printf("[ollama-plugin] [ERROR] Pull model %s failed: %v", modelName, err)
				return nil, fmt.Errorf("pull model failed: %w", err)
			}
		case <-ctx.Done():
			log.Printf("[ollama-plugin] [WARN] Pull model %s cancelled: %v", modelName, ctx.Err())
			return nil, ctx.Err()
		}
	}
}

// PullModelStream downloads a model with streaming progress
func (p *OllamaProvider) PullModelStream(ctx context.Context, req *types.PullModelRequest) (chan []byte, chan error) {
	// Prefer Model field, if empty use Name (backward compatible)
	modelName := req.Model
	if modelName == "" {
		modelName = req.Name
	}

	log.Printf("[ollama-plugin] [INFO] Pulling model (stream): %s", modelName)

	pullReq := map[string]interface{}{
		"name": modelName,
	}

	return p.client.StreamResponse(ctx, http.MethodPost, "/api/pull", "", pullReq)
}

// DeleteModel deletes a model
func (p *OllamaProvider) DeleteModel(ctx context.Context, req *types.DeleteRequest) error {
	log.Printf("[ollama-plugin] [INFO] Deleting model: %s", req.Model)
	deleteReq := map[string]interface{}{
		"name": req.Model,
	}

	if err := p.client.Do(ctx, http.MethodDelete, "/api/delete", "", deleteReq, nil); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Delete model %s failed: %v", req.Model, err)
		return fmt.Errorf("delete model failed: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] ✅ Model %s deleted successfully", req.Model)
	return nil
}

// ListModels lists available models
func (p *OllamaProvider) ListModels(ctx context.Context) (*types.ListResponse, error) {
	log.Printf("[ollama-plugin] [DEBUG] Listing models...")
	var lr types.ListResponse
	if err := p.client.Do(ctx, http.MethodGet, "/api/tags", "", nil, &lr); err != nil {
		log.Printf("[ollama-plugin] [ERROR] List models failed: %v", err)
		return nil, fmt.Errorf("list models failed: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] Found %d models", len(lr.Models))
	return &lr, nil
}

// LoadModel loads a model
func (p *OllamaProvider) LoadModel(ctx context.Context, req *types.LoadRequest) error {
	log.Printf("[ollama-plugin] [DEBUG] LoadModel called for: %s (Ollama auto-loads on first use)", req.Model)
	// Ollama automatically loads models on first request
	// No explicit load needed
	return nil
}

// UnloadModel unloads a model
func (p *OllamaProvider) UnloadModel(ctx context.Context, req *types.UnloadModelRequest) error {
	log.Printf("[ollama-plugin] [INFO] Unloading %d models: %v", len(req.Models), req.Models)
	// Unload by setting keep_alive to 0
	for _, model := range req.Models {
		log.Printf("[ollama-plugin] [DEBUG] Unloading model: %s", model)
		unloadReq := map[string]interface{}{
			"model":      model,
			"keep_alive": 0,
		}
		if err := p.client.Do(ctx, http.MethodPost, "/api/generate", "", unloadReq, nil); err != nil {
			log.Printf("[ollama-plugin] [ERROR] Unload model %s failed: %v", model, err)
			return fmt.Errorf("unload model %s failed: %w", model, err)
		}
	}
	log.Printf("[ollama-plugin] [INFO] ✅ Successfully unloaded %d models", len(req.Models))
	return nil
}

// GetRunningModels returns currently running models
func (p *OllamaProvider) GetRunningModels(ctx context.Context) (*types.ListResponse, error) {
	log.Printf("[ollama-plugin] [DEBUG] Getting running models...")
	var lr types.ListResponse
	if err := p.client.Do(ctx, http.MethodGet, "/api/ps", "", nil, &lr); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Get running models failed: %v", err)
		return nil, fmt.Errorf("get running models failed: %w", err)
	}

	log.Printf("[ollama-plugin] [INFO] Found %d running models", len(lr.Models))
	return &lr, nil
}

// =============== EngineInfoProvider Implementation ===============

// GetVersion returns engine version
func (p *OllamaProvider) GetVersion(ctx context.Context, resp *types.EngineVersionResponse) (*types.EngineVersionResponse, error) {
	log.Printf("[ollama-plugin] [DEBUG] Getting engine version...")
	var versionResp map[string]interface{}
	if err := p.client.Do(ctx, http.MethodGet, "/api/version", "", nil, &versionResp); err != nil {
		log.Printf("[ollama-plugin] [ERROR] Get version failed: %v", err)
		return nil, fmt.Errorf("get version failed: %w", err)
	}

	version := "unknown"
	if v, ok := versionResp["version"].(string); ok {
		version = v
	}

	log.Printf("[ollama-plugin] [INFO] Engine version: %s", version)
	return &types.EngineVersionResponse{
		Version: version,
	}, nil
}

// Note: GetOperateStatus and SetOperateStatus methods are already provided by LocalPluginAdapter, no need to re-implement
