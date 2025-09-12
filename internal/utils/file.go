package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func DownloadFile(downloadURL string, saveDir string, cover bool) (string, error) {
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %v", err)
	}

	fileName := filepath.Base(parsedURL.Path)
	if fileName == "" || fileName == "." || fileName == "/" {
		return "", fmt.Errorf("could not determine file name from URL: %s", downloadURL)
	}

	savePath := filepath.Join(saveDir, fileName)

	if _, err := os.Stat(savePath); err == nil {
		if !cover {
			fmt.Printf("%s already exists, skip download.\n", savePath)
			return savePath, nil
		}
		// if cover == true，remove and download
		if err := os.Remove(savePath); err != nil {
			return "", fmt.Errorf("failed to remove existing file %s: %v", savePath, err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check file %s: %v", savePath, err)
	}

	proxyURL, err := http.ProxyFromEnvironment(&http.Request{URL: parsedURL})
	if err != nil {
		return "", fmt.Errorf("failed to get proxy URL: %v", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: HTTP status %s", resp.Status)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return savePath, nil
}

// Unzip file, distinguish system win/linux/macos
func UnzipFile(zipFile, destDir string) error {
	// 检查目标目录是否存在，如果不存在则创建
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		err := os.MkdirAll(destDir, 0o750)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", destDir, err)
		}
	}
	filename := filepath.Base(zipFile)
	archiveType := getArchiveType(filename)

	switch runtime.GOOS {
	case "windows":
		err := extractArchiveWindows(zipFile, destDir, archiveType)
		if err != nil {
			return err
		}
	case "linux", "darwin":
		err := extractArchiveUnix(zipFile, destDir, archiveType)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return nil
}

func ReadImageFileToBase64(filePath string) (string, error) {
	imgData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read image file failed: %w", err)
	}
	return base64.StdEncoding.EncodeToString(imgData), nil
}

func ParseImageData(data []byte) ([][]byte, error) {
	// Create a reader
	r := bytes.NewReader(data)
	var images [][]byte
	fmt.Println(len(data))

	// First, read the number of images (4 bytes)
	var imageCount uint32
	err := binary.Read(r, binary.LittleEndian, &imageCount)
	if err != nil {
		return nil, fmt.Errorf("failed to read image count: %v", err)
	}

	// Read each image
	for i := 0; i < int(imageCount); i++ {
		// Read the image length (4 bytes)
		var imgLen uint32
		err := binary.Read(r, binary.LittleEndian, &imgLen)
		if err != nil {
			return nil, fmt.Errorf("failed to read image length for image %d: %v", i+1, err)
		}

		// Read image data
		imgData := make([]byte, imgLen)
		_, err = io.ReadFull(r, imgData)
		if err != nil {
			return nil, fmt.Errorf("failed to read image data for image %d: %v", i+1, err)
		}

		images = append(images, imgData)
	}

	return images, nil
}

// getArchiveType determines the archive type based on file extension
func getArchiveType(filename string) string {
	filename = strings.ToLower(filename)
	if strings.HasSuffix(filename, ".tar.gz") {
		return "tar.gz"
	} else if strings.HasSuffix(filename, ".tgz") {
		return "tgz"
	} else if strings.HasSuffix(filename, ".zip") {
		return "zip"
	}
	return "unknown"
}

// extractArchiveWindows handles archive extraction on Windows
func extractArchiveWindows(archiveFile, destDir, archiveType string) error {
	var cmd *exec.Cmd

	switch archiveType {
	case "zip":
		cmd = exec.Command("powershell", "-Command",
			fmt.Sprintf("Expand-Archive -Path \"%s\" -DestinationPath \"%s\" -Force", archiveFile, destDir))
	case "tar.gz", "tgz":
		// Use tar command on Windows (available in Windows 10+)
		cmd = exec.Command("tar", "-xzf", archiveFile, "-C", destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", archiveType)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract file %s: %v", archiveFile, err)
	}
	return nil
}

// extractArchiveUnix handles archive extraction on Unix-like systems (Linux/macOS)
func extractArchiveUnix(archiveFile, destDir, archiveType string) error {
	var cmd *exec.Cmd

	switch archiveType {
	case "zip":
		cmd = exec.Command("unzip", "-o", archiveFile, "-d", destDir)
	case "tar.gz", "tgz":
		cmd = exec.Command("tar", "-xzf", archiveFile, "-C", destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", archiveType)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract file %s: %v", archiveFile, err)
	}
	return nil
}
