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

package adapter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/intel/aog/plugin-sdk/client"
	"github.com/intel/aog/plugin-sdk/types"
)

var (
	_ client.PluginProvider       = (*RemotePluginAdapter)(nil)
	_ client.RemotePluginProvider = (*RemotePluginAdapter)(nil)
)

// RemotePluginAdapter is an adapter for remote API plugins.
//
// Designed specifically for Remote-type plugins, it provides authentication management interfaces.
// Remote plugins integrate with cloud AI services (e.g., OpenAI, Anthropic), require authentication
// management, but don't need local engine lifecycle management or model installation.
type RemotePluginAdapter struct {
	*BasePluginProvider
	credentials *types.Credentials
}

// NewRemotePluginAdapter creates a new remote plugin adapter.
func NewRemotePluginAdapter(manifest *types.PluginManifest) *RemotePluginAdapter {
	return &RemotePluginAdapter{
		BasePluginProvider: NewBasePluginProvider(manifest),
	}
}

// ===== Authentication Management Interface (requires plugin implementation) =====

// SetAuth sets authentication information
//
// Plugin needs to implement this method to save authentication credentials.
func (r *RemotePluginAdapter) SetAuth(req *http.Request, authType string, credentials map[string]string) error {
	r.credentials = &types.Credentials{
		Type:   authType,
		Values: credentials,
	}

	r.LogInfo("Authentication credentials set")
	return nil
}

// ValidateAuth validates authentication information
//
// Plugin needs to implement this method to validate if credentials are valid (e.g., by calling API test).
func (r *RemotePluginAdapter) ValidateAuth(ctx context.Context) error {
	if r.credentials == nil {
		return r.WrapError("validate_auth", fmt.Errorf("no credentials set"))
	}

	return r.WrapError("validate_auth", fmt.Errorf("ValidateAuth must be implemented by plugin"))
}

// RefreshAuth refreshes authentication information
//
// Plugin needs to implement this method to refresh expired authentication tokens (OAuth, etc.).
func (r *RemotePluginAdapter) RefreshAuth(ctx context.Context) error {
	return r.WrapError("refresh_auth", fmt.Errorf("RefreshAuth must be implemented by plugin"))
}

// GetCredentials returns the current authentication credentials.
func (r *RemotePluginAdapter) GetCredentials() *types.Credentials {
	return r.credentials
}

// IsAuthenticated checks if authentication credentials are set.
func (r *RemotePluginAdapter) IsAuthenticated() bool {
	return r.credentials != nil && len(r.credentials.Values) > 0
}
