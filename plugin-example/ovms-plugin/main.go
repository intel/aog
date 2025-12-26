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

	"github.com/intel/aog/plugin/examples/ovms-plugin/internal"
	"github.com/intel/aog/plugin/examples/ovms-plugin/internal/config"
)

var version = "1.0.0"

func main() {
	// CRITICAL: Write to stderr immediately to help diagnose startup issues
	fmt.Fprintf(os.Stderr, "[ovms-plugin] version %s starting...\n", version)
	fmt.Fprintf(os.Stderr, "[ovms-plugin] Working directory: %s\n", mustGetWd())
	fmt.Fprintf(os.Stderr, "[ovms-plugin] Executable path: %s\n", mustGetExe())

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ovms-plugin] FATAL: failed to load config: %v\n", err)
		fmt.Fprintf(os.Stderr, "[ovms-plugin] Plugin startup failed\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[ovms-plugin] Config loaded successfully\n")
	fmt.Fprintf(os.Stderr, "[ovms-plugin] DataDir: %s\n", cfg.DataDir)

	provider, err := internal.NewOvmsProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ovms-plugin] FATAL: failed to create provider: %v\n", err)
		fmt.Fprintf(os.Stderr, "[ovms-plugin] Plugin startup failed\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[ovms-plugin] Provider created successfully\n")
	fmt.Fprintf(os.Stderr, "[ovms-plugin] Starting plugin server...\n")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: server.PluginHandshake,
		Plugins: map[string]plugin.Plugin{
			server.PluginTypeProvider: server.NewProviderPlugin(provider),
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

func mustGetWd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "<unknown>"
	}
	return wd
}

func mustGetExe() string {
	exe, err := os.Executable()
	if err != nil {
		return "<unknown>"
	}
	return exe
}
