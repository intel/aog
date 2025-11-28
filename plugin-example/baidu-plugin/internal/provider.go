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
	"github.com/intel/aog/plugin/examples/baidu-plugin/internal/services"
)

// Compile-time assertions: ensure BaiduProvider implements all required SDK interfaces
var (
	_ client.PluginProvider       = (*BaiduProvider)(nil)
	_ client.RemotePluginProvider = (*BaiduProvider)(nil)
	_ client.StreamablePlugin     = (*BaiduProvider)(nil)
)

// BaiduProvider implements the RemotePluginProvider interface for Baidu
// Adapter pattern keeps the implementation minimal—override only custom logic
type BaiduProvider struct {
	*adapter.RemotePluginAdapter
	config *Config
	client *BaiduClient

	// Service handlers
	chatService         services.ServiceHandler
	embedService        services.ServiceHandler
	textToImageService  services.ServiceHandler
	textToSpeechService services.ServiceHandler
	speechToTextService services.ServiceHandler
	imageToImageService services.ServiceHandler
}

// NewBaiduProvider creates a new baidu provider using adapter pattern
func NewBaiduProvider(config *Config) (*BaiduProvider, error) {
	// Initialize remote adapter (build minimal manifest info from YAML)
	remoteAdapter := adapter.NewRemotePluginAdapter(&types.PluginManifest{})

	// Create Baidu API client
	client, err := NewBaiduClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create baidu client: %w", err)
	}

	// Create service handlers
	chatService := services.NewChatService(client)
	embedService := services.NewEmbedService(client)
	textToImageService := services.NewTextToImageService(client)
	textToSpeechService := services.NewTextToSpeechService(client)
	speechToTextService := services.NewSpeechToTextService(client)
	imageToImageService := services.NewImageToImageService(client)

	provider := &BaiduProvider{
		RemotePluginAdapter: remoteAdapter,
		config:              config,
		client:              client,
		chatService:         chatService,
		embedService:        embedService,
		textToImageService:  textToImageService,
		textToSpeechService: textToSpeechService,
		speechToTextService: speechToTextService,
		imageToImageService: imageToImageService,
	}

	return provider, nil
}

// =============== ServiceInvoker (Phase 3 Refactor) ===============

// InvokeService routes unary requests to matching service handlers
func (p *BaiduProvider) InvokeService(ctx context.Context, authInfo string, serviceName string, request []byte) ([]byte, error) {
	log.Printf("[baidu-plugin] [INFO] Invoking service: %s (unary)", serviceName)
	switch serviceName {
	case "chat":
		result, err := p.chatService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Chat service failed: %v", err)
		} else {
			log.Printf("[baidu-plugin] [DEBUG] Chat service completed successfully")
		}
		return result, err
	case "embed":
		result, err := p.embedService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[baidu-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	case "text-to-image":
		result, err := p.textToImageService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[baidu-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	case "text-to-speech":
		result, err := p.textToSpeechService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[baidu-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			log.Printf("[baidu-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
	default:
		log.Printf("[baidu-plugin] [ERROR] Unsupported service: %s", serviceName)
		return nil, fmt.Errorf("unsupported service: %s", serviceName)
	}
}

// ===== StreamablePlugin implementation =====

// InvokeServiceStream routes streaming requests to the appropriate handler
func (p *BaiduProvider) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string,
	request []byte,
) (<-chan client.StreamChunk, error) {
	log.Printf("[baidu-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
	ch := make(chan client.StreamChunk, 10)

	go func() {
		defer close(ch)
		defer log.Printf("[baidu-plugin] [DEBUG] Stream service %s completed", serviceName)

		switch serviceName {
		case "chat":
			if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
				log.Printf("[baidu-plugin] [DEBUG] Starting chat streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				log.Printf("[baidu-plugin] [ERROR] Chat service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("chat service does not support streaming"),
				}
			}
		default:
			log.Printf("[baidu-plugin] [ERROR] Service %s does not support streaming", serviceName)
			ch <- client.StreamChunk{
				Error: fmt.Errorf("service %s does not support streaming", serviceName),
			}
		}
	}()

	return ch, nil
}
