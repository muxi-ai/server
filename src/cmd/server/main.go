package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

// cmdStart starts the MUXI Server
func cmdStart() error {
	// Setup logging
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Logger()
	log.Logger = logger

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

	logger.Info().Msgf("Configuration loaded (%s)", configPath)
	logger.Info().Msgf("Server ID: %s", cfg.ServerID)

	// Get MUXI directory
	muxiDir, err := config.GetMuxiDir()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get MUXI directory")
	}

	// Ensure directories exist
	if err := config.EnsureDirectories(muxiDir, cfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create directories")
	}

	// Directory initialized silently

	// Create process manager
	processManager, err := process.NewManager(muxiDir, &logger)
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
	registryPath := filepath.Join(muxiDir, "registry.json")
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
	apiServer := api.NewServer(cfg, processManager, formationRegistry, authMiddleware, &logger)

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
