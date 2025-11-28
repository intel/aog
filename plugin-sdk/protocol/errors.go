//*****************************************************************************
// Copyright 2024-2025 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//*****************************************************************************

package protocol

import (
	"fmt"
	"strings"
)

// Plugin error code constants
const (
	// Success codes
	PluginCodeSuccess = 200
	PluginCodeOK      = 0

	// Client errors 4xx
	PluginCodeBadRequest     = 400 // Request parameter error
	PluginCodeUnauthorized   = 401 // Authentication failed
	PluginCodeNotFound       = 404 // Resource not found (e.g., model not found)
	PluginCodeTimeout        = 408 // Request timeout
	PluginCodePrecondFailed  = 412 // Precondition failed (e.g., engine not installed)
	PluginCodeNotImplemented = 501 // Method not implemented

	// Server errors 5xx
	PluginCodeInternalError  = 500 // Internal error
	PluginCodeServiceUnavail = 503 // Service unavailable (e.g., engine not started)
	PluginCodeBadGateway     = 502 // Bad gateway (upstream service error)
)

// PluginError represents a plugin gRPC error
type PluginError struct {
	Code    int32
	Message string
	Plugin  string
}

func (e *PluginError) Error() string {
	if e.Plugin != "" {
		return fmt.Sprintf("[Plugin:%s] (code=%d) %s", e.Plugin, e.Code, e.Message)
	}
	return fmt.Sprintf("Plugin error (code=%d): %s", e.Code, e.Message)
}

// NewPluginError creates a plugin error
func NewPluginError(code int32, message string) *PluginError {
	return &PluginError{
		Code:    code,
		Message: message,
	}
}

// MapErrorToPluginCode maps a Go error to a plugin error code
func MapErrorToPluginCode(err error) int32 {
	if err == nil {
		return PluginCodeSuccess
	}

	errMsg := strings.ToLower(err.Error())

	switch {
	// Authentication related errors
	case strings.Contains(errMsg, "auth"):
		return PluginCodeUnauthorized
	case strings.Contains(errMsg, "unauthorized"):
		return PluginCodeUnauthorized
	case strings.Contains(errMsg, "authentication"):
		return PluginCodeUnauthorized

	// Resource not found errors
	case strings.Contains(errMsg, "not found"):
		return PluginCodeNotFound
	case strings.Contains(errMsg, "does not exist"):
		return PluginCodeNotFound
	case strings.Contains(errMsg, "model not found"):
		return PluginCodeNotFound

	// Precondition errors
	case strings.Contains(errMsg, "not installed"):
		return PluginCodePrecondFailed
	case strings.Contains(errMsg, "not loaded"):
		return PluginCodePrecondFailed
	case strings.Contains(errMsg, "engine not"):
		return PluginCodePrecondFailed

	// Service unavailable errors
	case strings.Contains(errMsg, "not running"):
		return PluginCodeServiceUnavail
	case strings.Contains(errMsg, "unavailable"):
		return PluginCodeServiceUnavail
	case strings.Contains(errMsg, "not healthy"):
		return PluginCodeServiceUnavail

	// Timeout errors
	case strings.Contains(errMsg, "timeout"):
		return PluginCodeTimeout
	case strings.Contains(errMsg, "deadline"):
		return PluginCodeTimeout

	// Not implemented errors
	case strings.Contains(errMsg, "not implemented"):
		return PluginCodeNotImplemented
	case strings.Contains(errMsg, "not supported"):
		return PluginCodeNotImplemented

	// Parameter errors
	case strings.Contains(errMsg, "invalid"):
		return PluginCodeBadRequest
	case strings.Contains(errMsg, "bad request"):
		return PluginCodeBadRequest

	// Default to internal error
	default:
		return PluginCodeInternalError
	}
}

// IsSuccess checks if the error code indicates success
func IsSuccess(code int32) bool {
	return code == PluginCodeSuccess || code == PluginCodeOK
}

// CreateSuccessResponse is a helper function to create success response
func CreateSuccessResponse() (int32, string) {
	return PluginCodeSuccess, "success"
}

// CreateErrorResponse is a helper function to create error response
func CreateErrorResponse(err error) (int32, string) {
	if err == nil {
		return CreateSuccessResponse()
	}

	code := MapErrorToPluginCode(err)
	return code, err.Error()
}

// WrapPluginError wraps an error with plugin context information
func WrapPluginError(err error, pluginName, operation string) *PluginError {
	if err == nil {
		return nil
	}

	code := MapErrorToPluginCode(err)
	message := fmt.Sprintf("[%s] %s", operation, err.Error())

	return &PluginError{
		Code:    code,
		Message: message,
		Plugin:  pluginName,
	}
}

// IsPluginError checks if the error is of PluginError type
func IsPluginError(err error) bool {
	_, ok := err.(*PluginError)
	return ok
}

// AsPluginError attempts to convert the error to PluginError
func AsPluginError(err error) (*PluginError, bool) {
	if pluginErr, ok := err.(*PluginError); ok {
		return pluginErr, true
	}
	return nil, false
}
