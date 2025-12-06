package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/runtime"
)

// HandleRollback handles POST /rpc/formations/{id}/rollback
// Rolls back a formation to its previous version
func (s *Server) HandleRollback(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	s.logger.Info().Str("id", formationID).Msg("Rolling back formation")

	// Get existing formation
	existingFormation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Get formation base directory
	muxiDir := s.config.Formations.FormationsDir
	if muxiDir == "" {
		muxiDir = "formations"
	}
	formationBaseDir := filepath.Join(muxiDir, formationID)

	// Load version history
	history, err := formation.LoadVersionHistory(formationBaseDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load version history")
		RespondError(w, http.StatusInternalServerError, "Failed to load version history")
		return
	}

	// Check if previous version exists
	if !history.HasPreviousVersion() {
		s.logger.Warn().Str("id", formationID).Msg("No previous version to rollback to")
		RespondError(w, http.StatusBadRequest, "No previous version available for rollback")
		return
	}

	// Stop current formation
	if err := s.processManager.Stop(formationID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to stop formation (continuing anyway)")
	}

	// Swap: current <-> previous
	currentDir := filepath.Join(formationBaseDir, "current")
	previousDir := filepath.Join(formationBaseDir, "previous")
	tempDir := filepath.Join(formationBaseDir, "temp")

	// Ensure both directories exist
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		s.logger.Error().Msg("Current directory does not exist")
		RespondError(w, http.StatusInternalServerError, "Current version directory not found")
		return
	}
	if _, err := os.Stat(previousDir); os.IsNotExist(err) {
		s.logger.Error().Msg("Previous directory does not exist")
		RespondError(w, http.StatusBadRequest, "Previous version directory not found")
		return
	}

	// Three-way swap
	if err := os.Rename(currentDir, tempDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move current to temp")
		RespondError(w, http.StatusInternalServerError, "Failed to perform rollback (step 1)")
		return
	}

	if err := os.Rename(previousDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move previous to current")
		// Try to restore
		os.Rename(tempDir, currentDir)
		RespondError(w, http.StatusInternalServerError, "Failed to perform rollback (step 2)")
		return
	}

	if err := os.Rename(tempDir, previousDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move temp to previous")
		// Don't fail - the important swap is done
	}

	// Swap version history
	tempVersion := history.Current
	tempVersionNum := history.CurrentVersion

	history.Current = history.Previous
	history.CurrentVersion = history.PreviousVersion

	history.Previous = tempVersion
	history.PreviousVersion = tempVersionNum

	// Update backup paths
	if history.Current != nil {
		history.Current.BackupPath = "current"
	}
	if history.Previous != nil {
		history.Previous.BackupPath = "previous"
	}

	if err := history.Save(formationBaseDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save version history")
		// Don't fail the rollback for this
	}

	// Parse formation.yaml from rolled-back version
	formationYAMLPath := filepath.Join(currentDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation.yaml")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to parse formation.yaml: %v", err))
		return
	}

	// Get environment variables
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(existingFormation.Port, serverURL, bindHost)

	// Build spawn config
	spawnConfig := process.SpawnConfig{
		ID:          formationID,
		Name:        formationConfig.Name,
		Command:     formationConfig.GetDefaultCommand(),
		Args:        formationConfig.GetDefaultArgs(),
		Port:        existingFormation.Port,
		WorkDir:     currentDir,
		Env:         envVars,
		AutoRestart: s.config.Formations.AutoRestart,
		RuntimeType: "native",
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		muxiDir, err := getMuxiDir()
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
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
			RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to resolve runtime: %v", err))
			return
		}

		// Create downloader for auto-download
		downloader := runtime.NewDownloader(
			s.config.Runtime.SIFBaseURL,
			s.config.Runtime.RuntimeRunnerImage,
			runtimesDir,
			s.logger,
		)

		var sifPath string
		if s.config.Runtime.AutoDownload {
			sifPath, _, err = downloader.EnsureSIF(resolvedVersion)
			if err != nil {
				RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download runtime: %v", err))
				return
			}
			if _, err := downloader.EnsureRuntimeRunner(); err != nil {
				RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to pull runtime-runner: %v", err))
				return
			}
		} else {
			sifPath = resolver.GetSIFPath(resolvedVersion)
			if _, err := os.Stat(sifPath); os.IsNotExist(err) {
				RespondError(w, http.StatusNotFound, fmt.Sprintf("Runtime %s not found", resolvedVersion))
				return
			}
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.utils.run_formation",
			"/formation/formation.yaml",
			"--port", fmt.Sprintf("%d", existingFormation.Port),
			"--host", bindHost,
		}
	}

	// Restart formation with previous version
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to restart formation")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to restart formation: %v", err))
		return
	}

	s.logger.Info().
		Str("id", formationID).
		Int("version", history.CurrentVersion).
		Int("pid", proc.PID).
		Msg("Formation rolled back successfully")

	response := map[string]interface{}{
		"id":               formationID,
		"status":           "running",
		"version":          history.CurrentVersion,
		"previous_version": history.PreviousVersion,
		"pid":              proc.PID,
		"message":          "Rolled back to previous version",
	}

	RespondSuccess(w, response)
}
