package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/muxi-ai/server/pkg/api"
	"github.com/muxi-ai/server/pkg/auth"
	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Parse command from args
	command := "start" // default command
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	// Route to appropriate command
	var err error
	switch command {
	case "init":
		err = cmdInit()
	case "version":
		err = cmdVersion()
	case "config":
		if len(os.Args) > 2 && os.Args[2] == "show" {
			err = cmdConfigShow()
		} else {
			cmdHelp()
			os.Exit(1)
		}
	case "help", "-h", "--help":
		cmdHelp()
		return
	case "start":
		err = cmdStart()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		cmdHelp()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseLogLevel parses a log level string (case-insensitive)
func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// getLogLevel determines log level from flag > env > default
func getLogLevel() zerolog.Level {
	// Check for --log-level flag
	for i, arg := range os.Args {
		if arg == "--log-level" && i+1 < len(os.Args) {
			return parseLogLevel(os.Args[i+1])
		}
		if strings.HasPrefix(arg, "--log-level=") {
			return parseLogLevel(strings.TrimPrefix(arg, "--log-level="))
		}
	}

	// Check MUXI_LOG_LEVEL env var
	if level := os.Getenv("MUXI_LOG_LEVEL"); level != "" {
		return parseLogLevel(level)
	}

	// Default to INFO
	return zerolog.InfoLevel
}

// cmdStart starts the MUXI Server
func cmdStart() error {
	// Determine log level (--log-level > MUXI_LOG_LEVEL > INFO)
	logLevel := getLogLevel()

	// Setup logging
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Logger().
		Level(logLevel)
	log.Logger = logger
	zerolog.SetGlobalLevel(logLevel)

	logger.Info().Msgf("MUXI Server (v%s): Starting...", Version)

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get config path")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Ensure server_id exists (generate if missing for backward compatibility)
	if cfg.ServerID == "" {
		logger.Warn().Msg("No server_id found in config, generating new one")
		serverID, err := formation.GenerateServerID()
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to generate server ID")
		}
		cfg.ServerID = serverID

		// Save updated config
		if err := cfg.Save(configPath); err != nil {
			logger.Error().Err(err).Msg("Failed to save updated config with server_id")
		} else {
			logger.Info().Str("server_id", serverID).Msg("Generated and saved server_id")
		}
	}

	logger.Info().Msgf("Installation: %s", config.GetInstallType())
	logger.Info().Msgf("Configuration loaded (%s)", configPath)
	logger.Info().Msgf("Server ID: %s", cfg.ServerID)

	// Get directories
	dataDir, err := config.GetDataDir()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get data directory")
	}

	logDir, err := config.GetLogDir()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get log directory")
	}

	logger.Info().Msgf("Data directory: %s", dataDir)
	logger.Info().Msgf("Logs directory: %s", logDir)

	// Ensure directories exist
	if err := config.EnsureDirectories(dataDir, cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create directories")
	}

	// Create process manager
	processManager, err := process.NewManager(dataDir, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create process manager")
	}

	// Create formation registry
	formationRegistry, err := registry.NewRegistry(
		cfg.Formations.PortRangeStart,
		cfg.Formations.PortRangeEnd,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create formation registry")
	}

	// Setup persistence
	registryPath, err := config.GetRegistryPath()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get registry path")
	}
	persistence := registry.NewPersistence(formationRegistry, registryPath, &logger)

	// Load existing registry
	if err := persistence.Load(); err != nil {
		logger.Warn().Err(err).Msg("Failed to load registry (starting fresh)")
	}

	// Enable auto-save
	persistence.EnableAutoSave()
	defer persistence.DisableAutoSave()

	// Registry initialized silently

	// Create auth middleware
	authMiddleware := auth.NewMiddleware(&cfg.Auth, &logger)

	if !cfg.Auth.Enabled {
		logger.Warn().Msg("Authentication disabled (development mode)")
	}

	// Create API server
	apiServer := api.NewServer(cfg, processManager, formationRegistry, authMiddleware, &logger, Version)

	// Restore previously running formations
	apiServer.RestoreFormations()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info().Msg("Shutdown signal received")
		cancel()
	}()

	// Start API server in goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error().Err(err).Msg("API server stopped with error")
			cancel()
		}
	}()

	logger.Info().Msgf("MUXI Server listening on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info().Msg("Shutting down gracefully...")

	// Stop API server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Failed to stop API server")
	}

	// Stop all processes
	if err := processManager.StopAll(); err != nil {
		logger.Error().Err(err).Msg("Failed to stop all processes")
	}

	// Final save of registry
	if err := persistence.Save(); err != nil {
		logger.Error().Err(err).Msg("Failed to save registry")
	}

	logger.Info().Msg("MUXI Server stopped")

	return nil
}
