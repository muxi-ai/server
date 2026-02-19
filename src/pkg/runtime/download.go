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
	"time"

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

// fetchLatestVersion fetches the latest runtime version from GitHub
// Uses redirect URL instead of API to avoid rate limiting
func (d *Downloader) fetchLatestVersion() (string, error) {
	// Extract org/repo from sifBaseURL
	// Expected: https://github.com/muxi-ai/runtime/releases/download
	if !strings.Contains(d.sifBaseURL, "github.com") {
		return "", fmt.Errorf("cannot fetch latest version from non-GitHub URL")
	}

	// Parse: https://github.com/{org}/{repo}/releases/download
	parts := strings.Split(d.sifBaseURL, "/")
	var org, repo string
	for i, p := range parts {
		if p == "github.com" && i+2 < len(parts) {
			org = parts[i+1]
			repo = parts[i+2]
			break
		}
	}
	if org == "" || repo == "" {
		return "", fmt.Errorf("could not parse GitHub org/repo from URL: %s", d.sifBaseURL)
	}

	// Use the "latest" redirect URL - no API call, no rate limit
	// HEAD request to: https://github.com/{org}/{repo}/releases/latest/download/test.sif
	// Returns redirect to: https://github.com/{org}/{repo}/releases/download/v{version}/test.sif
	latestURL := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/version.txt", org, repo)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects - we want to capture the redirect URL
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Head(latestURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("expected redirect, got status %d", resp.StatusCode)
	}

	// Parse version from redirect URL
	// Location: https://github.com/{org}/{repo}/releases/download/v0.20260217.0/version.txt
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location in response")
	}

	// Extract version from /download/v{version}/
	idx := strings.Index(location, "/download/v")
	if idx == -1 {
		return "", fmt.Errorf("could not parse version from redirect URL: %s", location)
	}
	rest := location[idx+len("/download/v"):]
	endIdx := strings.Index(rest, "/")
	if endIdx == -1 {
		return "", fmt.Errorf("could not parse version from redirect URL: %s", location)
	}
	version := rest[:endIdx]

	d.logger.Info().
		Str("version", version).
		Msg("Resolved 'latest' to actual version")

	return version, nil
}

// EnsureSIF checks if the SIF file exists, downloads if missing
// Returns the path to the SIF file, the resolved version, and whether it was downloaded (vs already existed)
func (d *Downloader) EnsureSIF(version string) (string, string, bool, error) {
	// Resolve "latest" to actual version
	if version == "latest" {
		resolved, err := d.fetchLatestVersion()
		if err != nil {
			return "", "", false, fmt.Errorf("failed to resolve 'latest' version: %w", err)
		}
		version = resolved
	}

	arch := getPlatform()
	filename := fmt.Sprintf("muxi-runtime-%s-%s.sif", version, arch)
	sifPath := filepath.Join(d.runtimesDir, filename)

	// Check if SIF already exists
	if _, err := os.Stat(sifPath); err == nil {
		d.logger.Debug().
			Str("path", sifPath).
			Msg("SIF file already exists")
		return sifPath, version, false, nil
	}

	// Ensure runtimes directory exists
	if err := os.MkdirAll(d.runtimesDir, 0755); err != nil {
		return "", "", false, fmt.Errorf("failed to create runtimes directory: %w", err)
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
		return "", "", false, fmt.Errorf("invalid SIF base URL: %s", d.sifBaseURL)
	}

	d.logger.Info().
		Str("url", url).
		Str("destination", sifPath).
		Msg("Downloading SIF file (this may take a few minutes)...")

	// Download the file with progress logging
	if err := d.downloadFileWithProgress(url, sifPath); err != nil {
		return "", "", false, fmt.Errorf("failed to download SIF: %w", err)
	}

	d.logger.Info().
		Str("path", sifPath).
		Msg("SIF file downloaded successfully")

	return sifPath, version, true, nil
}

// progressReader wraps an io.Reader and logs progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	lastLog    time.Time
	logger     *zerolog.Logger
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	// Log progress every 10 seconds
	if time.Since(pr.lastLog) > 10*time.Second {
		pr.lastLog = time.Now()
		if pr.total > 0 {
			pct := float64(pr.downloaded) / float64(pr.total) * 100
			pr.logger.Info().
				Str("progress", fmt.Sprintf("%.0f%%", pct)).
				Str("downloaded", fmt.Sprintf("%d MB", pr.downloaded/1024/1024)).
				Str("total", fmt.Sprintf("%d MB", pr.total/1024/1024)).
				Msg("Downloading SIF file...")
		} else {
			pr.logger.Info().
				Str("downloaded", fmt.Sprintf("%d MB", pr.downloaded/1024/1024)).
				Msg("Downloading SIF file...")
		}
	}
	return n, err
}

// downloadFileWithProgress downloads a file from URL to destination with progress logging
func (d *Downloader) downloadFileWithProgress(url, destination string) error {
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

	// Log total size
	size := resp.ContentLength
	if size > 0 {
		d.logger.Info().
			Str("size", fmt.Sprintf("%d MB", size/1024/1024)).
			Msg("SIF file size")
	}

	// Create temporary file
	tmpPath := destination + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Wrap reader with progress logging
	pr := &progressReader{
		reader:  resp.Body,
		total:   size,
		lastLog: time.Now(),
		logger:  d.logger,
	}

	written, err := io.Copy(out, pr)
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

	d.logger.Info().
		Str("size", fmt.Sprintf("%d MB", written/1024/1024)).
		Msg("SIF download complete")

	return nil
}

// EnsureRuntimeRunner checks if the Docker image exists, pulls if missing
// Returns whether it was pulled (vs already existed)
func (d *Downloader) EnsureRuntimeRunner() (bool, error) {
	// Only needed on macOS and Windows
	if goruntime.GOOS == "linux" {
		return false, nil
	}

	// Check if image exists locally
	cmd := exec.Command("docker", "image", "inspect", d.runtimeRunnerImage)
	if err := cmd.Run(); err == nil {
		d.logger.Debug().
			Str("image", d.runtimeRunnerImage).
			Msg("Runtime runner image already exists")
		return false, nil
	}

	d.logger.Info().
		Str("image", d.runtimeRunnerImage).
		Msg("Pulling runtime runner image...")

	// Pull the image
	cmd = exec.Command("docker", "pull", d.runtimeRunnerImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("failed to pull runtime runner image: %w", err)
	}

	d.logger.Info().
		Str("image", d.runtimeRunnerImage).
		Msg("Runtime runner image pulled successfully")

	return true, nil
}

// EnsureRuntime ensures both SIF and runtime-runner are available
func (d *Downloader) EnsureRuntime(version string) (string, error) {
	// Download SIF if needed
	sifPath, _, _, err := d.EnsureSIF(version)
	if err != nil {
		return "", err
	}

	// Pull runtime-runner if needed (macOS/Windows only)
	if _, err := d.EnsureRuntimeRunner(); err != nil {
		return "", err
	}

	return sifPath, nil
}
