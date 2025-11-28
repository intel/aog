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

package provider

import (
	"fmt"

	"github.com/intel/aog/internal/provider/engine"
	sdktypes "github.com/intel/aog/plugin-sdk/types"
)

// ProviderFactory defines the factory interface for obtaining Providers.
type ProviderFactory interface {
	// GetProvider returns a Provider instance by name.
	GetProvider(name string) (ModelServiceProvider, error)

	// ListAvailableProviders returns all available Provider names.
	ListAvailableProviders() []string
}

// BuiltinProviderFactory implements built-in Provider factory.
type BuiltinProviderFactory struct {
	providers map[string]func(*sdktypes.EngineRecommendConfig) ModelServiceProvider
}

// NewBuiltinProviderFactory creates a built-in Provider factory.
func NewBuiltinProviderFactory() *BuiltinProviderFactory {
	return &BuiltinProviderFactory{
		providers: map[string]func(*sdktypes.EngineRecommendConfig) ModelServiceProvider{
			"ollama": func(config *sdktypes.EngineRecommendConfig) ModelServiceProvider {
				return engine.NewOllamaProvider(config)
			},
			"openvino": func(config *sdktypes.EngineRecommendConfig) ModelServiceProvider {
				return engine.NewOpenvinoProvider(config)
			},
		},
	}
}

// GetProvider returns a Provider instance by name.
func (f *BuiltinProviderFactory) GetProvider(name string) (ModelServiceProvider, error) {
	constructor, exists := f.providers[name]
	if !exists {
		return nil, fmt.Errorf("engine not found or not enabled: %s", name)
	}
	return constructor(nil), nil
}

// ListAvailableProviders returns all available Provider names.
func (f *BuiltinProviderFactory) ListAvailableProviders() []string {
	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}

// CompositeProviderFactory composes multiple ProviderFactories.
// Retrieves Providers from multiple factories in order, supporting coexistence of built-in engines and plugins.
type CompositeProviderFactory struct {
	factories []ProviderFactory
}

// NewCompositeProviderFactory creates a composite factory.
// Factories are ordered by priority, with the first factory having the highest priority.
func NewCompositeProviderFactory(factories ...ProviderFactory) *CompositeProviderFactory {
	return &CompositeProviderFactory{
		factories: factories,
	}
}

// GetProvider retrieves a Provider from factories in order.
// Searches from the first factory and returns upon finding.
func (f *CompositeProviderFactory) GetProvider(name string) (ModelServiceProvider, error) {
	for _, factory := range f.factories {
		provider, err := factory.GetProvider(name)
		if err == nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("engine not found or not enabled: %s", name)
}

// ListAvailableProviders lists available Providers from all factories (deduplicated).
func (f *CompositeProviderFactory) ListAvailableProviders() []string {
	nameSet := make(map[string]bool)
	var names []string

	for _, factory := range f.factories {
		for _, name := range factory.ListAvailableProviders() {
			if !nameSet[name] {
				nameSet[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}
