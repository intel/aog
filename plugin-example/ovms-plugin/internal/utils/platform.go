package utils

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// DetectLinuxDistribution detects Linux distribution and version
func DetectLinuxDistribution() (string, string, error) {
	if runtime.GOOS != "linux" {
		return "", "", fmt.Errorf("not a Linux system")
	}

	// Try /etc/os-release first
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(data), "\n")
	var distro, version string

	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			distro = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	if distro == "" {
		return "", "", fmt.Errorf("failed to detect distribution")
	}

	return distro, version, nil
}

// GetAOGDataDir returns the AOG data directory based on platform
func GetAOGDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + "/Library/Application Support/AOG", nil
	case "linux":
		return "/var/lib/aog", nil
	case "windows":
		// Use APPDATA (Roaming) to match AOG core behavior
		appData := os.Getenv("APPDATA")
		if appData == "" {
			// Fallback: use user home directory if APPDATA not set
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			return home + "\\AppData\\Roaming\\AOG", nil
		}
		return appData + "\\AOG", nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// GetDownloadDir returns the download directory
func GetDownloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/Downloads", nil
}
