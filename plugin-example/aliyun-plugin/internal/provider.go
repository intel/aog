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

	"github.com/intel/aog/plugin-sdk/adapter"
	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/types"
	"github.com/intel/aog/plugin/examples/aliyun-plugin/internal/services"
)

// Compile-time interface assertions to ensure AliyunProvider implements all SDK interfaces
var (
	_ client.PluginProvider       = (*AliyunProvider)(nil)
	_ client.RemotePluginProvider = (*AliyunProvider)(nil)
	_ client.StreamablePlugin     = (*AliyunProvider)(nil)
	_ client.BidirectionalPlugin  = (*AliyunProvider)(nil)
)

// AliyunProvider implements the RemotePluginProvider interface for Aliyun
// Adapter pattern keeps the implementation minimal—override only what needs customization
type AliyunProvider struct {
	*adapter.RemotePluginAdapter
	config *Config
	client *AliyunClient

	// Service handlers
	chatService           services.ServiceHandler
	embedService          services.ServiceHandler
	textToImageService    services.ServiceHandler
	imageToImageService   services.ServiceHandler
	textToSpeechService   services.ServiceHandler
	speechToTextService   services.ServiceHandler
	speechToTextWSService services.WebsocketHandler
}

// NewAliyunProvider creates a new aliyun provider using adapter pattern
func NewAliyunProvider(config *Config) (*AliyunProvider, error) {
	// Initialize remote adapter (construct minimal manifest info from YAML)
	remoteAdapter := adapter.NewRemotePluginAdapter(&types.PluginManifest{})

	// Create API client
	client, err := NewAliyunClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun client: %w", err)
	}

	// Create service handlers
	chatService := services.NewChatService(client)
	embedService := services.NewEmbedService(client)
	textToImageService := services.NewTextToImageService(client)
	imageToImageService := services.NewImageToImageService(client)
	textToSpeechService := services.NewTextToSpeechService(client)
	speechToTextService := services.NewSpeechToTextService(client)
	speechToTextWSService := services.NewSpeechToTextWSService(client)

	provider := &AliyunProvider{
		RemotePluginAdapter:   remoteAdapter,
		config:                config,
		client:                client,
		chatService:           chatService,
		embedService:          embedService,
		textToImageService:    textToImageService,
		imageToImageService:   imageToImageService,
		textToSpeechService:   textToSpeechService,
		speechToTextService:   speechToTextService,
		speechToTextWSService: speechToTextWSService,
	}

	return provider, nil
}

// =============== ServiceInvoker (Phase 3 Refactor) ===============

// InvokeService routes unary requests to matching service handlers
func (p *AliyunProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	// Add detailed debug logging
	LogicLogger.Info("[aliyun-plugin] [DEBUG] InvokeService called: serviceName=%q (len=%d, bytes=%v)",
		serviceName, len(serviceName), []byte(serviceName))

	switch serviceName {
	case "chat":
		result, err := p.chatService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Chat service failed: %v", err)
		} else {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Chat service completed successfully")
		}
		return result, err
	case "embed":
		LogicLogger.Info("[aliyun-plugin] [DEBUG] Matched 'embed' case, serviceName=%q", serviceName)
		result, err := p.embedService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Embed service failed: %v", err)
		} else {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Embed service completed successfully")
		}
		return result, err
		// return result, fmt.Errorf("%w, sn: %s, AuthInfo: %s", err, serviceName, authInfo)
	case "text-to-image":
		LogicLogger.Info("[aliyun-plugin] [DEBUG] Matched 'text-to-image' case, serviceName=%q", serviceName)
		result, err := p.textToImageService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Text-to-image service failed: %v, authInfo: %s", err, authInfo)
		} else {
			LogicLogger.Info("[aliyun-plugin] [DEBUG]  Text-to-image service completed successfully")
		}
		return result, err
	case "text-to-speech":
		result, err := p.textToSpeechService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Text-to-speech service failed: %v", err)
		} else {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Text-to-speech service completed successfully")
		}
		return result, err
	case "speech-to-text":
		result, err := p.speechToTextService.HandleUnary(ctx, authInfo, request)
		if err != nil {
			LogicLogger.Info("[aliyun-plugin] [ERROR] Text-to-speech service failed: %v", err)
		} else {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Text-to-speech service completed successfully")
		}
		return result, err
	default:
		LogicLogger.Info("[aliyun-plugin] [ERROR] Unsupported service: %q (len=%d, bytes=%v)",
			serviceName, len(serviceName), []byte(serviceName))
		return nil, fmt.Errorf("unsupported service: %s", serviceName)
	}
}

func (p *AliyunProvider) InvokeServiceBidirectional(
	ctx context.Context,
	serviceName string,
	wsConnID string,
	authInfo string,
	inStream <-chan client.BidiMessage,
	outStream chan<- client.BidiMessage,
) error {
	defer func() {
		if err := recover(); err != nil {
			LogicLogger.Info("[aliyun-plugin] [DEBUG] Invoke service bidirectional", err)
		}
	}()

	switch serviceName {
	case "speech-to-text-ws":
		err := p.speechToTextWSService.HandleWebsocket(ctx, wsConnID, authInfo, inStream, outStream)
		return err
	default:
		return fmt.Errorf("unsupported bidirectional service: %s, auth_info：%s", serviceName, authInfo)
	}
}

// ===== Implement StreamablePlugin interface =====

// InvokeServiceStream routes streaming requests to the corresponding handler
func (p *AliyunProvider) InvokeServiceStream(
	ctx context.Context,
	serviceName string,
	authInfo string,
	request []byte,
) (<-chan client.StreamChunk, error) {
	LogicLogger.Info("[aliyun-plugin] [INFO] Invoking service: %s (streaming)", serviceName)
	ch := make(chan client.StreamChunk, 10)

	go func() {
		defer close(ch)
		defer LogicLogger.Info("[aliyun-plugin] [DEBUG] Stream service %s completed", serviceName)

		switch serviceName {
		case "chat":
			if streamingHandler, ok := p.chatService.(services.StreamingHandler); ok {
				LogicLogger.Info("[aliyun-plugin] [DEBUG] Starting chat streaming...")
				streamingHandler.HandleStreaming(ctx, authInfo, request, ch)
			} else {
				LogicLogger.Info("[aliyun-plugin] [ERROR] Chat service does not support streaming")
				ch <- client.StreamChunk{
					Error: fmt.Errorf("chat service does not support streaming"),
				}
			}
		default:
			LogicLogger.Info("[aliyun-plugin] [ERROR] Service %s does not support streaming", serviceName)
			ch <- client.StreamChunk{
				Error: fmt.Errorf("service %s does not support streaming", serviceName),
			}
		}
	}()

	return ch, nil
}
