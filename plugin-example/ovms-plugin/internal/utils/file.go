package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnzipFile extracts a zip or tar.gz file to the destination directory
func UnzipFile(src, dest string) error {
	if strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		return extractTarGz(src, dest)
	}
	return extractZip(src, dest)
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check path security - prevent path traversal attacks
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open file in zip: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		target := filepath.Join(dest, header.Name)

		// Check path security - prevent path traversal attacks
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}
	return nil
}

// DownloadFile downloads a file from URL to destination
// Returns the path to the downloaded file
func DownloadFile(url, destDir string, overwrite bool) (string, error) {
	filename := filepath.Base(url)
	destPath := filepath.Join(destDir, filename)

	if !overwrite {
		if _, err := os.Stat(destPath); err == nil {
			return destPath, nil
		}
	}

	// TODO: implement actual HTTP download
	// For now, just create placeholder
	return destPath, fmt.Errorf("DownloadFile not implemented: %s", url)
}

// GetPluginDir returns the directory containing the plugin executable
func GetPluginDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Walk up to find plugin.yaml
	dir := filepath.Dir(exe)
	for i := 0; i < 5; i++ {
		manifestPath := filepath.Join(dir, "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Dir(exe), nil
}
