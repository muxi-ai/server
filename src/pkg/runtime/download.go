package runtime

import (
	"fmt"
	"io"
	"net"
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

// fetchLatestVersion resolves the "latest" alias to an actual version.
//
// The package mirror publishes a plain-text "latest.txt" file at the
// base URL whenever a new release is pushed
// (e.g. https://pkg.muxi.org/runtime/latest.txt contains "0.20260424.0").
// A simple GET + string.TrimSpace is all that's needed — no redirect
// parsing, no GitHub API, no rate-limit concerns.
func (d *Downloader) fetchLatestVersion() (string, error) {
	latestURL := strings.TrimSuffix(d.sifBaseURL, "/") + "/latest.txt"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(latestURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version from %s: %w", latestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest.txt returned status %d (url: %s)", resp.StatusCode, latestURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", fmt.Errorf("failed to read latest.txt: %w", err)
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("latest.txt is empty (url: %s)", latestURL)
	}

	d.logger.Info().
		Str("version", version).
		Msg("Resolved 'latest' to actual version")

	return version, nil
}

// EnsureSIF checks if the SIF file exists, downloads if missing.
// Returns the path to the SIF file, the resolved version, and whether it was
// downloaded (vs already existed).
//
// Equivalent to EnsureSIFForVariant(version, DefaultVariant). Kept as a
// variant-less convenience for callers that pre-date the variant system;
// new call sites should use EnsureSIFForVariant to carry the operator's
// variant choice through the deploy pipeline.
func (d *Downloader) EnsureSIF(version string) (string, string, bool, error) {
	return d.EnsureSIFForVariant(version, DefaultVariant)
}

// EnsureSIFForVariant checks if the SIF file for a (version, variant) pair
// exists on disk, downloads from the release mirror if missing. Returns the
// on-disk path, the resolved version (meaningful when the caller passed
// "latest"), and whether a download actually happened.
//
// Variant threads through filename construction (via sifFilename) so the
// same GitHub release can host lean and pytorch SIFs side by side without
// any change to URL-building logic — the variant lives in the filename
// segment, and the release directory stays shared.
func (d *Downloader) EnsureSIFForVariant(version, variant string) (string, string, bool, error) {
	// Resolve "latest" to actual version. This is variant-independent —
	// a release publishes all variants together, so the version redirect
	// on the mirror points at the same release for all variants.
	if version == "latest" {
		resolved, err := d.fetchLatestVersion()
		if err != nil {
			return "", "", false, fmt.Errorf("failed to resolve 'latest' version: %w", err)
		}
		version = resolved
	}

	filename := sifFilename(version, variant)
	sifPath := filepath.Join(d.runtimesDir, filename)

	// Check if SIF already exists
	if _, err := os.Stat(sifPath); err == nil {
		d.logger.Debug().
			Str("path", sifPath).
			Str("variant", variant).
			Msg("SIF file already exists")
		return sifPath, version, false, nil
	}

	// Ensure runtimes directory exists
	if err := os.MkdirAll(d.runtimesDir, 0755); err != nil {
		return "", "", false, fmt.Errorf("failed to create runtimes directory: %w", err)
	}

	// Build download URL: {baseURL}/{version}/{filename}
	// e.g. https://pkg.muxi.org/runtime/0.20260424.0/muxi-runtime-0.20260424.0-linux-amd64.sif
	if !strings.HasPrefix(d.sifBaseURL, "http://") && !strings.HasPrefix(d.sifBaseURL, "https://") {
		return "", "", false, fmt.Errorf("invalid SIF base URL: %s", d.sifBaseURL)
	}
	url := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(d.sifBaseURL, "/"), version, filename)

	d.logger.Info().
		Str("url", url).
		Str("destination", sifPath).
		Str("variant", variant).
		Msg("Downloading SIF file (this may take a few minutes)...")

	// Download the file with progress logging
	if err := d.downloadFileWithProgress(url, sifPath); err != nil {
		return "", "", false, fmt.Errorf("failed to download SIF: %w", err)
	}

	d.logger.Info().
		Str("path", sifPath).
		Str("variant", variant).
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

// downloadFileWithProgress downloads a file from URL to destination with progress logging.
//
// Uses a client with an explicit 30s connection timeout (http.DefaultClient
// has no timeout at all; an unreachable mirror would hang the caller
// indefinitely). Transport-level idle and response-header timeouts are set
// separately so a stalled CDN mid-stream eventually fails instead of
// blocking a spawn-and-wait handler forever — previously this hung the api
// test suite for minutes when test config pointed at an unreachable URL.
// The overall Timeout is intentionally omitted so a genuinely large SIF on
// a slow connection can still complete.
func (d *Downloader) downloadFileWithProgress(url, destination string) error {
	client := &http.Client{
		Transport: &http.Transport{
			// DialContext covers DNS resolution + TCP connect —
			// ResponseHeaderTimeout below only starts counting
			// after the request is on the wire, so a broken DNS
			// resolver or an unreachable host would otherwise still
			// hang indefinitely before any byte is sent.
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		},
	}
	resp, err := client.Get(url)
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

// EnsureRuntime ensures both SIF (for the requested variant) and the
// runtime-runner image are available. An empty variant defaults to
// DefaultVariant via EnsureSIFForVariant's own handling, matching the
// behavior of the per-variant API handlers.
//
// This function isn't called from the live deploy pipeline (which uses
// EnsureSIFForVariant directly so it can capture the resolved version
// and downloaded-flag), but keeping it variant-aware prevents a future
// caller from silently getting the lean SIF when the operator asked
// for gpu or cuda.
func (d *Downloader) EnsureRuntime(version, variant string) (string, error) {
	sifPath, _, _, err := d.EnsureSIFForVariant(version, variant)
	if err != nil {
		return "", err
	}

	if _, err := d.EnsureRuntimeRunner(); err != nil {
		return "", err
	}

	return sifPath, nil
}
