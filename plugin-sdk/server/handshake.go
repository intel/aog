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

import "github.com/hashicorp/go-plugin"

// PluginHandshake defines the plugin handshake configuration.
// Host and Plugin must use the same handshake configuration to communicate.
//
// This is go-plugin's security mechanism to ensure only compatible plugins can be loaded.
var PluginHandshake = plugin.HandshakeConfig{
	// ProtocolVersion is the version of the plugin protocol.
	// This needs to be incremented when the protocol becomes incompatible.
	ProtocolVersion: 1,

	// MagicCookie is a key-value pair for basic verification.
	// Ensures only AOG plugins can be loaded.
	MagicCookieKey:   "AOG_PLUGIN",
	MagicCookieValue: "aog-provider-plugin-v1",
}

// PluginTypeProvider is the identifier for Provider plugins.
// Used to register and look up plugins in PluginMap.
const PluginTypeProvider = "provider"

// PluginMap defines the plugin type mapping.
//
// This is the core mechanism of go-plugin:
//   - Host side uses this map to find and connect to plugins
//   - Plugin side uses this map to register services
//
// CRITICAL: Host and Plugin MUST use the exact same PluginMap instance!
// This is why this definition must be in the SDK, not separately in Host and Plugin.
var PluginMap = map[string]plugin.Plugin{
	PluginTypeProvider: &ProviderPlugin{},
}

// DefaultHandshake is an alias of PluginHandshake for backward compatibility.
var DefaultHandshake = PluginHandshake
