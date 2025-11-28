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

package bcode

import "net/http"

var (
	// Plugin Framework Success
	PluginCode = NewBcode(http.StatusOK, 100000, "Plugin operation success")

	// Plugin Framework Errors (100001-100099)
	ErrPluginNotFound    = NewBcode(http.StatusNotFound, 100001, "Plugin not found")
	ErrPluginLoadFailed  = NewBcode(http.StatusInternalServerError, 100002, "Plugin load failed")
	ErrPluginNotRunning  = NewBcode(http.StatusServiceUnavailable, 100003, "Plugin not running")
	ErrPluginInvalid     = NewBcode(http.StatusBadRequest, 100004, "Plugin configuration invalid")
	ErrPluginTimeout     = NewBcode(http.StatusRequestTimeout, 100005, "Plugin request timeout")
	ErrPluginUnavailable = NewBcode(http.StatusServiceUnavailable, 100006, "Plugin service unavailable")

	// Authentication Errors (100100-100199) - Remote Plugins
	ErrPluginAuthRequired = NewBcode(http.StatusUnauthorized, 100101, "Plugin authentication required")
	ErrPluginAuthFailed   = NewBcode(http.StatusUnauthorized, 100102, "Plugin authentication failed")
	ErrPluginAuthExpired  = NewBcode(http.StatusUnauthorized, 100103, "Plugin authentication expired")
	ErrPluginAuthInvalid  = NewBcode(http.StatusUnauthorized, 100104, "Plugin authentication invalid")

	// Engine Errors (100200-100299) - Local Plugins
	ErrPluginEngineNotInstalled = NewBcode(http.StatusPreconditionFailed, 100201, "Engine not installed")
	ErrPluginEngineStartFailed  = NewBcode(http.StatusInternalServerError, 100202, "Engine start failed")
	ErrPluginEngineStopFailed   = NewBcode(http.StatusInternalServerError, 100203, "Engine stop failed")
	ErrPluginEngineNotHealthy   = NewBcode(http.StatusServiceUnavailable, 100204, "Engine not healthy")
	ErrPluginEngineNotRunning   = NewBcode(http.StatusServiceUnavailable, 100205, "Engine not running")
	ErrPluginEngineConfigError  = NewBcode(http.StatusInternalServerError, 100206, "Engine configuration error")

	// Model Errors (100300-100399) - Local Plugins
	ErrPluginModelNotFound     = NewBcode(http.StatusNotFound, 100301, "Model not found")
	ErrPluginModelNotLoaded    = NewBcode(http.StatusPreconditionFailed, 100302, "Model not loaded")
	ErrPluginModelPullFailed   = NewBcode(http.StatusBadGateway, 100303, "Model pull failed")
	ErrPluginModelDeleteFailed = NewBcode(http.StatusInternalServerError, 100304, "Model delete failed")
	ErrPluginModelLoadFailed   = NewBcode(http.StatusInternalServerError, 100305, "Model load failed")
	ErrPluginModelUnloadFailed = NewBcode(http.StatusInternalServerError, 100306, "Model unload failed")

	// Service Invocation Errors (100400-100499)
	ErrPluginServiceError   = NewBcode(http.StatusBadGateway, 100401, "Plugin service error")
	ErrPluginMethodNotFound = NewBcode(http.StatusNotImplemented, 100402, "Plugin method not implemented")
	ErrPluginBadRequest     = NewBcode(http.StatusBadRequest, 100403, "Plugin bad request")
	ErrPluginInternalError  = NewBcode(http.StatusInternalServerError, 100404, "Plugin internal error")
)

// Plugin error code constants
const (
	PluginCodeSuccess = 200
	PluginCodeOK      = 0

	PluginCodeBadRequest     = 400
	PluginCodeUnauthorized   = 401
	PluginCodeNotFound       = 404
	PluginCodeTimeout        = 408
	PluginCodePrecondFailed  = 412
	PluginCodeNotImplemented = 501

	PluginCodeInternalError  = 500
	PluginCodeServiceUnavail = 503
	PluginCodeBadGateway     = 502
)

// PluginCodeToBcode maps plugin error codes to Bcode.
var PluginCodeToBcode = map[int32]*Bcode{
	PluginCodeUnauthorized: ErrPluginAuthFailed,

	PluginCodeNotFound:      ErrPluginModelNotFound,
	PluginCodePrecondFailed: ErrPluginEngineNotInstalled,

	PluginCodeBadRequest:     ErrPluginBadRequest,
	PluginCodeTimeout:        ErrPluginTimeout,
	PluginCodeNotImplemented: ErrPluginMethodNotFound,
	PluginCodeInternalError:  ErrPluginInternalError,
	PluginCodeServiceUnavail: ErrPluginUnavailable,
	PluginCodeBadGateway:     ErrPluginServiceError,
}

// ConvertPluginCodeToBcode converts plugin error codes to Bcode.
func ConvertPluginCodeToBcode(code int32, message string) *Bcode {
	if bcodeErr, exists := PluginCodeToBcode[code]; exists {
		if message != "" {
			return bcodeErr.SetMessage(message)
		}
		return bcodeErr
	}

	return ErrPluginInternalError.SetMessage(message)
}
