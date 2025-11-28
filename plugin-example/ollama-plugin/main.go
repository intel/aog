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

package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-plugin"
	"github.com/intel/aog/plugin-sdk/server"
	"github.com/intel/aog/plugin/examples/ollama-plugin/internal"
)

func main() {
	// Load configuration
	config, err := internal.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Create provider
	provider, err := internal.NewOllamaProvider(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Ollama provider: %v\n", err)
		os.Exit(1)
	}

	// Setup plugin using SDK
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: server.PluginHandshake,
		Plugins: map[string]plugin.Plugin{
			server.PluginTypeProvider: server.NewProviderPlugin(provider),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
