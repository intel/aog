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
	"bytes"
	"net/http"

	"github.com/intel/aog/internal/logger"
)

// RemoteServiceChecker checks remote service availability
type RemoteServiceChecker struct {
	BaseServiceChecker
	RequestBuilder RequestBuilder
}

// CheckServer implements ServiceChecker for remote services
func (r *RemoteServiceChecker) CheckServer() bool {
	if r.RequestBuilder == nil {
		return true // For services that don't need actual checking
	}

	jsonData, err := r.RequestBuilder.BuildRequest(r.ModelName)
	if err != nil {
		logger.LogicLogger.Error("[ServiceChecker] Failed to build request", "error", err)
		return false
	}

	req, err := http.NewRequest(r.ServiceProvider.Method, r.ServiceProvider.URL, bytes.NewReader(jsonData))
	if err != nil {
		logger.LogicLogger.Error("[ServiceChecker] Failed to create request", "error", err)
		return false
	}

	return ExecuteServerRequest(req, r.ServiceProvider, string(jsonData))
}
