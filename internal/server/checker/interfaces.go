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

import "github.com/intel/aog/internal/types"

// ServiceChecker defines the interface for checking service availability
type ServiceChecker interface {
	CheckServer() bool
}

// BaseServiceChecker contains common fields for all service checkers
type BaseServiceChecker struct {
	ServiceProvider types.ServiceProvider
	ModelName       string
}

// RequestBuilder interface for building different types of requests
type RequestBuilder interface {
	BuildRequest(modelName string) ([]byte, error)
}

//// ServiceType represents different service types
//type ServiceType string
//
//const (
//	ServiceTypeModels         ServiceType = "models"
//	ServiceTypeChat           ServiceType = "chat"
//	ServiceTypeGenerate       ServiceType = "generate"
//	ServiceTypeEmbed          ServiceType = "embed"
//	ServiceTypeTextToImage    ServiceType = "text_to_image"
//	ServiceTypeTextToSpeech   ServiceType = "text_to_speech"
//	ServiceTypeSpeechToText   ServiceType = "speech_to_text"
//	ServiceTypeSpeechToTextWS ServiceType = "speech_to_text_ws"
//	ServiceTypeTextToVideo    ServiceType = "text_to_video"
//	ServiceTypeImageToVideo   ServiceType = "image_to_video"
//	ServiceTypeImageToImage   ServiceType = "image_to_image"
//)
