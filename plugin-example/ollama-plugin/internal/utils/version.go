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
	"strconv"
	"strings"
)

// VersionCompare compares two version numbers
// Return value: 1 means v1>v2, -1 means v1<v2, 0 means v1==v2
// Complete port from built-in engine
func VersionCompare(v1, v2 string) int {
	s1 := strings.Split(v1, ".")
	s2 := strings.Split(v2, ".")

	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(s1) {
			n1, _ = strconv.Atoi(s1[i])
		}
		if i < len(s2) {
			n2, _ = strconv.Atoi(s2[i])
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

// Ollama minimum version requirement (ported from built-in engine)
const OllamaMinVersion = "0.7.1"
