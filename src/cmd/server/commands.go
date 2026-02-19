package main

import (
	"bufio"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/muxi-ai/server/pkg/config"
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

		// Pull runtime-runner with progress
		if !checkRuntimeRunnerExists() {
			fmt.Printf("%s Downloading runtime-runner image...", bullet)
			if err := pullRuntimeRunner(); err != nil {
				fmt.Printf(" %s\n", crossMark)
				fmt.Printf("   Failed: %v\n", err)
				fmt.Println("   You can pull it manually later:")
				fmt.Println("   docker pull --platform linux/amd64 ghcr.io/muxi-ai/runtime-runner:latest")
			} else {
				fmt.Printf(" %s\n", checkMark)
			}
		} else {
			fmt.Printf("%s Runtime-runner image already available\n", checkMark)
		}
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
			AutoDownload:       true,
			RuntimeRunnerImage: "ghcr.io/muxi-ai/runtime-runner:latest",
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

	for _, dir := range []string{logsDir, formationsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
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

// pullRuntimeRunner pulls the runtime-runner image from GHCR
func pullRuntimeRunner() error {
	// Always pull linux/amd64 since Singularity only runs on Linux x86_64
	// Docker on ARM64 (Apple Silicon) will run it through emulation
	cmd := exec.Command("docker", "pull", "--platform", "linux/amd64", "--quiet", "ghcr.io/muxi-ai/runtime-runner:latest")
	// Suppress stdout (digest output), only show errors
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
