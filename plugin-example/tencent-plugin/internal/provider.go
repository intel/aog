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

	"github.com/intel/aog/plugin-sdk/adapter"
	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/tencent-plugin/internal/services"
)

// Compile-time assertions: ensure TencentProvider implements all SDK interfaces
var (
	_ client.PluginProvider       = (*TencentProvider)(nil)
	_ client.RemotePluginProvider = (*TencentProvider)(nil)
	_ client.StreamablePlugin     = (*TencentProvider)(nil)
)

// TencentProvider implements the RemotePluginProvider interface for Tencent
// Adapter pattern keeps implementation minimal—override only custom logic
type TencentProvider struct {
	*adapter.RemotePluginAdapter
	config *Config
	client *TencentClient

	// Service handlers
	chatService         services.ServiceHandler
	embedService        services.ServiceHandler
	textToImageService  services.ServiceHandler
	textToSpeechService services.ServiceHandler
}

// NewTencentProvider creates a new tencent provider using adapter pattern
func NewTencentProvider(config *Config) (*TencentProvider, error) {
	// Initialize remote adapter (build minimal manifest info from YAML)
	remoteAdapter := adapter.NewRemotePluginAdapter(&types.PluginManifest{})

	// Create client
	client, err := NewTencentClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tencent client: %w", err)
	}

	// Create service handlers
	chatService := services.NewChatService(client)
	embedService := services.NewEmbedService(client)
	textToImageService := services.NewTextToImageService(client)
	textToSpeechService := services.NewTextToSpeechService(client)

	provider := &TencentProvider{
		RemotePluginAdapter: remoteAdapter,
		config:              config,
		client:              client,
		chatService:         chatService,
		embedService:        embedService,
		textToImageService:  textToImageService,
		textToSpeechService: textToSpeechService,
	}

	return provider, nil
}

// =============== ServiceInvoker (Phase 3 Refactor) ===============

// InvokeService routes unary requests to matching handlers
func (p *TencentProvider) InvokeService(ctx context.Context, authInfo string, serviceName string, request []byte) ([]byte, error) {
	log.Printf("[tencent-plugin] [INFO] Invoking service: %s (unary)", serviceName)
	switch serviceName {
	case "chat":
		result, err := p.chatService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[tencent-plugin] [ERROR] Chat service failed: %v", err)
		} else {
			log.Printf("[tencent-plugin] [DEBUG] Chat service completed successfully")
		}
		return result, err
	case "embed":
		result, err := p.embedService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[tencent-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[tencent-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	case "text-to-image":
		result, err := p.textToImageService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[tencent-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[tencent-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	case "text-to-speech":
		result, err := p.textToSpeechService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[tencent-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[tencent-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	default:
		log.Printf("[tencent-plugin] [ERROR] Unsupported service: %s", serviceName)
		return nil, fmt.Errorf("unsupported service: %s", serviceName)
	}
}

// ===== StreamablePlugin implementation =====

// InvokeServiceStream routes streaming requests to the appropriate handler
func (p *TencentProvider) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string,
	request []byte,
) (<-chan client.StreamChunk, error) {
	log.Printf("[tencent-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
	ch := make(chan client.StreamChunk, 10)

	go func() {
		defer close(ch)
		defer log.Printf("[tencent-plugin] [DEBUG] Stream service %s completed", serviceName)

		switch serviceName {
		case "chat":
			if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
				log.Printf("[tencent-plugin] [DEBUG] Starting chat streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				log.Printf("[tencent-plugin] [ERROR] Chat service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("chat service does not support streaming"),
				}
			}
		default:
			log.Printf("[tencent-plugin] [ERROR] Service %s does not support streaming", serviceName)
			ch <- client.StreamChunk{
				Error: fmt.Errorf("service %s does not support streaming", serviceName),
			}
		}
	}()

	return ch, nil
}
