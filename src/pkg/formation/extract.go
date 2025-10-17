package formation

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractBundle extracts a gzipped tarball to a destination directory
// Returns the path to the extracted formation directory
func ExtractBundle(gzipPath string, destDir string) (string, error) {
	// Open gzip file
	gzipFile, err := os.Open(gzipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open gzip file: %w", err)
	}
	defer gzipFile.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(gzipFile)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Track the root directory name
	var rootDir string

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar header: %w", err)
		}

		// Get the first directory component as root
		if rootDir == "" {
			parts := strings.Split(header.Name, "/")
			if len(parts) > 0 && parts[0] != "" {
				rootDir = parts[0]
			}
		}

		// Build target path
		target := filepath.Join(destDir, header.Name)

		// Ensure target is within destDir (security check)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create file
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}

			// Copy content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()

			// Set permissions
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return "", fmt.Errorf("failed to set permissions: %w", err)
			}
		}
	}

	if rootDir == "" {
		return "", fmt.Errorf("empty archive or no root directory found")
	}

	return filepath.Join(destDir, rootDir), nil
}
