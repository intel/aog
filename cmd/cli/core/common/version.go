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

package common

import (
	"fmt"

	"github.com/intel/aog/version"
	"github.com/spf13/cobra"
)

// NewVersionCommand print client version
func NewVersionCommand() *cobra.Command {
	ver := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  "Display version information for AOG (AIPC Open Gateway) and its API specification.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("AOG Version: %s\nSpec Version: %s\n", version.AOGVersion, version.SpecVersion)
		},
	}

	return ver
}
