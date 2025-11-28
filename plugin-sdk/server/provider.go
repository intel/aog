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

package server

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/protocol"
)

// ProviderPlugin implements the hashicorp/go-plugin.Plugin interface.
//
// This is the core component of the Plugin SDK, responsible for:
//   - Registering gRPC services on the plugin side
//   - Creating gRPC clients on the host side
//
// Plugin developers only need to create this object and pass it to plugin.Serve().
type ProviderPlugin struct {
	plugin.Plugin

	// Impl is the actual provider implementation.
	// Can be *adapter.BasePluginProvider, *adapter.LocalPluginAdapter or *adapter.RemotePluginAdapter.
	Impl interface{}
}

// NewProviderPlugin creates a ProviderPlugin instance.
//
// Parameters:
//   - impl: The actual plugin implementation (usually *adapter.LocalPluginAdapter or *adapter.RemotePluginAdapter)
//
// Returns:
//   - *ProviderPlugin: A Plugin instance that can be passed to plugin.Serve()
func NewProviderPlugin(impl interface{}) *ProviderPlugin {
	return &ProviderPlugin{
		Impl: impl,
	}
}

// GRPCServer registers gRPC services on the plugin side.
//
// When AOG Core starts the plugin process, this method is called to register its gRPC services.
// This method is automatically called by the go-plugin framework.
func (p *ProviderPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	debugFile, _ := os.OpenFile("/tmp/aog_sdk_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if debugFile != nil {
		fmt.Fprintf(debugFile, "[GRPCServer] Method called! p.Impl type = %T\n", p.Impl)
		debugFile.Close()
	}

	server := NewGRPCProviderServer(p.Impl)
	protocol.RegisterProviderServiceServer(s, server)
	return nil
}

// GRPCClient creates a gRPC client on the host side.
//
// When AOG Core connects to a plugin, this method is called to create a client proxy.
// This method is automatically called by the go-plugin framework.
//
// Returns a wrapped GRPCProviderClient that implements:
//   - client.PluginProvider
//   - client.LocalPluginProvider
//   - client.RemotePluginProvider
//
// AOG Core can directly type-assert the return value to these interfaces.
func (p *ProviderPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return client.NewGRPCProviderClient(c), nil
}
