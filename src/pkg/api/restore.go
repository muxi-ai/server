package api

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/runtime"
)

// RestoreFormations starts all formations that were previously registered
// This is called on server startup to restore state
func (s *Server) RestoreFormations() {
	formations := s.registry.List()
	if len(formations) == 0 {
		s.logger.Debug().Msg("No formations to restore")
		return
	}

	s.logger.Info().Int("count", len(formations)).Msg("Restoring formations from registry")

	for _, f := range formations {
		// Skip formations that were explicitly stopped
		if f.Status == "stopped" {
			s.logger.Info().
				Str("id", f.ID).
				Msg("Skipping stopped formation")
			continue
		}

		if err := s.restoreFormation(f.ID, f.Port); err != nil {
			s.logger.Error().
				Err(err).
				Str("id", f.ID).
				Msg("Failed to restore formation")
		} else {
			s.logger.Info().
				Str("id", f.ID).
				Int("port", f.Port).
				Msg("Formation restored")
		}
	}
}

// restoreFormation starts a single formation
func (s *Server) restoreFormation(formationID string, port int) error {
	// Get MUXI directory
	muxiDir, err := getMuxiDir()
	if err != nil {
		return err
	}

	// Check formation directory exists (new structure: formations/{id}/current/)
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)
	currentDir := filepath.Join(formationBaseDir, "current")
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		return fmt.Errorf("formation current directory not found: %s", currentDir)
	}

	// Find and parse formation config from current directory (formation.afs/yaml/yml)
	formationConfigPath, err := formation.FindFormationFile(currentDir)
	if err != nil {
		return fmt.Errorf("failed to find formation config: %w", err)
	}
	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read formation config: %w", err)
	}

	// Compute environment variables
	bindHost := getBindHost(s.config.Formations.BindHost)
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)

	// Build spawn config
	envVars := formationConfig.GetEnvironmentVars(port, serverURL, bindHost)
	if s.rceManager != nil {
		s.rceManager.InjectEnvVars(envVars)
	}
	spawnConfig := process.SpawnConfig{
		ID:          formationID,
		WorkDir:     currentDir,
		Port:        port,
		Env:         envVars,
		AutoRestart: s.config.Formations.AutoRestart,
		RuntimeType: "native",
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			return err
		}

		// Parse "<version>[:<variant>]" — restore path runs during server
		// startup for previously-deployed formations, so this must honor
		// variants already persisted in their formation configs.
		parsedVersion, variant, err := runtime.ParseMuxiRuntime(formationConfig.MuxiRuntime)
		if err != nil {
			return fmt.Errorf("invalid muxi_runtime in restored formation: %w", err)
		}

		// Use downloader to resolve "latest" from GitHub and ensure SIF exists
		downloader := runtime.NewDownloader(
			s.config.Runtime.SIFBaseURL,
			s.config.Runtime.RuntimeRunnerImage,
			runtimesDir,
			s.logger,
		)

		sifPath, _, _, err := downloader.EnsureSIFForVariant(parsedVersion, variant)
		if err != nil {
			return fmt.Errorf("failed to ensure runtime SIF: %w", err)
		}

		cacheDir, err := getCacheDir()
		if err != nil {
			return fmt.Errorf("failed to resolve cache directory: %w", err)
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.HFCacheDir = cacheDir
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.runtime.utils.run_formation",
			"/formation",
			"--port", fmt.Sprintf("%d", port),
			"--host", bindHost,
		}
	}

	// Get old restart count from registry before starting
	oldRestartCount := 0
	if f, err := s.registry.Get(formationID); err == nil {
		oldRestartCount = f.RestartCount
	}

	// Start the process
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		return err
	}

	// Preserve restart count from before server restart
	proc.RestartCount = oldRestartCount

	// Update registry with new process info
	f, err := s.registry.Get(formationID)
	if err == nil {
		f.ProcessID = proc.PID
		f.Status = "starting"
		f.RestartCount = oldRestartCount // Preserve across server restart
	}

	return nil
}
