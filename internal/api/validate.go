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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/intel/aog/internal/api/dto"
	"github.com/intel/aog/internal/types"
	"github.com/intel/aog/internal/utils"
)

var validate = validator.New()

// Custom error message mapping
var validationErrorMessages = map[string]string{
	"required":                 "This field is required",
	"supported_service":        "Unsupported service type",
	"supported_service_source": "Unsupported service source type",
	"supported_flavor":         "Unsupported API flavor",
	"supported_auth_type":      "Unsupported authentication type",
	"supported_http_method":    "Unsupported HTTP method",
	"required_with_auth":       "Authentication key is required when using authentication",
	"json_format":              "Invalid JSON format",
	"url":                      "Invalid URL format",
	"min":                      "Length cannot be less than minimum value",
	"max":                      "Length cannot exceed maximum value",
	"must_be_remote":           "This service only supports remote mode",
	"required_for_remote_auth": "Authentication key is required for remote service when using authentication",
}

func init() {
	// Register custom validation rules
	validate.RegisterValidation("supported_service", validateSupportedService)
	validate.RegisterValidation("supported_service_source", validateSupportedServiceSource)
	validate.RegisterValidation("supported_flavor", validateSupportedFlavor)
	validate.RegisterValidation("supported_auth_type", validateSupportedAuthType)
	validate.RegisterValidation("supported_http_method", validateSupportedHTTPMethod)
	validate.RegisterValidation("required_with_auth", validateRequiredWithAuth)
	validate.RegisterValidation("json_format", validateJSONFormat)

	// Register struct-level validation
	validate.RegisterStructValidation(validateCreateAIGCServiceRequest, dto.CreateAIGCServiceRequest{})
}

// Validate supported service types
func validateSupportedService(fl validator.FieldLevel) bool {
	serviceName := fl.Field().String()
	return utils.Contains(types.SupportService, serviceName)
}

// Validate supported service sources
func validateSupportedServiceSource(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true // Allow empty, will use default value
	}
	serviceSource := fl.Field().String()
	return serviceSource == types.ServiceSourceLocal || serviceSource == types.ServiceSourceRemote
}

// Validate supported API flavors
func validateSupportedFlavor(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true // Allow empty, will use recommended configuration
	}
	flavor := fl.Field().String()
	return utils.Contains(types.SupportFlavor, flavor)
}

// Validate supported authentication types
func validateSupportedAuthType(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true // Allow empty, will use default value
	}
	authType := fl.Field().String()
	return utils.Contains(types.SupportAuthType, authType)
}

// Validate supported HTTP methods
func validateSupportedHTTPMethod(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true // Allow empty, will use default value POST
	}
	method := strings.ToUpper(fl.Field().String())
	return method == http.MethodPost || method == http.MethodGet ||
		method == http.MethodPut || method == http.MethodDelete
}

// Validate authentication information completeness
func validateRequiredWithAuth(fl validator.FieldLevel) bool {
	// Get the entire struct
	parent := fl.Parent()

	// Get AuthType field
	authTypeField := parent.FieldByName("AuthType")
	if !authTypeField.IsValid() {
		return true // Skip validation if AuthType field is not found
	}

	authType := authTypeField.String()
	authKey := fl.Field().String()

	// Validation fails if AuthType is not none and AuthKey is empty
	if authType != "" && authType != types.AuthTypeNone && authKey == "" {
		return false
	}

	return true
}

// Validate JSON format
func validateJSONFormat(fl validator.FieldLevel) bool {
	jsonStr := fl.Field().String()
	if jsonStr == "" {
		return true // Empty string is considered valid
	}

	var temp interface{}
	return json.Unmarshal([]byte(jsonStr), &temp) == nil
}

// Struct-level validation - CreateAIGCServiceRequest
func validateCreateAIGCServiceRequest(sl validator.StructLevel) {
	request := sl.Current().Interface().(dto.CreateAIGCServiceRequest)

	// Validate services that only support remote mode
	if utils.Contains(types.SupportOnlyRemoteService, request.ServiceName) {
		if request.ServiceSource != "" && request.ServiceSource != types.ServiceSourceRemote {
			sl.ReportError(request.ServiceSource, "ServiceSource", "ServiceSource", "must_be_remote", "")
		}
	}

	// Validate authentication information for remote services
	if request.ServiceSource == types.ServiceSourceRemote {
		if request.AuthType != "" && request.AuthType != types.AuthTypeNone && request.AuthKey == "" {
			sl.ReportError(request.AuthKey, "AuthKey", "AuthKey", "required_for_remote_auth", "")
		}
	}
}

// Format validation errors
func FormatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, e := range validationErrors {
			if msg, exists := validationErrorMessages[e.Tag()]; exists {
				messages = append(messages, fmt.Sprintf("%s: %s", e.Field(), msg))
			} else {
				messages = append(messages, fmt.Sprintf("%s: validation failed (%s)", e.Field(), e.Tag()))
			}
		}
		return fmt.Errorf("parameter validation failed: %s", strings.Join(messages, "; "))
	}
	return err
}

// RequestDefaultSetter interface defines request types that need default value setting
type RequestDefaultSetter interface {
	SetDefaults()
}

// ValidateAndSetDefaults uniformly handles default value setting and validation
func ValidateAndSetDefaults(request interface{}) error {
	// If the request implements RequestDefaultSetter interface, set default values
	if setter, ok := request.(RequestDefaultSetter); ok {
		setter.SetDefaults()
	}

	// Execute validation
	if err := validate.Struct(request); err != nil {
		return FormatValidationError(err)
	}
	return nil
}

// TestValidateStruct validation function for testing
func TestValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		return FormatValidationError(err)
	}
	return nil
}
