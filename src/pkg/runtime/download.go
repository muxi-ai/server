package runtime

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/rs/zerolog"
)

// Downloader handles downloading runtime components
type Downloader struct {
	sifBaseURL         string
	runtimeRunnerImage string
	runtimesDir        string
	logger             *zerolog.Logger
}

// NewDownloader creates a new runtime downloader
func NewDownloader(sifBaseURL, runtimeRunnerImage, runtimesDir string, logger *zerolog.Logger) *Downloader {
	return &Downloader{
		sifBaseURL:         sifBaseURL,
		runtimeRunnerImage: runtimeRunnerImage,
		runtimesDir:        runtimesDir,
		logger:             logger,
	}
}

// EnsureSIF checks if the SIF file exists, downloads if missing
// Returns the path to the SIF file
func (d *Downloader) EnsureSIF(version string) (string, error) {
	arch := getPlatform()
	filename := fmt.Sprintf("muxi-runtime-%s-%s.sif", version, arch)
	sifPath := filepath.Join(d.runtimesDir, filename)

	// Check if SIF already exists
	if _, err := os.Stat(sifPath); err == nil {
		d.logger.Debug().
			Str("path", sifPath).
			Msg("SIF file already exists")
		return sifPath, nil
	}

	// Ensure runtimes directory exists
	if err := os.MkdirAll(d.runtimesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create runtimes directory: %w", err)
	}

	// Build download URL
	var url string
	if strings.HasPrefix(d.sifBaseURL, "http://") || strings.HasPrefix(d.sifBaseURL, "https://") {
		// Check if it's a GitHub releases URL or direct URL
		if strings.Contains(d.sifBaseURL, "github.com") && strings.Contains(d.sifBaseURL, "releases/download") {
			// GitHub releases format: baseURL/v{version}/{filename}
			url = fmt.Sprintf("%s/v%s/%s", d.sifBaseURL, version, filename)
		} else {
			// Direct URL format: baseURL/{filename}
			url = fmt.Sprintf("%s/%s", strings.TrimSuffix(d.sifBaseURL, "/"), filename)
		}
	} else {
		return "", fmt.Errorf("invalid SIF base URL: %s", d.sifBaseURL)
	}

	d.logger.Info().
		Str("url", url).
		Str("destination", sifPath).
		Msg("Downloading SIF file...")

	// Download the file
	if err := d.downloadFile(url, sifPath); err != nil {
		return "", fmt.Errorf("failed to download SIF: %w", err)
	}

	d.logger.Info().
		Str("path", sifPath).
		Msg("SIF file downloaded successfully")

	return sifPath, nil
}

// downloadFile downloads a file from URL to destination
func (d *Downloader) downloadFile(url, destination string) error {
	// Create HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Create temporary file
	tmpPath := destination + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy with progress logging
	size := resp.ContentLength
	written, err := io.Copy(out, resp.Body)
	out.Close()

	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}

	if size > 0 && written != size {
		os.Remove(tmpPath)
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", written, size)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Rename to final destination
	if err := os.Rename(tmpPath, destination); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move file: %w", err)
	}

	d.logger.Debug().
		Int64("bytes", written).
		Msg("Download complete")

	return nil
}

// EnsureRuntimeRunner checks if the Docker image exists, pulls if missing
func (d *Downloader) EnsureRuntimeRunner() error {
	// Only needed on macOS and Windows
	if goruntime.GOOS == "linux" {
		return nil
	}

	// Check if image exists locally
	cmd := exec.Command("docker", "image", "inspect", d.runtimeRunnerImage)
	if err := cmd.Run(); err == nil {
		d.logger.Debug().
			Str("image", d.runtimeRunnerImage).
			Msg("Runtime runner image already exists")
		return nil
	}

	d.logger.Info().
		Str("image", d.runtimeRunnerImage).
		Msg("Pulling runtime runner image...")

	// Pull the image
	cmd = exec.Command("docker", "pull", d.runtimeRunnerImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull runtime runner image: %w", err)
	}

	d.logger.Info().
		Str("image", d.runtimeRunnerImage).
		Msg("Runtime runner image pulled successfully")

	return nil
}

// EnsureRuntime ensures both SIF and runtime-runner are available
func (d *Downloader) EnsureRuntime(version string) (string, error) {
	// Download SIF if needed
	sifPath, err := d.EnsureSIF(version)
	if err != nil {
		return "", err
	}

	// Pull runtime-runner if needed (macOS/Windows only)
	if err := d.EnsureRuntimeRunner(); err != nil {
		return "", err
	}

	return sifPath, nil
}
