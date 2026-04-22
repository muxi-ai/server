package main

import (
	"bufio"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/hfcache"
	"github.com/muxi-ai/server/pkg/rce"
	"github.com/muxi-ai/server/pkg/registry"
	"gopkg.in/yaml.v3"
)

// Version is embedded from .version file at build time
//
//go:embed .version
var embeddedVersion string

// Version information
var (
	Version   = ""
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func init() {
	Version = strings.TrimSpace(embeddedVersion)
	if Version == "" {
		Version = "0.0.0-dev"
	}
}

// ANSI constants for clean output
const (
	checkMark  = "✓"
	crossMark  = "✗"
	arrowRight = "→"
	bullet     = "•"
	boxH       = "─"
	resetColor = "\x1b[0m"
)

// Banner lines with gradient colors
var bannerLines = []struct {
	color string
	text  string
}{
	{"\x1b[38;2;217;170;84m", "███╗   ███╗██╗   ██╗██╗  ██╗██╗"},
	{"\x1b[38;2;218;158;75m", "████╗ ████║██║   ██║╚██╗██╔╝██║"},
	{"\x1b[38;2;219;150;71m", "██╔████╔██║██║   ██║ ╚███╔╝ ██║"},
	{"\x1b[38;2;220;143;66m", "██║╚██╔╝██║██║   ██║ ██╔██╗ ██║"},
	{"\x1b[38;2;216;137;62m", "██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗██║"},
	{"\x1b[38;2;191;120;64m", "╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝"},
}

// getArchString returns the architecture string for display
func getArchString() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

// printBanner prints the gradient-colored MUXI banner
func printBanner() {
	fmt.Println()
	for _, line := range bannerLines {
		fmt.Printf("%s%s%s\n", line.color, line.text, resetColor)
	}
}

// printWelcome prints the welcome message with version and arch
func printWelcome() {
	printBanner()
	fmt.Println()
	fmt.Printf("Welcome to MUXI Server %s (ELv2 %s)\n", Version, getArchString())
	fmt.Println()
	fmt.Println(" * Documentation:  https://muxi.org/docs")
	fmt.Println(" * Support:        https://muxi.org/support")
	fmt.Println()
}

// cmdUpgrade handles the 'upgrade' command
// Upgrades the server binary, RCE, and runtime components
func cmdUpgrade() error {
	fmt.Println()
	fmt.Printf("%s MUXI Server Upgrade\n", bullet)
	fmt.Println(strings.Repeat(boxH, 40))
	fmt.Printf("  Current version: %s\n\n", Version)

	upgraded := false

	// Load config
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 1. Upgrade server binary
	fmt.Printf("%s Checking for server updates...\n", bullet)
	latestVersion, err := fetchLatestServerVersion()
	if err != nil {
		fmt.Printf("  %s Could not check for updates: %v\n", crossMark, err)
	} else if latestVersion == Version {
		fmt.Printf("  %s Server is up to date (%s)\n", checkMark, Version)
	} else {
		fmt.Printf("  %s New version available: %s\n", arrowRight, latestVersion)
		if err := upgradeServerBinary(latestVersion); err != nil {
			fmt.Printf("  %s Failed to upgrade server: %v\n", crossMark, err)
		} else {
			fmt.Printf("  %s Server upgraded to %s\n", checkMark, latestVersion)
			upgraded = true
		}
	}

	// 2. Upgrade Skills RCE
	fmt.Printf("\n%s Checking Skills RCE...\n", bullet)
	dataDir, err := config.GetDataDir()
	if err != nil {
		fmt.Printf("  %s Could not get data dir: %v\n", crossMark, err)
	} else if runtime.GOOS == "linux" {
		if _, err := rce.EnsureSIF(dataDir); err != nil {
			fmt.Printf("  %s Failed to update RCE SIF: %v\n", crossMark, err)
		} else {
			fmt.Printf("  %s Skills RCE SIF is up to date\n", checkMark)
			upgraded = true
		}
	} else {
		if err := rce.EnsureDocker(); err != nil {
			fmt.Printf("  %s Failed to pull RCE image: %v\n", crossMark, err)
		} else {
			fmt.Printf("  %s Skills RCE Docker image updated\n", checkMark)
			upgraded = true
		}
	}

	// 3. Migrate config (add missing fields)
	configChanged := false
	if cfg.RCE.AuthToken == "" {
		cfg.RCE.AuthToken = rce.GenerateAuthToken()
		configChanged = true
		fmt.Printf("\n%s Generated RCE auth token\n", checkMark)
	}
	if cfg.RCE.Port == 0 {
		cfg.RCE.Port = findAvailableRCEPort()
		configChanged = true
	}

	if configChanged {
		if err := cfg.Save(configPath); err != nil {
			fmt.Printf("  %s Failed to save config: %v\n", crossMark, err)
		} else {
			fmt.Printf("%s Configuration updated\n", checkMark)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat(boxH, 40))
	if upgraded || configChanged {
		fmt.Printf("%s Upgrade complete!\n", checkMark)
	} else {
		fmt.Printf("%s Everything is up to date.\n", checkMark)
	}
	fmt.Println()

	return nil
}

// fetchLatestServerVersion resolves the latest server release version from GitHub
func fetchLatestServerVersion() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Head("https://github.com/muxi-ai/server/releases/latest")
	if err != nil {
		return "", fmt.Errorf("failed to check latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("expected redirect, got status %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	// Location: https://github.com/muxi-ai/server/releases/tag/v0.20260305.0
	idx := strings.LastIndex(location, "/v")
	if idx == -1 {
		return "", fmt.Errorf("could not parse version from: %s", location)
	}
	return location[idx+2:], nil
}

// upgradeServerBinary downloads and replaces the current server binary
func upgradeServerBinary(version string) error {
	// Determine binary name
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	binaryName := fmt.Sprintf("muxi-server-%s-%s", goos, goarch)
	if goos == "windows" {
		binaryName += ".exe"
	}

	url := fmt.Sprintf("https://releases.muxi.org/server/releases/download/v%s/%s", version, binaryName)

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	// Resolve symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Download to temp file
	tmpPath := exePath + ".new"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download failed: %w", err)
	}
	out.Close()

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Replace old binary: rename current to .old, move new to current
	oldPath := exePath + ".old"
	os.Remove(oldPath) // clean up any previous .old

	if err := os.Rename(exePath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		// Try to restore
		os.Rename(oldPath, exePath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Clean up old binary
	os.Remove(oldPath)

	return nil
}

// cmdInit handles the 'init' command - improved UX version
func cmdInit() error {
	reader := bufio.NewReader(os.Stdin)

	// Print welcome message
	printWelcome()
	fmt.Println("This will initialize your MUXI Server with credentials and configuration.")
	fmt.Println()

	// Get MUXI directory
	muxiDir, err := config.GetMuxiDir()
	if err != nil {
		return fmt.Errorf("failed to get MUXI directory: %w", err)
	}

	// Create server directory if it doesn't exist
	if err := os.MkdirAll(muxiDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	configPath := filepath.Join(muxiDir, "config.yaml")

	// Check for existing config
	var existingConfig *config.Config
	var editMode bool

	if _, err := os.Stat(configPath); err == nil {
		// Config exists - load it
		existingConfig, err = config.Load(configPath)
		if err != nil {
			fmt.Printf("%s Warning: Could not load existing config: %v\n", crossMark, err)
			fmt.Println("Continuing with fresh initialization...")
		} else {
			// Show existing config
			fmt.Printf("%s Configuration file found at %s\n\n", bullet, configPath)
			fmt.Println("Current settings:")
			fmt.Printf("  Server ID: %s\n", existingConfig.ServerID)
			fmt.Printf("  Port:      %d\n", existingConfig.Server.Port)
			fmt.Printf("  Key:       %s\n", maskKey(existingConfig.Auth.Key))
			fmt.Print("\n")

			// Ask Edit or Override
			fmt.Print("Do you want to (E)dit or (O)verride? [E/o]: ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "o" || response == "override" {
				fmt.Print("\n")
				fmt.Println("Starting fresh initialization...")
				editMode = false
				existingConfig = nil
			} else {
				fmt.Print("\n")
				fmt.Println("Editing existing configuration...")
				editMode = true
			}
			fmt.Print("\n")
		}
	}

	// Interactive prompts
	var serverName string
	var port int
	var email string

	// Email first (breaks autopilot pattern)
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Println("EMAIL FOR UPDATES AND NOTIFICATIONS")
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Print("\n")
	fmt.Println("Stay informed about:")
	fmt.Printf("  %s Security updates and patches\n", bullet)
	fmt.Printf("  %s New features and improvements\n", bullet)
	fmt.Printf("  %s Breaking changes and migrations\n", bullet)
	fmt.Print("\n")
	fmt.Print("Email (optional but recommended): ")
	email, _ = reader.ReadString('\n')
	email = strings.TrimSpace(email)

	// Show confirmation if email provided
	if email != "" {
		fmt.Printf("\n%s Thank you! We've sent you a confirmation email.\n", checkMark)
	}
	fmt.Print("\n")
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Print("\n")

	// Server Name
	defaultName := ""
	if editMode && existingConfig != nil {
		defaultName = extractServerName(existingConfig.ServerID)
	}
	if defaultName == "" {
		defaultName, _ = os.Hostname()
		defaultName = strings.ToLower(defaultName) // Lowercase for consistency
	}

	fmt.Printf("Server name [%s]: ", defaultName)
	serverName, _ = reader.ReadString('\n')
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		serverName = defaultName
	}

	// Port with availability check
	defaultPort := 7890
	if editMode && existingConfig != nil {
		defaultPort = existingConfig.Server.Port
	}

	for {
		fmt.Printf("Server port [%d]: ", defaultPort)
		portInput, _ := reader.ReadString('\n')
		portInput = strings.TrimSpace(portInput)

		if portInput == "" {
			port = defaultPort
		} else {
			parsedPort, err := strconv.Atoi(portInput)
			if err != nil {
				fmt.Printf("%s Invalid port number. Please enter a valid port.\n", crossMark)
				continue
			}
			port = parsedPort
		}

		// Check if port is available
		if isPortAvailable(port) {
			break
		} else {
			fmt.Printf("%s Port %d is already in use. Please choose another port.\n", crossMark, port)
			defaultPort = port + 1
		}
	}

	fmt.Print("\n")

	// Generate or reuse credentials
	var key, secret string
	var serverID string

	if editMode && existingConfig != nil {
		// Keep existing credentials
		key = existingConfig.Auth.Key
		secret = existingConfig.Auth.Secret
		// Generate new server ID with the user's chosen name
		serverID = generateServerIDFromName(serverName)
	} else {
		// Generate fresh credentials
		serverID = generateServerIDFromName(serverName)

		key, err = generateKey()
		if err != nil {
			return fmt.Errorf("failed to generate key: %w", err)
		}

		secret, err = generateSecret()
		if err != nil {
			return fmt.Errorf("failed to generate secret: %w", err)
		}
	}

	// Detect OS and handle Docker
	osName := runtime.GOOS
	fmt.Printf("%s %s environment detected\n", bullet, strings.Title(osName))

	if osName == "darwin" || osName == "windows" {
		// Check Docker
		if !checkDockerAvailable() {
			fmt.Printf("%s Docker is required for SIF runtime on %s\n", crossMark, osName)
			fmt.Print("\n")
			fmt.Println("Please install Docker Desktop:")
			if osName == "darwin" {
				fmt.Println("  brew install --cask docker")
			} else {
				fmt.Println("  https://docs.docker.com/desktop/install/windows-install/")
			}
			fmt.Print("\n")
			fmt.Println("After installing Docker, run 'muxi-server init' again.")
			return fmt.Errorf("Docker not found")
		}

		// Header ends with a newline so the spinner-driven progress
		// line renders cleanly on the next row. No size hint in the
		// header — the spinner conveys "working, be patient" better
		// than a static "may take a few minutes".
		if !checkRuntimeRunnerExists() {
			fmt.Printf("%s Setting up runtime-runner...\n", bullet)
			if err := pullRuntimeRunner(); err != nil {
				fmt.Printf("%s Failed to download runtime-runner: %v\n", crossMark, err)
				fmt.Println("   You can pull it manually later:")
				fmt.Println("   docker pull --platform linux/amd64 ghcr.io/muxi-ai/runtime-runner:latest")
			} else {
				fmt.Printf("%s Runtime-runner ready\n", checkMark)
			}
		} else {
			fmt.Printf("%s Runtime-runner ready\n", checkMark)
		}
	} else if osName == "linux" {
		// Check for Singularity or Apptainer
		if !checkSingularityAvailable() {
			fmt.Printf("%s Installing dependencies (Apptainer)...\n", bullet)
			if err := installApptainer(); err != nil {
				fmt.Printf("%s Failed to install Apptainer: %v\n", crossMark, err)
				fmt.Println()
				fmt.Println("Please install Apptainer manually:")
				fmt.Println("  Ubuntu/Debian: sudo apt update && sudo apt install -y apptainer")
				fmt.Println("  RHEL/Fedora:   sudo dnf install -y apptainer")
				fmt.Println("  Other:         https://apptainer.org/docs/admin/main/installation.html")
				fmt.Println()
				fmt.Println("After installing, run 'muxi-server init' again.")
				return fmt.Errorf("Apptainer installation failed")
			}
			fmt.Printf("%s Apptainer installed successfully\n", checkMark)
		} else {
			singularityPath := getSingularityPath()
			fmt.Printf("%s Singularity/Apptainer available: %s\n", checkMark, singularityPath)
		}

		// Ensure timezone data exists (required by Apptainer)
		ensureTimezoneData()
	}

	// Create config
	cfg := &config.Config{
		ServerID: serverID,
		Server: config.ServerConfig{
			Port: port,
			Host: "0.0.0.0",
		},
		Auth: config.AuthConfig{
			Enabled:            true,
			Key:                key,
			Secret:             secret,
			TimestampTolerance: 300,
		},
		Formations: config.FormationsConfig{
			RuntimeType:    "native",
			PortRangeStart: 8000,
			PortRangeEnd:   9000,
			LogsDir:        "logs",       // Relative path, not absolute
			PIDsDir:        "pids",       // Relative path
			FormationsDir:  "formations", // Relative path
			BindHost:       "127.0.0.1",  // Formations bind to localhost
			AutoRestart:    true,
			MaxRestarts:    10,
			RestartDelay:   1,
		},
		Runtime: config.RuntimeConfig{
			SIFBaseURL:         "https://github.com/muxi-ai/runtime/releases/download",
			RuntimeRunnerImage: "ghcr.io/muxi-ai/runtime-runner:latest",
		},
		RCE: config.RCEConfig{
			Port:      findAvailableRCEPort(),
			AuthToken: rce.GenerateAuthToken(),
		},
	}

	// Store email if provided (TODO: add to config struct)
	if email != "" {
		fmt.Printf("%s Email registered: %s\n", checkMark, email)
	}

	// Write config file
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create required directories
	logsDir := filepath.Join(muxiDir, "logs")
	formationsDir := filepath.Join(muxiDir, "formations")

	// Cache dir lives under the data dir (GetDataDir() + /cache) so it is
	// co-located with other per-install state, bind-mounted into every SIF
	// at /opt/hf-cache at formation-start time. Created empty here; the
	// runtime inside the SIF handles populating it on first formation run.
	cacheDir, err := config.GetCacheDir()
	if err != nil {
		return fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	for _, dir := range []string{logsDir, formationsDir, cacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Download Skills RCE
	fmt.Printf("\n%s Setting up Skills RCE...\n", bullet)
	dataDir, _ := config.GetDataDir()
	if runtime.GOOS == "linux" {
		if _, err := rce.EnsureSIF(dataDir); err != nil {
			fmt.Printf("%s Failed to download RCE SIF: %v\n", crossMark, err)
			fmt.Println("  Skills RCE can be downloaded later. Formations will start without code execution.")
		} else {
			fmt.Printf("%s Skills RCE ready\n", checkMark)
		}
	} else {
		if err := rce.EnsureDocker(); err != nil {
			fmt.Printf("%s Failed to pull RCE Docker image: %v\n", crossMark, err)
			fmt.Println("  Skills RCE can be pulled later. Formations will start without code execution.")
		} else {
			fmt.Printf("%s Skills RCE ready\n", checkMark)
		}
	}

	// Pre-download the default embedding model into the HF cache so the
	// first formation deploy doesn't stall on a ~300MB download.
	// Platform-agnostic (pure HTTP — works the same on apptainer and
	// docker-wrapper paths). Best-effort — if HF is unreachable we
	// print a warning and let the runtime inside the SIF fetch the
	// model on first use.
	//
	// UX note: we deliberately say "Setting up embeddings..." rather
	// than "Checking embedding model cache (nomic-ai/…)" because:
	//   1. Users don't care about the model name at init-time; they
	//      care that something named "embeddings" is being arranged.
	//   2. On a cache-hit re-init the old wording printed the full
	//      model URL and a "(skipping download)" parenthetical —
	//      mechanical status updates without useful user signal.
	//   3. The downloadReporter's ticker-driven spinner handles the
	//      "still alive" job; no need for text hints like "may take a
	//      few minutes".
	// Both cached and fresh-download paths converge on the same
	// "Embeddings ready" confirmation; the only difference the user
	// sees is whether a spinner+bytes line appeared in between.
	fmt.Printf("\n%s Setting up embeddings...\n", bullet)
	progress := startDownloadReporter(os.Stdout)
	_, err = hfcache.EnsureLeanModel(cacheDir, progress)
	progress.finish()
	if err != nil {
		fmt.Printf("%s Could not prepare embedding model: %v\n", crossMark, err)
		fmt.Println("  The model will be downloaded on first formation deploy.")
	} else {
		fmt.Printf("%s Embeddings ready\n", checkMark)
	}

	// Success message
	fmt.Print("\n")
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Printf("%s MUXI Server initialized successfully!\n", checkMark)
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Print("\n")

	fmt.Printf("Configuration saved to: %s\n", configPath)
	fmt.Print("\n")

	if !editMode {
		fmt.Println("Your authentication credentials have been generated and saved.")
		fmt.Printf("To view them: muxi-server config show\n")
		fmt.Print("\n")
	}

	// Create CLI profile if CLI is installed
	if isCLIInstalled() {
		result, err := createOrUpdateCLIProfile(port, key, secret)
		if err != nil {
			fmt.Printf("%s Warning: Could not update CLI profile: %v\n", crossMark, err)
		} else {
			home, _ := os.UserHomeDir()
			switch result {
			case ProfileCreated:
				fmt.Printf("%s CLI profile created: %s/.muxi/cli/profiles.yaml\n", checkMark, home)
				fmt.Println("  You can now use 'muxi' commands with the 'localhost' profile.")
				fmt.Print("\n")
			case ProfileUpdated:
				fmt.Printf("%s CLI profile updated: %s/.muxi/cli/profiles.yaml\n", checkMark, home)
				fmt.Println("  The 'localhost' profile credentials have been updated.")
				fmt.Print("\n")
			case ProfileUnchanged:
				fmt.Printf("%s CLI profile already configured (credentials match)\n", checkMark)
				fmt.Print("\n")
			}
		}
	}

	// Offer service setup
	promptServiceSetup(reader, port)

	fmt.Println("Next steps:")
	fmt.Printf("  1. Start the server:    muxi-server start\n")
	fmt.Printf("  2. Check server status: curl http://localhost:%d/health\n", port)
	fmt.Printf("  3. View configuration:  muxi-server config show\n")
	fmt.Print("\n")

	fmt.Println("Documentation: https://muxi.org/docs/getting-started")
	fmt.Print("\n")

	return nil
}

// cmdVersion handles the 'version' command
func cmdVersion() error {
	fmt.Printf("MUXI Server %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Build Time: %s\n", BuildTime)
	return nil
}

// cmdConfigShow handles the 'config show' command
func cmdConfigShow() error {
	// Load config
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Display config (with key and secret masked)
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("   MUXI Server Configuration")
	fmt.Println("═══════════════════════════════════════")
	fmt.Print("\n")
	fmt.Println("Server:")
	fmt.Printf("  ID: %s\n", cfg.ServerID)
	fmt.Printf("  Host: %s\n", cfg.Server.Host)
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Print("\n")
	fmt.Println("Authentication:")
	fmt.Printf("  Enabled: %v\n", cfg.Auth.Enabled)
	fmt.Printf("  Key: %s\n", maskKey(cfg.Auth.Key))
	fmt.Printf("  Secret: %s\n", maskSecret(cfg.Auth.Secret))
	fmt.Printf("  Timestamp Tolerance: %d seconds\n", cfg.Auth.TimestampTolerance)
	fmt.Print("\n")
	fmt.Println("Formations:")
	fmt.Printf("  Runtime Type: %s\n", cfg.Formations.RuntimeType)
	fmt.Printf("  Port Range: %d - %d\n", cfg.Formations.PortRangeStart, cfg.Formations.PortRangeEnd)
	fmt.Printf("  Logs Directory: %s\n", cfg.Formations.LogsDir)
	fmt.Printf("  Auto Restart: %v\n", cfg.Formations.AutoRestart)
	fmt.Printf("  Max Restarts: %d\n", cfg.Formations.MaxRestarts)
	fmt.Print("\n")
	fmt.Printf("Config File: %s\n", configPath)
	fmt.Print("\n")

	return nil
}

// cmdHelp shows usage information
func cmdHelp() {
	printBanner()
	fmt.Println()
	fmt.Printf("MUXI Server %s - Formation Orchestration Platform\n", Version)
	fmt.Print("\n")
	fmt.Println("USAGE")
	fmt.Printf("  muxi-server <command> [options]\n")
	fmt.Print("\n")
	fmt.Println("COMMANDS")
	fmt.Printf("  init           Initialize server with credentials and configuration\n")
	fmt.Printf("  start          Start the MUXI Server (default)\n")
	fmt.Printf("  upgrade        Upgrade server binary, RCE, and runtime components\n")
	fmt.Printf("  version        Display version information\n")
	fmt.Printf("  config show    Display current configuration\n")
	fmt.Printf("  help           Show this help message\n")
	fmt.Print("\n")
	fmt.Println("OPTIONS (for start command)")
	fmt.Printf("  --log-level=LEVEL   Set log level: debug, info, warn, error (default: info)\n")
	fmt.Print("\n")
	fmt.Println("ENVIRONMENT")
	fmt.Printf("  MUXI_LOG_LEVEL      Set log level (overridden by --log-level flag)\n")
	fmt.Print("\n")
	fmt.Println("EXAMPLES")
	fmt.Printf("  %s muxi-server init                    %s First-time setup\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server start                   %s Start the server\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server start --log-level=debug %s Start with debug logging\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server version                 %s Show version\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server config show             %s View configuration\n", arrowRight, bullet)
	fmt.Print("\n")
	fmt.Println("DOCUMENTATION")
	fmt.Printf("  %s https://muxi.org/docs\n", arrowRight)
	fmt.Printf("  %s https://github.com/muxi-ai/server\n", arrowRight)
	fmt.Print("\n")
}

// Helper functions

// findAvailableRCEPort finds an available port for the RCE service,
// starting from the default (7891) and scanning upward if occupied.
func findAvailableRCEPort() int {
	const defaultPort = 7891
	const maxAttempts = 100

	for port := defaultPort; port < defaultPort+maxAttempts; port++ {
		if registry.IsPortAvailable(port) {
			return port
		}
	}
	return defaultPort
}

// generateKey generates a random MUXI key
func generateKey() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "muxi_pk_" + hex.EncodeToString(bytes), nil
}

// generateSecret generates a random secret key
func generateSecret() (string, error) {
	bytes := make([]byte, 28) // 56 hex chars (total length: 64 with muxi_sk_ prefix)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "muxi_sk_" + hex.EncodeToString(bytes), nil
}

// maskSecret masks a secret for display
func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:8] + "..." + secret[len(secret)-4:]
}

// maskKey masks a key for display
func maskKey(key string) string {
	if len(key) <= 12 {
		return "muxi_pk_••••••••"
	}
	return key[:12] + "••••••••"
}

// checkDockerAvailable checks if Docker is installed and available
func checkDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// checkRuntimeRunnerExists checks if the runtime-runner image is already pulled
func checkRuntimeRunnerExists() bool {
	cmd := exec.Command("docker", "images", "-q", "ghcr.io/muxi-ai/runtime-runner:latest")
	output, err := cmd.Output()
	return err == nil && len(output) > 0
}

// pullRuntimeRunner pulls the runtime-runner image from GHCR and renders
// a single collapsed progress line instead of Docker's native per-layer
// output (50+ lines for a typical multi-layer pull).
//
// Design: stdout is piped into renderPullProgress which watches for
// "Pulling fs layer" (increments the total) and "Pull complete"
// (increments the done-count) events and repaints a single in-place
// line via \r. Stderr stays wired to the terminal so real Docker
// errors (auth, network, daemon down) remain visible at full fidelity.
//
// DOCKER_CLI_HINTS=false suppresses the "What's next:" promotional
// footer Docker Desktop appends after every pull (docker scout
// quickview…). It's noise during a bootstrap flow where we're already
// managing user attention carefully.
func pullRuntimeRunner() error {
	// Always pull linux/amd64 since Singularity only runs on Linux x86_64;
	// Docker on ARM64 (Apple Silicon) will run it through emulation.
	cmd := exec.Command("docker", "pull", "--platform", "linux/amd64", "ghcr.io/muxi-ai/runtime-runner:latest")
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
		renderPullProgress(stdout, os.Stdout)
	}()

	waitErr := cmd.Wait()
	<-done
	return waitErr
}

// spinnerFrames is a braille-dot spinner rotated by renderPullProgress
// and downloadReporter to signal "still working" during long silent
// phases of a download. Braille dots are monospace and render crisply
// in every modern terminal muxi-server targets (Terminal.app, iTerm2,
// Windows Terminal, VS Code, etc.); we don't fall back to ASCII "/-\|"
// because every platform we support has had Unicode-capable defaults
// since long before the oldest supported macOS/Windows version.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerTick is the repaint interval. 100 ms is fast enough for the
// dot sequence to look alive, slow enough to not flood a serial tty.
const spinnerTick = 100 * time.Millisecond

// renderPullProgress collapses Docker's verbose non-TTY pull output
// (one line per layer × five events per layer = a wall of text) into a
// single in-place progress line with an animated spinner:
//
//	⠙ Layers 5/8 (62%)
//
// The spinner ticks independently of event arrival so the line keeps
// animating during a large silent layer download — the exact moment
// the user wonders "is this hung?".
//
// Event parsing:
//
//	"Pulling fs layer"   increments the total layer count
//	"Pull complete"      increments the completed count
//
// The running total adapts as new layers are announced; when Docker
// staggers announcements the display may briefly show e.g. 3/5 then
// 3/8, which is honest reporting of what's actually known at each
// moment rather than a falsified precomputed total.
//
// If the transcript yields zero "Pulling fs layer" lines (image was
// already up to date — rare because cmdInit guards with
// checkRuntimeRunnerExists), the function prints nothing and the
// caller's success message takes over the line.
func renderPullProgress(in io.Reader, out io.Writer) {
	// Producer goroutine: drain the scanner into a buffered channel so
	// the ticker-driven renderer can select between new events and
	// spinner ticks without blocking.
	events := make(chan string, 128)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(in)
		// Docker can emit long lines on some statuses; bump the buffer
		// from the default 64 KiB to 1 MiB to be safe.
		scanner.Buffer(make([]byte, 1<<16), 1<<20)
		for scanner.Scan() {
			events <- scanner.Text()
		}
	}()

	ticker := time.NewTicker(spinnerTick)
	defer ticker.Stop()

	var total, pulled, frame int
	repaint := func() {
		if total == 0 {
			return
		}
		pct := 100 * pulled / total
		// Trailing spaces clear any leftover characters from a longer
		// previous line (single-digit → double-digit count transitions).
		fmt.Fprintf(out, "\r  %s Layers %d/%d (%d%%)   ",
			spinnerFrames[frame%len(spinnerFrames)], pulled, total, pct)
	}

	for {
		select {
		case line, ok := <-events:
			if !ok {
				if total > 0 {
					// Close out the in-place line with a real newline
					// so whatever prints next doesn't overwrite our
					// final status.
					fmt.Fprintln(out)
				}
				return
			}
			switch {
			case strings.Contains(line, "Pulling fs layer"):
				total++
			case strings.Contains(line, "Pull complete"):
				pulled++
			}
			repaint()
		case <-ticker.C:
			frame++
			repaint()
		}
	}
}

// downloadReporter is an io.Writer that accumulates bytes written to it
// and renders a single in-place spinner + MiB-downloaded line via a
// ticker goroutine. Replaces the older progressPrinter which only
// printed every 4 MiB threshold and therefore appeared frozen during
// slow transfers.
//
// Lifecycle:
//
//	r := startDownloadReporter(os.Stdout)
//	// pass r.Writer() somewhere that reads into it
//	r.finish()
//
// Concurrency: Write is called from the HTTP-read goroutine; render is
// called from an internal ticker goroutine. The total byte count is
// atomic-protected; we don't guard the final newline because finish()
// synchronizes via a done-channel before returning.
type downloadReporter struct {
	out    io.Writer
	bytes  atomic.Int64
	stop   chan struct{}
	done   chan struct{}
	render bool // true once at least one repaint has happened; guards final newline
}

func startDownloadReporter(out io.Writer) *downloadReporter {
	r := &downloadReporter{
		out:  out,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *downloadReporter) Write(b []byte) (int, error) {
	r.bytes.Add(int64(len(b)))
	return len(b), nil
}

func (r *downloadReporter) run() {
	defer close(r.done)
	ticker := time.NewTicker(spinnerTick)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			n := r.bytes.Load()
			if n == 0 {
				// Nothing has flowed yet; don't paint a "0 MiB" line
				// that would just confuse during the HTTP connect phase.
				continue
			}
			r.render = true
			fmt.Fprintf(r.out, "\r  %s %.1f MiB downloaded   ",
				spinnerFrames[frame%len(spinnerFrames)],
				float64(n)/1024/1024)
			frame++
		}
	}
}

// finish signals the ticker to stop, waits for it to exit, and writes
// a terminating newline if any progress was actually painted. Safe to
// call even if no bytes flowed (empty cache-skip path).
func (r *downloadReporter) finish() {
	close(r.stop)
	<-r.done
	if r.render {
		fmt.Fprintln(r.out)
	}
}

// checkSingularityAvailable checks if Singularity or Apptainer is installed
func checkSingularityAvailable() bool {
	// Check for apptainer first (newer, community fork)
	if _, err := exec.LookPath("apptainer"); err == nil {
		return true
	}
	// Check for singularity
	if _, err := exec.LookPath("singularity"); err == nil {
		return true
	}
	return false
}

// getSingularityPath returns the path to singularity or apptainer binary
func getSingularityPath() string {
	if path, err := exec.LookPath("apptainer"); err == nil {
		return path
	}
	if path, err := exec.LookPath("singularity"); err == nil {
		return path
	}
	return ""
}

// getLinuxDistro returns the Linux distribution ID from /etc/os-release
func getLinuxDistro() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			id := strings.TrimPrefix(line, "ID=")
			id = strings.Trim(id, "\"")
			return strings.ToLower(id)
		}
	}
	return ""
}

// getLinuxDistroLike returns the ID_LIKE field from /etc/os-release (parent distros)
func getLinuxDistroLike() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID_LIKE=") {
			idLike := strings.TrimPrefix(line, "ID_LIKE=")
			idLike = strings.Trim(idLike, "\"")
			return strings.ToLower(idLike)
		}
	}
	return ""
}

// installApptainer installs Apptainer based on the Linux distribution
func installApptainer() error {
	distro := getLinuxDistro()
	distroLike := getLinuxDistroLike()

	// Helper to run a command with output
	runCmd := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	switch distro {
	case "ubuntu":
		// Ubuntu: add Apptainer PPA first
		// Install software-properties-common for add-apt-repository
		if err := runCmd("apt-get", "update"); err != nil {
			return fmt.Errorf("apt-get update failed: %w", err)
		}
		if err := runCmd("apt-get", "install", "-y", "software-properties-common"); err != nil {
			return fmt.Errorf("failed to install software-properties-common: %w", err)
		}
		if err := runCmd("add-apt-repository", "-y", "ppa:apptainer/ppa"); err != nil {
			return fmt.Errorf("failed to add Apptainer PPA: %w", err)
		}
		if err := runCmd("apt-get", "update"); err != nil {
			return fmt.Errorf("apt-get update failed: %w", err)
		}
		if err := runCmd("apt-get", "install", "-y", "apptainer"); err != nil {
			return fmt.Errorf("apt-get install apptainer failed: %w", err)
		}
		return nil

	case "debian", "linuxmint", "pop":
		// Debian-based: download and install .deb package directly
		// Apptainer PPA is Ubuntu-specific, so we use the release .deb
		if err := runCmd("apt-get", "update"); err != nil {
			return fmt.Errorf("apt-get update failed: %w", err)
		}
		// Install dependencies
		if err := runCmd("apt-get", "install", "-y", "wget"); err != nil {
			return fmt.Errorf("failed to install wget: %w", err)
		}
		// Download and install Apptainer .deb
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "amd64"
		} else {
			arch = "arm64"
		}
		debURL := fmt.Sprintf("https://github.com/apptainer/apptainer/releases/download/v1.3.0/apptainer_1.3.0_linux_%s.deb", arch)
		if err := runCmd("wget", "-q", debURL, "-O", "/tmp/apptainer.deb"); err != nil {
			return fmt.Errorf("failed to download Apptainer: %w", err)
		}
		if err := runCmd("apt-get", "install", "-y", "/tmp/apptainer.deb"); err != nil {
			return fmt.Errorf("failed to install Apptainer: %w", err)
		}
		return nil

	case "fedora", "rhel", "centos", "rocky", "almalinux", "ol":
		// RHEL-based: use dnf with EPEL
		if err := runCmd("dnf", "install", "-y", "epel-release"); err != nil {
			// EPEL might not be needed on Fedora, continue anyway
		}
		if err := runCmd("dnf", "install", "-y", "apptainer"); err != nil {
			return fmt.Errorf("dnf install apptainer failed: %w", err)
		}
		return nil

	case "arch", "manjaro":
		// Arch-based: use pacman
		if err := runCmd("pacman", "-S", "--noconfirm", "apptainer"); err != nil {
			return fmt.Errorf("pacman install failed: %w", err)
		}
		return nil

	case "opensuse", "sles":
		// SUSE-based: use zypper
		if err := runCmd("zypper", "install", "-y", "apptainer"); err != nil {
			return fmt.Errorf("zypper install failed: %w", err)
		}
		return nil

	default:
		// Check ID_LIKE for derivative distros
		if strings.Contains(distroLike, "ubuntu") {
			// Ubuntu derivative - try PPA
			if err := runCmd("apt-get", "update"); err != nil {
				return fmt.Errorf("apt-get update failed: %w", err)
			}
			if err := runCmd("apt-get", "install", "-y", "software-properties-common"); err != nil {
				return fmt.Errorf("failed to install software-properties-common: %w", err)
			}
			if err := runCmd("add-apt-repository", "-y", "ppa:apptainer/ppa"); err != nil {
				return fmt.Errorf("failed to add Apptainer PPA: %w", err)
			}
			if err := runCmd("apt-get", "update"); err != nil {
				return fmt.Errorf("apt-get update failed: %w", err)
			}
			if err := runCmd("apt-get", "install", "-y", "apptainer"); err != nil {
				return fmt.Errorf("apt-get install apptainer failed: %w", err)
			}
			return nil
		} else if strings.Contains(distroLike, "debian") {
			// Debian derivative - download .deb
			if err := runCmd("apt-get", "update"); err != nil {
				return fmt.Errorf("apt-get update failed: %w", err)
			}
			if err := runCmd("apt-get", "install", "-y", "wget"); err != nil {
				return fmt.Errorf("failed to install wget: %w", err)
			}
			arch := runtime.GOARCH
			if arch == "amd64" {
				arch = "amd64"
			} else {
				arch = "arm64"
			}
			debURL := fmt.Sprintf("https://github.com/apptainer/apptainer/releases/download/v1.3.0/apptainer_1.3.0_linux_%s.deb", arch)
			if err := runCmd("wget", "-q", debURL, "-O", "/tmp/apptainer.deb"); err != nil {
				return fmt.Errorf("failed to download Apptainer: %w", err)
			}
			if err := runCmd("apt-get", "install", "-y", "/tmp/apptainer.deb"); err != nil {
				return fmt.Errorf("failed to install Apptainer: %w", err)
			}
			return nil
		} else if strings.Contains(distroLike, "rhel") || strings.Contains(distroLike, "fedora") {
			if err := runCmd("dnf", "install", "-y", "apptainer"); err != nil {
				return fmt.Errorf("dnf install apptainer failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("unsupported Linux distribution: %s", distro)
	}
}

// ensureTimezoneData ensures /etc/localtime exists (required by Apptainer)
func ensureTimezoneData() {
	// Check if /etc/localtime already exists
	if _, err := os.Stat("/etc/localtime"); err == nil {
		return
	}

	// Try to install tzdata and create localtime
	distro := getLinuxDistro()
	distroLike := getLinuxDistroLike()

	switch distro {
	case "ubuntu", "debian":
		runCommand("apt-get", "update")
		runCommand("apt-get", "install", "-y", "tzdata")
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		runCommand("dnf", "install", "-y", "tzdata")
	case "arch", "manjaro":
		runCommand("pacman", "-S", "--noconfirm", "tzdata")
	default:
		if strings.Contains(distroLike, "debian") || strings.Contains(distroLike, "ubuntu") {
			runCommand("apt-get", "update")
			runCommand("apt-get", "install", "-y", "tzdata")
		} else if strings.Contains(distroLike, "rhel") || strings.Contains(distroLike, "fedora") {
			runCommand("dnf", "install", "-y", "tzdata")
		}
	}

	// Create /etc/localtime symlink if it still doesn't exist
	if _, err := os.Stat("/etc/localtime"); os.IsNotExist(err) {
		// Try common timezone file locations
		tzFiles := []string{
			"/usr/share/zoneinfo/UTC",
			"/usr/share/zoneinfo/Etc/UTC",
			"/usr/share/lib/zoneinfo/UTC",
		}
		for _, tzFile := range tzFiles {
			if _, err := os.Stat(tzFile); err == nil {
				os.Symlink(tzFile, "/etc/localtime")
				return
			}
		}
	}
}

// extractServerName extracts hostname from server ID (format: server-{hostname}-{hash})
func extractServerName(serverID string) string {
	parts := strings.Split(serverID, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// generateServerIDFromName generates a server ID from a name
func generateServerIDFromName(name string) string {
	// Format: server-{name}-{short-hash}
	hash := make([]byte, 4)
	rand.Read(hash)
	return fmt.Sprintf("server-%s-%x", name, hash)
}

// isPortAvailable checks if a port is available for binding
func isPortAvailable(port int) bool {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// ============================================================================
// CLI Profile Management
// ============================================================================

// isCLIInstalled checks if the muxi CLI is installed
func isCLIInstalled() bool {
	// Check PATH
	if _, err := exec.LookPath("muxi"); err == nil {
		return true
	}
	// Check common locations
	home, _ := os.UserHomeDir()
	locations := []string{
		filepath.Join(home, ".local", "bin", "muxi"),
		"/usr/local/bin/muxi",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return true
		}
	}
	return false
}

// CLIProfile represents a single CLI profile
type CLIProfile struct {
	URL       string `yaml:"url"`
	KeyID     string `yaml:"key_id"`
	SecretKey string `yaml:"secret_key"`
	AddedAt   string `yaml:"added_at"`
}

// CLIProfiles represents the profiles.yaml structure
type CLIProfiles struct {
	Version  string                `yaml:"version"`
	Default  string                `yaml:"default"`
	Profiles map[string]CLIProfile `yaml:"profiles"`
}

// ProfileUpdateResult indicates what happened during profile update
type ProfileUpdateResult int

const (
	ProfileCreated ProfileUpdateResult = iota
	ProfileUpdated
	ProfileUnchanged
)

// createOrUpdateCLIProfile creates or updates the CLI profile for localhost
// Returns the result indicating what action was taken
func createOrUpdateCLIProfile(port int, keyID, secretKey string) (ProfileUpdateResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ProfileUnchanged, fmt.Errorf("failed to get home directory: %w", err)
	}

	profilesDir := filepath.Join(home, ".muxi", "cli")
	profilesPath := filepath.Join(profilesDir, "profiles.yaml")

	// Ensure directory exists
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		return ProfileUnchanged, fmt.Errorf("failed to create CLI profiles directory: %w", err)
	}

	// Load existing profiles or create new
	var profiles CLIProfiles
	fileExists := false
	if data, err := os.ReadFile(profilesPath); err == nil {
		fileExists = true
		if err := yaml.Unmarshal(data, &profiles); err != nil {
			// If parsing fails, start fresh
			profiles = CLIProfiles{
				Version:  "1.0",
				Profiles: make(map[string]CLIProfile),
			}
		}
	} else {
		profiles = CLIProfiles{
			Version:  "1.0",
			Profiles: make(map[string]CLIProfile),
		}
	}

	// Ensure profiles map is initialized
	if profiles.Profiles == nil {
		profiles.Profiles = make(map[string]CLIProfile)
	}

	newURL := fmt.Sprintf("http://localhost:%d", port)
	result := ProfileCreated

	// Check if localhost profile exists and if credentials match
	if existing, exists := profiles.Profiles["localhost"]; exists {
		if existing.URL == newURL && existing.KeyID == keyID && existing.SecretKey == secretKey {
			// Credentials match, no update needed
			return ProfileUnchanged, nil
		}
		result = ProfileUpdated
	} else if fileExists {
		// File exists but no localhost profile - we're adding to existing file
		result = ProfileCreated
	}

	// Add/update localhost profile
	profiles.Profiles["localhost"] = CLIProfile{
		URL:       newURL,
		KeyID:     keyID,
		SecretKey: secretKey,
		AddedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Set default if not set
	if profiles.Default == "" {
		profiles.Default = "localhost"
	}

	// Write back
	data, err := yaml.Marshal(&profiles)
	if err != nil {
		return ProfileUnchanged, fmt.Errorf("failed to marshal profiles: %w", err)
	}

	if err := os.WriteFile(profilesPath, data, 0600); err != nil {
		return ProfileUnchanged, fmt.Errorf("failed to write profiles file: %w", err)
	}

	return result, nil
}

// ============================================================================
// Service/Daemon Setup
// ============================================================================

// getMuxiServerPath returns the path to the muxi-server binary
func getMuxiServerPath() string {
	// Try to find the current executable
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	// Fallback to common locations
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".local", "bin", "muxi-server"),
		"/usr/local/bin/muxi-server",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "muxi-server" // fallback to PATH
}

// isRunningInContainer checks if we're running inside a container
func isRunningInContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check for container environment variable (Podman, etc.)
	if os.Getenv("container") != "" {
		return true
	}
	return false
}

// runCommand runs a command, using sudo if not root
func runCommand(name string, args ...string) error {
	if os.Getuid() != 0 {
		// Not root, use sudo
		args = append([]string{name}, args...)
		name = "sudo"
	}
	return exec.Command(name, args...).Run()
}

// runCommandWithStdin runs a command with stdin, using sudo if not root
func runCommandWithStdin(stdin string, name string, args ...string) error {
	if os.Getuid() != 0 {
		// Not root, use sudo
		args = append([]string{name}, args...)
		name = "sudo"
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = nil
	return cmd.Run()
}

// setupSystemdService creates and enables a systemd service (Linux)
func setupSystemdService() error {
	// Check if running in a container
	if isRunningInContainer() {
		return fmt.Errorf("container environment detected (systemd not available)")
	}

	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}

	serverPath := getMuxiServerPath()

	serviceContent := fmt.Sprintf(`[Unit]
Description=MUXI Server
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, user, serverPath)

	servicePath := "/etc/systemd/system/muxi-server.service"

	// Write service file (requires sudo if not root)
	if err := runCommandWithStdin(serviceContent, "tee", servicePath); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}

	// Reload systemd
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := runCommand("systemctl", "enable", "muxi-server"); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start service
	if err := runCommand("systemctl", "start", "muxi-server"); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// setupLaunchdService creates and loads a launchd service (macOS)
func setupLaunchdService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	serverPath := getMuxiServerPath()

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>org.muxi.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/.muxi/server/logs/launchd.log</string>
    <key>StandardErrorPath</key>
    <string>%s/.muxi/server/logs/launchd.log</string>
</dict>
</plist>
`, serverPath, home, home)

	// Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "org.muxi.server.plist")

	// Unload existing service if present (ignore errors)
	exec.Command("launchctl", "unload", plistPath).Run()

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load service
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

// promptServiceSetup asks the user if they want to set up as a service
func promptServiceSetup(reader *bufio.Reader, port int) {
	// Only offer on Linux and macOS
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return
	}

	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Println("SYSTEM SERVICE SETUP")
	fmt.Println(strings.Repeat(boxH, 60))
	fmt.Println()
	fmt.Println("Would you like to run MUXI Server as a system service?")
	fmt.Println("This will start the server automatically on boot.")
	fmt.Println()
	fmt.Print("Set up as service? [y/N]: ")

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println()
		fmt.Printf("%s Skipped service setup. Start manually with: muxi-server start\n", bullet)
		fmt.Println()
		return
	}

	fmt.Println()

	var err error
	switch runtime.GOOS {
	case "linux":
		fmt.Printf("%s Creating systemd service...\n", bullet)
		err = setupSystemdService()
		if err == nil {
			fmt.Printf("%s Created /etc/systemd/system/muxi-server.service\n", checkMark)
			fmt.Printf("%s Enabled muxi-server.service\n", checkMark)
			fmt.Printf("%s Started muxi-server.service\n", checkMark)
		}
	case "darwin":
		fmt.Printf("%s Creating launchd service...\n", bullet)
		err = setupLaunchdService()
		if err == nil {
			home, _ := os.UserHomeDir()
			fmt.Printf("%s Created %s/Library/LaunchAgents/org.muxi.server.plist\n", checkMark, home)
			fmt.Printf("%s Loaded org.muxi.server\n", checkMark)
		}
	}

	if err != nil {
		fmt.Printf("%s Failed to set up service: %v\n", crossMark, err)
		fmt.Printf("  You can start the server manually: muxi-server start\n")
	} else {
		fmt.Println()
		fmt.Printf("%s Server is running at http://localhost:%d\n", checkMark, port)
	}
	fmt.Println()
}
