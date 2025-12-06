package api

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

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

	// Check formation directory exists
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	if _, err := os.Stat(formationDir); os.IsNotExist(err) {
		return err
	}

	// Parse formation.yaml
	formationYAMLPath := filepath.Join(formationDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		return err
	}

	// Compute environment variables
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)

	// Build spawn config
	spawnConfig := process.SpawnConfig{
		ID:          formationID,
		WorkDir:     formationDir,
		Port:        port,
		Env:         formationConfig.GetEnvironmentVars(port, serverURL, bindHost),
		AutoRestart: s.config.Formations.AutoRestart,
		RuntimeType: "native",
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			return err
		}

		runtimeRegistryPath := filepath.Join(runtimesDir, "registry.json")
		runtimeRegistry := runtime.NewRegistry(runtimeRegistryPath)
		if err := runtimeRegistry.Load(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to load runtimes registry")
		}

		availableVersions := runtimeRegistry.List()
		resolver := runtime.NewResolver(availableVersions, runtimesDir)

		resolvedVersion, err := resolver.Resolve(formationConfig.MuxiRuntime)
		if err != nil {
			return err
		}

		sifPath := resolver.GetSIFPath(resolvedVersion)
		if _, err := os.Stat(sifPath); os.IsNotExist(err) {
			return err
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.utils.run_formation",
			"/formation/formation.yaml",
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
