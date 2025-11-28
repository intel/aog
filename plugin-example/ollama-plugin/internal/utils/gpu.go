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
	"os/exec"
	"runtime"
	"strings"
)

// GPU type constants (ported from built-in engine)
const (
	GPUTypeNone     = "none"
	GPUTypeNvidia   = "nvidia"
	GPUTypeAmd      = "amd"
	GPUTypeIntelArc = "intel_arc"
)

// DetectGPUType detects GPU type (complete port of built-in engine logic)
func DetectGPUType() string {
	switch runtime.GOOS {
	case "windows":
		return detectGPUWindows()
	case "linux":
		return detectGPULinux()
	case "darwin":
		return GPUTypeNone // macOS unified handling
	default:
		return GPUTypeNone
	}
}

// detectGPUWindows Windows platform GPU detection
func detectGPUWindows() string {
	// Use wmic command to detect graphics card
	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
	output, err := cmd.Output()
	if err != nil {
		return GPUTypeNone
	}

	outputStr := strings.ToLower(string(output))
	hasNvidia := strings.Contains(outputStr, "nvidia")
	hasAMD := strings.Contains(outputStr, "amd") || strings.Contains(outputStr, "radeon")
	hasIntelArc := strings.Contains(outputStr, "intel") && strings.Contains(outputStr, "arc")

	if hasIntelArc {
		return GPUTypeIntelArc
	}
	if hasNvidia && hasAMD {
		return GPUTypeNvidia + "," + GPUTypeAmd
	}
	if hasNvidia {
		return GPUTypeNvidia
	}
	if hasAMD {
		return GPUTypeAmd
	}
	return GPUTypeNone
}

// detectGPULinux Linux platform GPU detection
func detectGPULinux() string {
	// Use lspci command to detect graphics card
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err != nil {
		return GPUTypeNone
	}

	outputStr := strings.ToLower(string(output))
	hasNvidia := strings.Contains(outputStr, "nvidia")
	hasAMD := strings.Contains(outputStr, "amd") || strings.Contains(outputStr, "radeon")
	hasIntelArc := strings.Contains(outputStr, "intel") && (strings.Contains(outputStr, "arc") || strings.Contains(outputStr, "dg2"))

	if hasIntelArc {
		return GPUTypeIntelArc
	}
	if hasNvidia && hasAMD {
		return GPUTypeNvidia + "," + GPUTypeAmd
	}
	if hasNvidia {
		return GPUTypeNvidia
	}
	if hasAMD {
		return GPUTypeAmd
	}
	return GPUTypeNone
}
