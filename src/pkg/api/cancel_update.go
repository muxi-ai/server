package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

// HandleCancelUpdate cancels an in-progress update by stopping the staging formation
// POST /rpc/formations/{id}/cancel-update
func (s *Server) HandleCancelUpdate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	s.logger.Info().
		Str("id", formationID).
		Msg("Cancelling update for formation")

	// Get the formation from registry
	formation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Check if there's a staging port (indicates update in progress)
	stagingPort := s.registry.GetStagingPort(formationID)
	if stagingPort == 0 {
		s.logger.Info().
			Str("id", formationID).
			Msg("No update in progress to cancel")
		RespondError(w, http.StatusBadRequest, "No update in progress for this formation")
		return
	}

	// Stop the staging process
	stagingProcessID := formationID + "-staging"
	if err := s.processManager.ForceKill(stagingProcessID); err != nil {
		s.logger.Warn().
			Err(err).
			Str("staging_id", stagingProcessID).
			Msg("Failed to kill staging process (may have already stopped)")
	} else {
		s.logger.Info().
			Str("staging_id", stagingProcessID).
			Msg("Staging process killed")
	}

	// Release the staging port
	s.registry.ReleasePort(stagingPort)

	// Clear staging port from formation
	if err := s.registry.ClearStagingPort(formationID); err != nil {
		s.logger.Warn().
			Err(err).
			Str("id", formationID).
			Msg("Failed to clear staging port from registry")
	}

	// Clean up staging directory if it exists
	muxiDir, _ := getMuxiDir()
	stagingDir := filepath.Join(muxiDir, "formations", formationID, "staging")
	if _, err := os.Stat(stagingDir); err == nil {
		if err := os.RemoveAll(stagingDir); err != nil {
			s.logger.Warn().
				Err(err).
				Str("path", stagingDir).
				Msg("Failed to remove staging directory")
		} else {
			s.logger.Info().
				Str("path", stagingDir).
				Msg("Staging directory removed")
		}
	}

	s.logger.Info().
		Str("id", formationID).
		Int("staging_port", stagingPort).
		Msg("Update cancelled successfully")

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Update cancelled successfully",
		"formation_id":  formationID,
		"staging_port":  stagingPort,
		"status":        formation.Status,
		"current_port":  formation.Port,
	})
}
