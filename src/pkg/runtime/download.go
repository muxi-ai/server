package runtime

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Downloader handles downloading runtime SIF files
type Downloader struct {
	runtimesDir string // Directory to store runtime SIF files
	registry    *Registry
}

// NewDownloader creates a new runtime downloader
func NewDownloader(runtimesDir string, registry *Registry) *Downloader {
	return &Downloader{
		runtimesDir: runtimesDir,
		registry:    registry,
	}
}

// Download fetches a runtime SIF file
// For now, this is a placeholder that expects SIF files to be present locally
// TODO: Add GitHub/CDN download support
func (d *Downloader) Download(version string) (string, error) {
	sifPath := d.GetSIFPath(version)

	// Check if already exists
	if _, err := os.Stat(sifPath); err == nil {
		return sifPath, nil
	}

	// For now, we don't actually download - we expect SIF to be manually placed
	// This is a placeholder for future GitHub/CDN integration
	return "", fmt.Errorf("runtime %s not found at %s - please download manually", version, sifPath)
}

// GetSIFPath returns the expected path for a runtime SIF file
func (d *Downloader) GetSIFPath(version string) string {
	filename := fmt.Sprintf("muxi-runtime-%s-%s.sif", version, getPlatform())
	return filepath.Join(d.runtimesDir, filename)
}

// Register registers a manually downloaded SIF file
func (d *Downloader) Register(sifPath string, version string) error {
	// Ensure runtimes directory exists
	if err := os.MkdirAll(d.runtimesDir, 0755); err != nil {
		return fmt.Errorf("failed to create runtimes directory: %w", err)
	}

	// Get file info
	stat, err := os.Stat(sifPath)
	if err != nil {
		return fmt.Errorf("failed to stat SIF file: %w", err)
	}

	// Compute hash
	hash, err := computeFileHash(sifPath)
	if err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	// Get destination path
	destPath := d.GetSIFPath(version)

	// Copy to runtimes directory if not already there
	if sifPath != destPath {
		if err := copyFile(sifPath, destPath); err != nil {
			return fmt.Errorf("failed to copy SIF: %w", err)
		}
	}

	// Register in registry
	info := &RuntimeInfo{
		Version:      version,
		Hash:         hash,
		Path:         destPath,
		Size:         stat.Size(),
		DownloadedAt: stat.ModTime(),
		Formations:   []string{},
	}

	if err := d.registry.Add(info); err != nil {
		return fmt.Errorf("failed to add to registry: %w", err)
	}

	if err := d.registry.Save(); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

// getPlatform returns the platform string for SIF files
// SIF files are always Linux containers, so we return "linux-{arch}"
// The architecture matches the host (arm64 or amd64) since:
// - On Linux: native Singularity runs the SIF directly
// - On macOS/Windows: runtime-runner (Docker) provides the Linux environment,
//   and we have matching architecture runtime-runner images (amd64 + arm64)
func getPlatform() string {
	return fmt.Sprintf("linux-%s", runtime.GOARCH)
}

// computeFileHash computes SHA256 hash of a file
func computeFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Sync to disk
	return dstFile.Sync()
}
