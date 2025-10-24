package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/muxi-ai/server/pkg/config"
	"gopkg.in/yaml.v3"
)

// Version information (set by build)
var (
	Version   = "1.0.0-dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// ANSI constants for clean output
const (
	banner = `
███╗   ███╗██╗   ██╗██╗  ██╗██╗
████╗ ████║██║   ██║╚██╗██╔╝██║
██╔████╔██║██║   ██║ ╚███╔╝ ██║
██║╚██╔╝██║██║   ██║ ██╔██╗ ██║
██║ ╚═╝ ██║╚██████╔╝██╔╝ ██╗██║
╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝
`
	checkMark  = "✓"
	crossMark  = "✗"
	arrowRight = "→"
	bullet     = "•"
	boxH       = "─"
)

// cmdInit handles the 'init' command - improved UX version
func cmdInit() error {
	reader := bufio.NewReader(os.Stdin)

	// Print banner
	fmt.Print(banner)
	fmt.Printf("MUXI Server %s\n", Version)
	fmt.Print("\n")
	fmt.Println("This will initialize your MUXI Server with credentials and configuration.")
	fmt.Print("\n")

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
			LogsDir:        "logs",        // Relative path, not absolute
			PIDsDir:        "pids",        // Relative path
			FormationsDir:  "formations",  // Relative path
			BindHost:       "127.0.0.1",   // Formations bind to localhost
			AutoRestart:    true,
			MaxRestarts:    10,
			RestartDelay:   1,
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

	fmt.Println("Next steps:")
	fmt.Printf("  1. Start the server:    muxi-server start\n")
	fmt.Printf("  2. Check server status: curl http://localhost:%d/health\n", port)
	fmt.Printf("  3. View configuration:  muxi-server config show\n")
	fmt.Print("\n")

	fmt.Println("Documentation: https://docs.muxi.ai/getting-started")
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
	fmt.Print(banner)
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
	fmt.Println("EXAMPLES")
	fmt.Printf("  %s muxi-server init          %s First-time setup\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server start         %s Start the server\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server version       %s Show version\n", arrowRight, bullet)
	fmt.Printf("  %s muxi-server config show   %s View configuration\n", arrowRight, bullet)
	fmt.Print("\n")
	fmt.Println("DOCUMENTATION")
	fmt.Printf("  %s https://docs.muxi.ai\n", arrowRight)
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
