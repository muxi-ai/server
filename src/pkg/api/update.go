package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
)

// HandleUpdate handles PUT /rpc/formations/{id}
// Updates a formation to a new version, keeping previous version as backup
func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	s.logger.Info().Str("id", formationID).Msg("Updating formation")

	// Get existing formation
	existingFormation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Create temporary file for uploaded bundle
	tmpFile, err := os.CreateTemp("", "formation-update-*.tar.gz")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create temp file")
		RespondError(w, http.StatusInternalServerError, "Failed to create temp file")
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy uploaded data to temp file
	bundleData, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to read bundle")
		RespondError(w, http.StatusInternalServerError, "Failed to read bundle")
		return
	}

	if _, err := tmpFile.Write(bundleData); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save bundle")
		RespondError(w, http.StatusInternalServerError, "Failed to save bundle")
		return
	}
	tmpFile.Close()

	// Get formation base directory
	muxiDir, err := s.config.Formations.FormationsDir, nil
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

	// Stop current formation
	if err := s.processManager.Stop(formationID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to stop formation (continuing anyway)")
	}

	// Backup current version
	currentDir := filepath.Join(formationBaseDir, "current")
	previousDir := filepath.Join(formationBaseDir, "previous")

	// Remove old previous if exists
	if _, err := os.Stat(previousDir); err == nil {
		if err := os.RemoveAll(previousDir); err != nil {
			s.logger.Error().Err(err).Msg("Failed to remove old previous")
			RespondError(w, http.StatusInternalServerError, "Failed to remove old backup")
			return
		}
	}

	// Move current → previous
	if _, err := os.Stat(currentDir); err == nil {
		if err := os.Rename(currentDir, previousDir); err != nil {
			s.logger.Error().Err(err).Msg("Failed to backup current version")
			RespondError(w, http.StatusInternalServerError, "Failed to backup current version")
			return
		}
	}

	// Extract new version to current/
	extractDir, err := os.MkdirTemp("", "formation-extract-*")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create extraction directory")
		RespondError(w, http.StatusInternalServerError, "Failed to create extraction directory")
		return
	}
	defer os.RemoveAll(extractDir)

	newFormationDir, err := formation.ExtractBundle(tmpFile.Name(), extractDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to extract bundle")
		// Restore previous version
		if _, statErr := os.Stat(previousDir); statErr == nil {
			os.Rename(previousDir, currentDir)
		}
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to extract bundle: %v", err))
		return
	}

	// Move extracted formation to current/
	if err := os.Rename(newFormationDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move new version to current")
		// Restore previous version
		if _, statErr := os.Stat(previousDir); statErr == nil {
			os.Rename(previousDir, currentDir)
		}
		RespondError(w, http.StatusInternalServerError, "Failed to deploy new version")
		return
	}

	// Update version history
	bundleHash := formation.ComputeBundleHash(bundleData)
	newVersion := history.CurrentVersion + 1

	history.Previous = history.Current
	history.PreviousVersion = history.CurrentVersion
	history.Current = &formation.Version{
		Version:    newVersion,
		DeployedAt: time.Now(),
		BundleHash: bundleHash,
		BackupPath: "current",
	}
	if history.Previous != nil {
		history.Previous.BackupPath = "previous"
	}
	history.CurrentVersion = newVersion

	if err := history.Save(formationBaseDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save version history")
		// Don't fail the deployment for this
	}

	// Parse new formation.yaml
	formationYAMLPath := filepath.Join(currentDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation.yaml")
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse formation.yaml: %v", err))
		return
	}

	// Inject metadata
	serverID, _ := formation.GenerateServerID()
	if err := formation.InjectMetadata(currentDir, serverID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to inject metadata (continuing anyway)")
	}

	// Get environment variables
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(existingFormation.Port, serverURL, s.config.Formations.BindHost)

	// Restart formation with new version
	proc, err := s.processManager.Start(process.SpawnConfig{
		ID:          formationID,
		Name:        formationConfig.Name,
		Command:     formationConfig.GetDefaultCommand(),
		Args:        formationConfig.GetDefaultArgs(),
		Port:        existingFormation.Port,
		WorkDir:     currentDir,
		Env:         envVars,
		AutoRestart: s.config.Formations.AutoRestart,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to restart formation")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to restart formation: %v", err))
		return
	}

	s.logger.Info().
		Str("id", formationID).
		Int("version", newVersion).
		Int("pid", proc.PID).
		Msg("Formation updated successfully")

	response := map[string]interface{}{
		"id":               formationID,
		"status":           "running",
		"version":          newVersion,
		"previous_version": history.PreviousVersion,
		"pid":              proc.PID,
		"message":          "Formation updated successfully",
	}

	RespondSuccess(w, response)
}
