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
	ProcessCode = NewBcode(http.StatusOK, 50000, "Process interface call success")

	// Engine process errors (50001-50019)
	ErrEngineNotFound       = NewBcode(http.StatusNotFound, 50001, "Engine executable not found")
	ErrEngineStartFailed    = NewBcode(http.StatusInternalServerError, 50002, "Engine failed to start")
	ErrEngineStopFailed     = NewBcode(http.StatusInternalServerError, 50003, "Engine failed to stop")
	ErrEngineNotRunning     = NewBcode(http.StatusConflict, 50004, "Engine is not running")
	ErrEngineAlreadyRunning = NewBcode(http.StatusConflict, 50005, "Engine is already running")

	// Process management errors (50020-50039)
	ErrProcessNotFound    = NewBcode(http.StatusNotFound, 50020, "Process not found")
	ErrProcessStartFailed = NewBcode(http.StatusInternalServerError, 50021, "Process failed to start")
	ErrProcessStopFailed  = NewBcode(http.StatusInternalServerError, 50022, "Process failed to stop")
	ErrProcessTimeout     = NewBcode(http.StatusRequestTimeout, 50023, "Process operation timed out")
	ErrProcessKillFailed  = NewBcode(http.StatusInternalServerError, 50024, "Process failed to kill")

	// Configuration errors (50040-50059)
	ErrInvalidProcessConfig = NewBcode(http.StatusBadRequest, 50040, "Invalid process configuration")
	ErrMissingExecutable    = NewBcode(http.StatusBadRequest, 50041, "Missing executable path")
	ErrUnsupportedPlatform  = NewBcode(http.StatusBadRequest, 50042, "Unsupported platform")

	// Service manager errors (50060-50079)
	ErrServiceNotReady = NewBcode(http.StatusServiceUnavailable, 50060, "Service manager not ready")
	ErrNoEngineStarter = NewBcode(http.StatusInternalServerError, 50061, "No engine starter available")
	ErrNoEngineStopper = NewBcode(http.StatusInternalServerError, 50062, "No engine stopper available")

	ErrEngineNotAvailable = NewBcode(http.StatusServiceUnavailable, 50063, "Engine is not available")
)
