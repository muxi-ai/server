package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/formation"
	"gopkg.in/yaml.v3"
)

// Version information (set by build)
var (
	Version   = "1.0.0-dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// cmdInit handles the 'init' command
// Generates credentials and creates config file
func cmdInit() error {
	fmt.Println("🔐 Initializing MUXI Server...")
	fmt.Println()

	// Get MUXI directory (already points to ~/.muxi/server)
	muxiDir, err := config.GetMuxiDir()
	if err != nil {
		return fmt.Errorf("failed to get MUXI directory: %w", err)
	}

	// Create server directory
	if err := os.MkdirAll(muxiDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	configPath := filepath.Join(muxiDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("⚠️  Config file already exists: %s\n", configPath)
		fmt.Print("Overwrite? (y/N): ")

		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
		fmt.Println()
	}

	// Generate server ID
	serverID, err := formation.GenerateServerID()
	if err != nil {
		return fmt.Errorf("failed to generate server ID: %w", err)
	}

	// Generate credentials
	key, err := generateKey()
	if err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	secret, err := generateSecret()
	if err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	// Create default config
	cfg := &config.Config{
		ServerID: serverID,
		Server: config.ServerConfig{
			Port: 7890, // Official MUXI Port
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
			LogsDir:        filepath.Join(muxiDir, "logs"),
			AutoRestart:    true,
			MaxRestarts:    10,
			RestartDelay:   1,
		},
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

	// Print success message
	fmt.Println("✅ MUXI Server initialized successfully!")
	fmt.Println()
	fmt.Println("📁 Configuration saved to:")
	fmt.Printf("   %s\n", configPath)
	fmt.Println()
	fmt.Println("🆔 Server ID:")
	fmt.Printf("   %s\n", serverID)
	fmt.Println()
	fmt.Println("🔑 Authentication Credentials:")
	fmt.Printf("   Key:    %s\n", key)
	fmt.Printf("   Secret: %s\n", secret)
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Keep your secret secure!")
	fmt.Println("   Never commit it to version control or share it publicly.")
	fmt.Println()
	
	// Runtime runner setup for non-Linux systems
	fmt.Println("🐳 Runtime Execution:")
	fmt.Println("   MUXI Server can run SIF formations using:")
	fmt.Println("   - Linux: Native Singularity execution")
	fmt.Println("   - macOS/Windows: Docker wrapper (runtime-runner)")
	fmt.Println()
	
	// Check if Docker is available and pull runtime-runner if needed
	if checkDockerAvailable() {
		if !checkRuntimeRunnerExists() {
			fmt.Println("   📦 Pulling runtime-runner image...")
			if err := pullRuntimeRunner(); err != nil {
				fmt.Printf("   ⚠️  Failed to pull runtime-runner: %v\n", err)
				fmt.Println("   You can pull it manually later with:")
				fmt.Println("   docker pull ghcr.io/muxi-ai/runtime-runner:latest")
			} else {
				fmt.Println("   ✅ Runtime-runner image ready!")
			}
		} else {
			fmt.Println("   ✅ Runtime-runner image already available")
		}
	} else {
		fmt.Println("   ⚠️  Docker not found. To enable SIF support, install Docker and run:")
		fmt.Println("   docker pull ghcr.io/muxi-ai/runtime-runner:latest")
	}
	fmt.Println()
	
	fmt.Println("📝 Next steps:")
	fmt.Println("   1. Review configuration: muxi-server config show")
	fmt.Println("   2. Start server: muxi-server start")
	fmt.Println("   3. Add credentials to CLI profile:")
	fmt.Printf("      muxi config add-profile default --key=%s --secret=%s\n", key, secret)
	fmt.Println()

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

	// Display config (with secret masked)
	fmt.Println("📋 MUXI Server Configuration")
	fmt.Println()
	fmt.Println("Server:")
	fmt.Printf("  ID: %s\n", cfg.ServerID)
	fmt.Printf("  Host: %s\n", cfg.Server.Host)
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Println()
	fmt.Println("Authentication:")
	fmt.Printf("  Enabled: %v\n", cfg.Auth.Enabled)
	fmt.Printf("  Key: %s\n", cfg.Auth.Key)
	fmt.Printf("  Secret: %s\n", maskSecret(cfg.Auth.Secret))
	fmt.Printf("  Timestamp Tolerance: %d seconds\n", cfg.Auth.TimestampTolerance)
	fmt.Println()
	fmt.Println("Formations:")
	fmt.Printf("  Runtime Type: %s\n", cfg.Formations.RuntimeType)
	fmt.Printf("  Port Range: %d - %d\n", cfg.Formations.PortRangeStart, cfg.Formations.PortRangeEnd)
	fmt.Printf("  Logs Directory: %s\n", cfg.Formations.LogsDir)
	fmt.Printf("  Auto Restart: %v\n", cfg.Formations.AutoRestart)
	fmt.Printf("  Max Restarts: %d\n", cfg.Formations.MaxRestarts)
	fmt.Println()
	fmt.Printf("Config File: %s\n", configPath)
	fmt.Println()

	return nil
}

// cmdHelp shows usage information
func cmdHelp() {
	fmt.Println("MUXI Server - Formation Orchestration Platform")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  muxi-server <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init           Generate credentials and initialize configuration")
	fmt.Println("  start          Start the MUXI Server (default if no command)")
	fmt.Println("  version        Show version information")
	fmt.Println("  config show    Display current configuration")
	fmt.Println("  help           Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  muxi-server init               # First-time setup")
	fmt.Println("  muxi-server start              # Start the server")
	fmt.Println("  muxi-server version            # Show version")
	fmt.Println("  muxi-server config show        # View configuration")
	fmt.Println()
	fmt.Println("For more information, visit: https://github.com/muxi-ai/server")
	fmt.Println()
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
	bytes := make([]byte, 32) // 64 hex chars
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
	cmd := exec.Command("docker", "pull", "ghcr.io/muxi-ai/runtime-runner:latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
