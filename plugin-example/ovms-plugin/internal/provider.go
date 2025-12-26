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
	"os/exec"

	"github.com/intel/aog/plugin-sdk/adapter"
	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/config"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/grpc/streaming"
	grpcservices "github.com/intel/aog/plugin/examples/ovms-plugin/internal/services/grpc"
	httpservices "github.com/intel/aog/plugin/examples/ovms-plugin/internal/services/http"
)

// Compile-time interface checks
var (
	_ client.PluginProvider      = (*OvmsProvider)(nil)
	_ client.LocalPluginProvider = (*OvmsProvider)(nil)
	_ client.StreamablePlugin    = (*OvmsProvider)(nil)
	_ client.BidirectionalPlugin = (*OvmsProvider)(nil)
)

type OvmsProvider struct {
	*adapter.LocalPluginAdapter
	config *config.Config
	cmd    *exec.Cmd // OVMS process

	// Service handlers
	chatHandler           *httpservices.ChatHandler
	embedHandler          *httpservices.EmbedHandler
	generateHandler       *httpservices.GenerateHandler
	rerankHandler         *httpservices.RerankHandler
	textToImageHandler    *httpservices.TextToImageHandler
	speechToTextHandler   *grpcservices.SpeechToTextHandler
	textToSpeechHandler   *grpcservices.TextToSpeechHandler
	speechToTextWSHandler *grpcservices.SpeechToTextWSHandler
}

func NewOvmsProvider(config *config.Config) (*OvmsProvider, error) {
	if config == nil || config.Manifest == nil {
		return nil, fmt.Errorf("invalid config")
	}

	localAdapter := adapter.NewLocalPluginAdapter(config.Manifest)
	localAdapter.EngineHost = config.EngineHost

	p := &OvmsProvider{
		LocalPluginAdapter: localAdapter,
		config:             config,
	}

	// Initialize service handlers with OVMS configuration from config
	// Get endpoints from manifest for each service
	getEndpoint := func(serviceName string) string {
		if svc, err := config.Manifest.GetServiceByName(serviceName); err == nil {
			return svc.Endpoint
		}
		return "" // fallback to empty if not found
	}

	// HTTP services use port 16666 (matching built-in OpenVINO)
	p.chatHandler = httpservices.NewChatHandler(config.OVMSHost, config.OVMSHTTPPort, getEndpoint("chat"))
	p.embedHandler = httpservices.NewEmbedHandler(config.OVMSHost, config.OVMSHTTPPort, getEndpoint("embed"))
	p.generateHandler = httpservices.NewGenerateHandler(config.OVMSHost, config.OVMSHTTPPort, getEndpoint("generate"))
	p.rerankHandler = httpservices.NewRerankHandler(config.OVMSHost, config.OVMSHTTPPort, getEndpoint("rerank"))
	p.textToImageHandler = httpservices.NewTextToImageHandler(config.OVMSHost, config.OVMSHTTPPort, getEndpoint("text-to-image"))
	// gRPC services use port 9000 (matching built-in OpenVINO)
	p.speechToTextHandler = grpcservices.NewSpeechToTextHandler(config.OVMSHost, config.OVMSGRPCPort, getEndpoint("speech-to-text"))
	p.textToSpeechHandler = grpcservices.NewTextToSpeechHandler(config.OVMSHost, config.OVMSGRPCPort, getEndpoint("text-to-speech"))
	p.speechToTextWSHandler = grpcservices.NewSpeechToTextWSHandler(config.OVMSHost, config.OVMSGRPCPort, getEndpoint("speech-to-text-ws"))

	// Validate manifest before setting status
	if err := p.ValidateManifest(); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	p.SetOperateStatus(1)
	p.LogInfo("Provider initialized")
	return p, nil
}

// ========== ServiceInvoker ==========

func (p *OvmsProvider) InvokeService(ctx context.Context, serviceName string, authInfo string, request []byte) ([]byte, error) {
	type ServiceHandler interface {
		Handle(ctx context.Context, request []byte) ([]byte, error)
		HandleStream(ctx context.Context, request []byte) (<-chan []byte, error)
	}
	var handler ServiceHandler
	switch serviceName {
	case "chat":
		handler = p.chatHandler
	case "embed":
		handler = p.embedHandler
	case "generate":
		handler = p.generateHandler
	case "rerank":
		handler = p.rerankHandler
	case "text-to-image":
		handler = p.textToImageHandler
	case "speech-to-text":
		handler = p.speechToTextHandler
	case "text-to-speech":
		handler = p.textToSpeechHandler
	case "speech-to-text-ws":
		handler = p.speechToTextWSHandler
	default:
		return nil, fmt.Errorf("unknown service: %s", serviceName)
	}

	return handler.Handle(ctx, request)
}

// ========== StreamablePlugin ==========

func (p *OvmsProvider) InvokeServiceStream(ctx context.Context, serviceName string, authInfo string, request []byte) (<-chan client.StreamChunk, error) {
	type ServiceHandler interface {
		Handle(ctx context.Context, request []byte) ([]byte, error)
		HandleStream(ctx context.Context, request []byte) (<-chan []byte, error)
	}
	var handler ServiceHandler
	switch serviceName {
	case "chat":
		handler = p.chatHandler
	case "embed":
		handler = p.embedHandler
	case "generate":
		handler = p.generateHandler
	case "rerank":
		handler = p.rerankHandler
	case "text-to-image":
		handler = p.textToImageHandler
	case "speech-to-text":
		handler = p.speechToTextHandler
	case "text-to-speech":
		handler = p.textToSpeechHandler
	case "speech-to-text-ws":
		handler = p.speechToTextWSHandler
	default:
		ch := make(chan client.StreamChunk, 1)
		ch <- client.StreamChunk{
			Error: fmt.Errorf("unknown service: %s", serviceName),
		}
		close(ch)
		return ch, nil
	}

	// Get stream from handler
	streamCh, err := handler.HandleStream(ctx, request)
	if err != nil {
		ch := make(chan client.StreamChunk, 1)
		ch <- client.StreamChunk{
			Error: err,
		}
		close(ch)
		return ch, nil
	}

	// Convert service stream to SDK stream
	resultCh := make(chan client.StreamChunk, 10)
	go func() {
		defer close(resultCh)
		for data := range streamCh {
			resultCh <- client.StreamChunk{
				Data: data,
				Metadata: map[string]string{
					"content-type": "text/event-stream",
				},
			}
		}
	}()

	return resultCh, nil
}

// ========== BidirectionalPlugin ==========

func (p *OvmsProvider) InvokeServiceBidirectional(
	ctx context.Context,
	serviceName string,
	wsConnID string,
	authInfo string,
	inStream <-chan client.BidiMessage,
	outStream chan<- client.BidiMessage,
) error {
	// Only speech-to-text-ws supports bidirectional communication
	if serviceName != "speech-to-text-ws" {
		return p.WrapError("invoke_service_bidirectional",
			fmt.Errorf("service %s does not support bidirectional communication", serviceName))
	}

	// Create gRPC bidirectional stream handler
	grpcHandler := streaming.NewGRPCBidiStreamHandler(
		p.config.OVMSHost,
		p.config.OVMSGRPCPort,
		p, // OvmsProvider implements LoggerAdapter
	)

	// Handle bidirectional communication using gRPC stream
	// This is completely independent and uses only plugin-sdk BidiMessage interface
	return grpcHandler.HandleBidirectional(ctx, wsConnID, inStream, outStream)
}

// ========== EngineLifecycleManager & EngineInstaller ==========
// Implemented in engine.go

// CheckEngine wrapper for LocalPluginProvider interface (no context)
func (p *OvmsProvider) CheckEngine() (bool, error) {
	ctx := context.Background()
	return p.CheckEngineCtx(ctx)
}

// InstallEngine, InitEnv, UpgradeEngine implemented in installer.go

// ========== ModelManager ==========
// Implemented in models.go

// ========== EngineInfoProvider ==========
// GetVersion implemented in engine.go
