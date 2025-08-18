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

package common

import (
	"fmt"

	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
)

// ValidateHybridPolicy validates hybrid policy parameter
func ValidateHybridPolicy(hybridPolicy string) error {
	if !utils.Contains(types.SupportHybridPolicy, hybridPolicy) {
		return fmt.Errorf("invalid hybrid_policy value: %s, Allowed values are: %v", hybridPolicy, types.SupportHybridPolicy)
	}
	return nil
}

// ValidateServiceName validates service name parameter
func ValidateServiceName(serviceName string) error {
	if !utils.Contains(types.SupportService, serviceName) {
		return fmt.Errorf("unsupported service types: %s", serviceName)
	}
	return nil
}

// ValidateFlavor validates flavor parameter
func ValidateFlavor(flavorName string) error {
	if flavorName != types.FlavorTencent && flavorName != types.FlavorDeepSeek &&
		flavorName != types.FlavorOllama && flavorName != types.FlavorOpenAI {
		return fmt.Errorf("invalid flavor: %s", flavorName)
	}
	return nil
}

// ValidateServiceProvider validates service provider configuration
func ValidateServiceProvider(serviceName, serviceSource, apiFlavor, authType, authKey string) error {
	if serviceName == "" || serviceSource == "" || apiFlavor == "" {
		return fmt.Errorf("service_name, service_source, flavor_name are required")
	}

	if authType != "none" && authKey == "" {
		return fmt.Errorf("auth_key is required when auth_type is not none")
	}

	return nil
}

// ValidateRequiredFlag validates that a required flag is provided
func ValidateRequiredFlag(flagName, flagValue string) error {
	if flagValue == "" {
		return fmt.Errorf("%s is required", flagName)
	}
	return nil
}

// ValidateFileExists validates that a file exists
func ValidateFileExists(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path is required")
	}
	return nil
}
