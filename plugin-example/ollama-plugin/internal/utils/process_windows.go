//go:build windows
// +build windows

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

package utils

import (
	"fmt"
)

// setupProcAttr sets Windows platform-specific process attributes
func (pm *ProcessManager) setupProcAttr() {
	// Windows system: no special settings for now
	// Can set CREATE_NEW_PROCESS_GROUP if needed
}

// stopProcess Windows platform process stop logic
func (pm *ProcessManager) stopProcess() error {
	// Windows directly force kill (no SIGTERM)
	if err := pm.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	pm.cmd.Wait()
	return nil
}
