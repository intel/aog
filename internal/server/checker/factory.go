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

package checker

import (
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/types"
)

// ServiceCheckerFactory creates appropriate service checkers
type ServiceCheckerFactory struct{}

// CreateServiceChecker creates a service checker based on service provider and model
func (f *ServiceCheckerFactory) CreateServiceChecker(sp types.ServiceProvider, modelName string) ServiceChecker {
	if sp.ServiceSource == types.ServiceSourceLocal {
		return &LocalServiceChecker{
			BaseServiceChecker: BaseServiceChecker{
				ServiceProvider: sp,
				ModelName:       modelName,
			},
		}
	}

	// For remote services, create appropriate request builder
	var requestBuilder RequestBuilder
	switch sp.ServiceName {
	case types.ServiceModels:
		requestBuilder = &ModelsRequestBuilder{}
	case types.ServiceChat:
		requestBuilder = &ChatRequestBuilder{}
	case types.ServiceEmbed:
		requestBuilder = &EmbeddingRequestBuilder{}
	case types.ServiceTextToImage:
		requestBuilder = &TextToImageRequestBuilder{Flavor: sp.Flavor}
	case types.ServiceTextToSpeech:
		requestBuilder = &TextToSpeechRequestBuilder{Flavor: sp.Flavor}
	case types.ServiceGenerate,
		types.ServiceSpeechToText,
		types.ServiceSpeechToTextWS,
		types.ServiceTextToVideo,
		types.ServiceImageToVideo,
		types.ServiceImageToImage:
		// These services return true without actual checking
		requestBuilder = nil
	default:
		logger.LogicLogger.Error("[ServiceChecker] Unknown service name", "service", sp.ServiceName)
		return nil
	}

	return &RemoteServiceChecker{
		BaseServiceChecker: BaseServiceChecker{
			ServiceProvider: sp,
			ModelName:       modelName,
		},
		RequestBuilder: requestBuilder,
	}
}

// Global factory instance
var DefaultFactory = &ServiceCheckerFactory{}

// CreateChecker is a convenience function that uses the default factory
func CreateChecker(sp types.ServiceProvider, modelName string) ServiceChecker {
	return DefaultFactory.CreateServiceChecker(sp, modelName)
}
