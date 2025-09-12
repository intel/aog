package utils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

func GetDownloadDir() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		downloadsPath := os.Getenv("USERPROFILE")
		if downloadsPath == "" {
			return "", fmt.Errorf("unable to get user profile directory on Windows")
		}
		return filepath.Join(downloadsPath, "Downloads"), nil
	case "darwin":
		return filepath.Join(currentUser.HomeDir, "Downloads"), nil
	case "linux":
		xdgDownload := os.Getenv("XDG_DOWNLOAD_DIR")
		if xdgDownload != "" {
			return xdgDownload, nil
		}
		downloadsDir := filepath.Join(currentUser.HomeDir, "Downloads")
		if err := os.MkdirAll(downloadsDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create Downloads directory: %w", err)
		}
		return downloadsDir, nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func GetAOGDataDir() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "linux":
		dir = "/var/lib/aog"
	default:
		userDir, err := GetUserDataDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(userDir, "AOG")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	return dir, nil
}

func DownloadImageUrlToPath(url string) (string, error) {
	downLoadPath, err := GetDownloadDir()
	if err != nil {
		return "", fmt.Errorf("get download dir failed: %w", err)
	}
	savePath, err := DownloadFile(url, downLoadPath, false)
	if err != nil {
		return "", fmt.Errorf("download image file failed: %w", err)
	}
	return savePath, nil
}

func GetUserDataDir() (string, error) {
	var dir string
	switch sys := runtime.GOOS; sys {
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support")
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"))
	case "linux":
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome != "" {
			dir = xdgDataHome
		} else {
			dir = filepath.Join(os.Getenv("HOME"), ".local", "share")
		}
	default:
		return "", fmt.Errorf("unsupported operating system")
	}

	return dir, nil
}

// GetAbsolutePath Convert relative path to absolute path from the passed in base directory
// No change if the passed in path is already an absolute path
func GetAbsolutePath(p string, base string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}
