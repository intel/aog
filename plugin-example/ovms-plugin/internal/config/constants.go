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

package config

// OVMS Server Configuration
const (
	// Default ports
	OpenvinoGRPCPort = "9000"
	OpenvinoHTTPPort = "16666"
	OpenvinoGRPCHost = "127.0.0.1:" + OpenvinoGRPCPort
	OpenvinoHTTPHost = "http://127.0.0.1:" + OpenvinoHTTPPort

	// ModelScope configuration
	ModelScopeSCHEME     = "https"
	ModelScopeEndpointCN = "www.modelscope.cn"
	ModelScopeRevision   = "master"
	BufferSize           = 64 * 1024

	// Download URLs
	BaseDownloadURL = "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog"
	OVMSVersion     = "v0.7"
)

// Service Handler Configuration
const (
	// DefaultChannelBufferSize is the default buffer size for response channels
	DefaultChannelBufferSize = 10

	// MaxRetries is the maximum number of retry attempts for HTTP requests
	MaxRetries = 3
)

// GetOVMSDownloadURL returns the download URL for OVMS based on OS and distribution
func GetOVMSDownloadURL(goos, distro, version, aogVersion string) string {
	switch goos {
	case "windows":
		return BaseDownloadURL + "/windows/" + OVMSVersion + "/ovms_windows_python_on.zip"
	case "linux":
		switch distro {
		case "ubuntu":
			if version == "22.04" {
				return BaseDownloadURL + "/linux/" + OVMSVersion + "/ovms_ubuntu22_python_on.tar.gz"
			} else if version == "24.04" {
				return BaseDownloadURL + "/linux/" + OVMSVersion + "/ovms_ubuntu24_python_on.tar.gz"
			}
		case "deepin":
			// Deepin 使用 Ubuntu 22.04 版本
			return BaseDownloadURL + "/linux/" + OVMSVersion + "/ovms_ubuntu22_python_on.tar.gz"
		}
	}
	return ""
}

// GetScriptsDownloadURL returns the download URL for scripts
// Note: scripts.zip is the same for both Windows and Linux
func GetScriptsDownloadURL(goos, version string) string {
	// Fixed URL for scripts.zip (same for all platforms)
	return "https://smartvision-aipc-open.oss-cn-hangzhou.aliyuncs.com/aog/windows/v0.7/scripts.zip"
}
