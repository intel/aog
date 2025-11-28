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
	"github.com/intel/aog/plugin/examples/deepseek-plugin/internal/services"
)

// Compile-time assertions: ensure DeepseekProvider implements all SDK interfaces
var (
	_ client.PluginProvider       = (*DeepseekProvider)(nil)
	_ client.RemotePluginProvider = (*DeepseekProvider)(nil)
	_ client.StreamablePlugin     = (*DeepseekProvider)(nil)
)

// DeepseekProvider implements the RemotePluginProvider interface for Deepseek
// Adapter pattern keeps the implementation minimal—override only custom logic
type DeepseekProvider struct {
	*adapter.RemotePluginAdapter
	config *Config
	client *DeepseekClient

	// Service handlers
	chatService services.ServiceHandler
}

// NewDeepseekProvider creates a new Deepseek provider using adapter pattern
func NewDeepseekProvider(config *Config) (*DeepseekProvider, error) {
	// Initialize remote adapter (build minimal manifest info from YAML)
	remoteAdapter := adapter.NewRemotePluginAdapter(&types.PluginManifest{})

	// Create client
	client, err := NewDeepseekClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Deepseek client: %w", err)
	}

	// Create service handlers
	chatService := services.NewChatService(client)

	provider := &DeepseekProvider{
		RemotePluginAdapter: remoteAdapter,
		config:              config,
		client:              client,
		chatService:         chatService,
	}

	return provider, nil
}

// =============== ServiceInvoker (Phase 3 Refactor) ===============

// InvokeService routes unary requests to matching handlers
func (p *DeepseekProvider) InvokeService(ctx context.Context, authInfo string, serviceName string, request []byte) ([]byte, error) {
	log.Printf("[deepseek-plugin] [INFO] Invoking service: %s (unary)", serviceName)
	switch serviceName {
	case "chat":
		result, err := p.chatService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			log.Printf("[deepseek-plugin] [ERROR] Chat service failed: %v", err)
		} else {
			log.Printf("[deepseek-plugin] [DEBUG] Chat service completed successfully")
		}
		return result, err
	default:
		log.Printf("[deepseek-plugin] [ERROR] Unsupported service: %s", serviceName)
		return nil, fmt.Errorf("unsupported service: %s", serviceName)
	}
}

// ===== StreamablePlugin implementation =====

// InvokeServiceStream routes streaming requests appropriately
func (p *DeepseekProvider) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string,
	request []byte,
) (<-chan client.StreamChunk, error) {
	log.Printf("[deepseek-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
	ch := make(chan client.StreamChunk, 10)

	go func() {
		defer close(ch)
		defer log.Printf("[deepseek-plugin] [DEBUG] Stream service %s completed", serviceName)

		switch serviceName {
		case "chat":
			if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
				log.Printf("[deepseek-plugin] [DEBUG] Starting chat streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				log.Printf("[deepseek-plugin] [ERROR] Chat service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("chat service does not support streaming"),
				}
			}
		default:
			log.Printf("[deepseek-plugin] [ERROR] Service %s does not support streaming", serviceName)
			ch <- client.StreamChunk{
				Error: fmt.Errorf("service %s does not support streaming", serviceName),
			}
		}
	}()

	return ch, nil
}
