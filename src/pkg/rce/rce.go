package rce

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/dockerutil"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/rs/zerolog"
)

const (
	DefaultPort       = 7891
	DockerImage       = "ghcr.io/muxi-ai/skills-rce:latest"
	GitHubReleasesURL = "https://github.com/muxi-ai/skills-rce/releases"
	HealthEndpoint    = "/health"
)

// Manager handles the lifecycle of the Skills RCE service
type Manager struct {
	port      int
	authToken string
	sifPath   string
	dataDir   string
	logsDir   string
	pidsDir   string
	logger    *zerolog.Logger
	process   *process.Process
	procMgr   *process.Manager
}

// NewManager creates a new RCE manager
func NewManager(cfg *config.Config, procMgr *process.Manager, logger *zerolog.Logger) *Manager {
	port := cfg.RCE.Port
	if port == 0 {
		port = DefaultPort
	}

	return &Manager{
		port:      port,
		authToken: cfg.RCE.AuthToken,
		dataDir:   cfg.RCE.DataDir,
		logsDir:   cfg.Formations.LogsDir,
		pidsDir:   cfg.Formations.PIDsDir,
		logger:    logger,
		procMgr:   procMgr,
	}
}

// Start launches the RCE service as a managed process
func (m *Manager) Start() error {
	if m.authToken == "" {
		return fmt.Errorf("RCE auth token not configured")
	}

	env := map[string]string{
		"RCE_PORT":       strconv.Itoa(m.port),
		"RCE_AUTH_TOKEN": m.authToken,
		"RCE_CACHE_DIR":  filepath.Join(m.dataDir, "rce", "cache"),
	}

	// Ensure cache dir exists
	os.MkdirAll(filepath.Join(m.dataDir, "rce", "cache"), 0755)

	var spawnCfg process.SpawnConfig

	if goruntime.GOOS == "linux" {
		// Native Linux: run SIF via Apptainer
		sifPath, err := m.findSIF()
		if err != nil {
			return fmt.Errorf("RCE SIF not found: %w (run 'muxi-server init' to download)", err)
		}
		m.sifPath = sifPath

		spawnCfg = process.SpawnConfig{
			ID:          "muxi-rce",
			Command:     "skills-rce",
			WorkDir:     filepath.Join(m.dataDir, "rce"),
			Env:         env,
			AutoRestart: true,
			RuntimeType: "singularity",
			SIFPath:     sifPath,
		}
	} else {
		// macOS/Windows: run Docker image
		spawnCfg = process.SpawnConfig{
			ID:          "muxi-rce",
			Command:     "skills-rce",
			Port:        m.port,
			WorkDir:     filepath.Join(m.dataDir, "rce"),
			Env:         env,
			AutoRestart: true,
			RuntimeType: "native",
		}
		// Docker run is handled separately since it's not a SIF
		return m.startDocker(env)
	}

	spawnCfg.LogsDir = m.logsDir
	spawnCfg.PIDsDir = m.pidsDir

	proc, err := m.procMgr.Start(spawnCfg)
	if err != nil {
		return fmt.Errorf("failed to start RCE: %w", err)
	}
	m.process = proc

	m.logger.Info().
		Int("port", m.port).
		Str("pid", fmt.Sprintf("%d", proc.PID)).
		Msg("Skills RCE started")

	return nil
}

// startDocker starts the RCE as a Docker container (macOS/Windows)
func (m *Manager) startDocker(env map[string]string) error {
	containerName := "muxi-rce"

	// Clean up any existing container
	exec.Command("docker", "rm", "-f", containerName).Run()

	cacheDir := filepath.Join(m.dataDir, "rce", "cache")
	os.MkdirAll(cacheDir, 0755)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:7891", m.port),
		"-v", fmt.Sprintf("%s:/cache/skills", cacheDir),
		"-e", fmt.Sprintf("RCE_AUTH_TOKEN=%s", env["RCE_AUTH_TOKEN"]),
	}

	args = append(args, DockerImage)

	cmd := exec.Command("docker", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start RCE Docker container: %w", err)
	}

	m.logger.Info().
		Int("port", m.port).
		Str("container", containerName).
		Msg("Skills RCE started (Docker)")

	return nil
}

// Stop stops the RCE service
func (m *Manager) Stop() {
	if goruntime.GOOS != "linux" {
		exec.Command("docker", "rm", "-f", "muxi-rce").Run()
	}
	if m.process != nil {
		m.procMgr.Stop("muxi-rce")
	}
}

// WaitForHealthy waits for the RCE to become healthy
func (m *Manager) WaitForHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d%s", m.port, HealthEndpoint)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				m.logger.Info().Int("port", m.port).Msg("Skills RCE is healthy")
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("RCE health check timed out after %v", timeout)
}

// GetURL returns the RCE URL for formations to use
func (m *Manager) GetURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.port)
}

// GetAuthToken returns the RCE auth token
func (m *Manager) GetAuthToken() string {
	return m.authToken
}

// Port returns the configured port
func (m *Manager) Port() int {
	return m.port
}

// InjectEnvVars adds RCE connection info to a formation's environment variables.
// Only injects if the formation doesn't already have RCE configured.
func (m *Manager) InjectEnvVars(env map[string]string) {
	if _, ok := env["MUXI_RCE_URL"]; ok {
		return // formation has its own RCE config
	}
	env["MUXI_RCE_URL"] = m.GetURL()
	env["MUXI_RCE_TOKEN"] = m.authToken
}

// findSIF looks for the RCE SIF in the data directory
func (m *Manager) findSIF() (string, error) {
	rceDir := filepath.Join(m.dataDir, "rce")
	matches, err := filepath.Glob(filepath.Join(rceDir, "skills-rce-*.sif"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no RCE SIF found in %s", rceDir)
	}
	// Return the most recent (lexicographic sort works with date-based versions)
	latest := matches[0]
	for _, m := range matches[1:] {
		if m > latest {
			latest = m
		}
	}
	return latest, nil
}

// EnsureSIF downloads the latest RCE SIF if not present or outdated.
// Always fetches the latest version.
func EnsureSIF(dataDir string) (string, error) {
	rceDir := filepath.Join(dataDir, "rce")
	os.MkdirAll(rceDir, 0755)

	// Resolve latest version from GitHub redirect
	latestVersion, err := fetchLatestVersion()
	if err != nil {
		return "", fmt.Errorf("failed to resolve latest RCE version: %w", err)
	}

	arch := getPlatform()
	filename := fmt.Sprintf("skills-rce-%s-%s.sif", latestVersion, arch)
	sifPath := filepath.Join(rceDir, filename)

	// Already have it?
	if _, err := os.Stat(sifPath); err == nil {
		return sifPath, nil
	}

	// Download
	url := fmt.Sprintf("%s/download/v%s/%s", GitHubReleasesURL, latestVersion, filename)

	if err := downloadFile(url, sifPath); err != nil {
		return "", fmt.Errorf("failed to download RCE SIF: %w", err)
	}

	return sifPath, nil
}

// EnsureDocker pulls the latest RCE Docker image (macOS/Windows) and
// renders a single collapsed progress line (with spinner) instead of
// Docker's native per-layer output. Same rendering as
// cmd/server/commands.go's pullRuntimeRunner — routed through the
// shared pkg/dockerutil.RenderPullProgress so any future tweak to the
// format lands in both pulls at once.
//
// DOCKER_CLI_HINTS=false suppresses Docker Desktop's "What's next:"
// promotional footer (docker scout quickview…) that's pure noise in a
// bootstrap flow.
//
// Stderr stays wired to the terminal so real Docker errors (auth,
// network, daemon down) remain visible at full fidelity.
//
// out is the progress destination — typically os.Stdout from cmdInit,
// but accepting an io.Writer keeps EnsureDocker usable from daemon /
// test / JSON-API contexts that shouldn't touch stdout. Pass io.Discard
// to silence progress entirely.
func EnsureDocker(out io.Writer) error {
	cmd := exec.Command("docker", "pull", DockerImage)
	cmd.Env = append(os.Environ(), "DOCKER_CLI_HINTS=false")
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start docker pull: %w", err)
	}

	// Render progress on a background goroutine so cmd.Wait doesn't
	// block on an un-drained pipe; signal completion via `done` so we
	// don't return before the final line is painted.
	done := make(chan struct{})
	go func() {
		defer close(done)
		dockerutil.RenderPullProgress(stdout, out)
	}()

	waitErr := cmd.Wait()
	<-done
	if waitErr != nil {
		return fmt.Errorf("failed to pull RCE image: %w", waitErr)
	}
	return nil
}

// GenerateAuthToken generates a random auth token for the RCE service
func GenerateAuthToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "rce_" + hex.EncodeToString(b)
}

// fetchLatestVersion resolves the latest release version from GitHub
func fetchLatestVersion() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Head(GitHubReleasesURL + "/latest/download/version.txt")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("expected redirect, got status %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	idx := strings.Index(location, "/download/v")
	if idx == -1 {
		return "", fmt.Errorf("could not parse version from redirect URL: %s", location)
	}
	rest := location[idx+len("/download/v"):]
	endIdx := strings.Index(rest, "/")
	if endIdx == -1 {
		return "", fmt.Errorf("could not parse version from redirect URL: %s", location)
	}
	return rest[:endIdx], nil
}

func getPlatform() string {
	arch := goruntime.GOARCH
	if arch == "amd64" {
		return "linux-amd64"
	}
	return "linux-arm64"
}

func downloadFile(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpPath := destination + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}

	os.Chmod(tmpPath, 0755)
	if err := os.Rename(tmpPath, destination); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}
