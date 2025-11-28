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
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/intel/aog/plugin-sdk/adapter"
	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/ollama-plugin/internal/services"
	"github.com/intel/aog/plugin/examples/ollama-plugin/internal/utils"
	"gopkg.in/yaml.v3"
)

// Compile-time interface checks: ensure OllamaProvider implements all SDK interfaces
var (
	_ client.PluginProvider      = (*OllamaProvider)(nil)
	_ client.LocalPluginProvider = (*OllamaProvider)(nil)
	_ client.StreamablePlugin    = (*OllamaProvider)(nil)
)

// OllamaProvider implements the LocalPluginProvider interface for Ollama
// Uses adapter pattern to greatly simplify implementation, only need to override methods that require customization
type OllamaProvider struct {
	*adapter.LocalPluginAdapter
	config *Config
	client *OllamaClient

	// Service handlers
	chatService     services.ServiceHandler
	embedService    services.ServiceHandler
	generateService services.ServiceHandler
}

// NewOllamaProvider creates a new Ollama provider using adapter pattern
func NewOllamaProvider(config *Config) (*OllamaProvider, error) {
	// Load plugin metadata
	manifest, err := loadManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create adapter
	localAdapter := adapter.NewLocalPluginAdapter(manifest)

	// Set engine address
	localAdapter.EngineHost = fmt.Sprintf("%s://%s", config.Scheme, config.Host)

	// Create client
	client, err := NewOllamaClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}

	// Create service handlers
	chatService := services.NewChatService(client)
	embedService := services.NewEmbedService(client)
	generateService := services.NewGenerateService(client)

	provider := &OllamaProvider{
		LocalPluginAdapter: localAdapter,
		config:             config,
		client:             client,
		chatService:        chatService,
		embedService:       embedService,
		generateService:    generateService,
	}

	// Initial status is running
	provider.SetOperateStatus(1)

	return provider, nil
}

// loadManifest loads metadata from plugin.yaml (static function)
func loadManifest() (*types.PluginManifest, error) {
	// Get plugin root directory (searches upward for plugin.yaml)
	pluginDir, err := utils.GetPluginDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin dir: %w", err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.yaml")

	// Read plugin.yaml
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
	}

	// Parse YAML
	var manifest types.PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.yaml: %w", err)
	}

	return &manifest, nil
}

// =============== ServiceInvoker (Phase 3 Refactor) ===============

// InvokeService unified service invocation interface - routes to corresponding service's unary handler
func (p *OllamaProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	log.Printf("[ollama-plugin] [INFO] Invoking service: %s (unary)", serviceName)
	switch serviceName {
	case "chat":
		result, err := p.chatService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Chat service failed: %v", err)
		} else {
			log.Printf("[ollama-plugin] [DEBUG] Chat service completed successfully")
		}
		return result, err
	case "embed":
		result, err := p.embedService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[ollama-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	case "generate":
		result, err := p.generateService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[ollama-plugin] [ERROR] Generate service failed: %v", err)
		} else {
			log.Printf("[ollama-plugin] [DEBUG] Generate service completed successfully")
		}
		return result, err
	default:
		log.Printf("[ollama-plugin] [ERROR] Unsupported service: %s", serviceName)
		return nil, fmt.Errorf("unsupported service: %s", serviceName)
	}
}

// ===== Implement StreamablePlugin Interface =====

// InvokeServiceStream implements streaming service invocation - routes to corresponding service's streaming handler
func (p *OllamaProvider) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string, // Local plugin doesn't need auth, but keeps interface consistency
	request []byte,
) (<-chan client.StreamChunk, error) {
	log.Printf("[ollama-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
	ch := make(chan client.StreamChunk, 10)

	go func() {
		defer close(ch)
		defer log.Printf("[ollama-plugin] [DEBUG] Stream service %s completed", serviceName)

		switch serviceName {
		case "chat":
			if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
				log.Printf("[ollama-plugin] [DEBUG] Starting chat streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				log.Printf("[ollama-plugin] [ERROR] Chat service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("chat service does not support streaming"),
				}
			}
		case "generate":
			if streamingHandler, ok := p.generateService.(services.StreamingHandler); ok {
				log.Printf("[ollama-plugin] [DEBUG] Starting generate streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				log.Printf("[ollama-plugin] [ERROR] Generate service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("generate service does not support streaming"),
				}
			}
		default:
			log.Printf("[ollama-plugin] [ERROR] Service %s does not support streaming", serviceName)
			ch <- client.StreamChunk{
				Error: fmt.Errorf("service %s does not support streaming", serviceName),
			}
		}
	}()

	return ch, nil
}
