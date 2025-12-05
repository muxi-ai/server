package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleDelete handles DELETE /formations/{id}
// Stops and removes a formation
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Info().
		Str("formation_id", formationID).
		Msg("Deleting formation")

	// Check if formation exists
	formation, err := s.registry.Get(formationID)
	if err != nil {
		// DELETE is idempotent - not finding a resource is not a warning
		log.Debug().
			Str("formation_id", formationID).
			Msg("Formation not found (already deleted or never existed)")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Stop the process
	if err := s.processManager.Stop(formationID); err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to stop formation process")
		// Continue with deletion even if stop fails
	}

	// Remove from registry (this also releases the port)
	if err := s.registry.Unregister(formationID); err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to remove formation from registry")
		RespondError(w, http.StatusInternalServerError, "Failed to delete formation")
		return
	}

	// Remove formation directory
	muxiDir, err := getMuxiDir()
	if err == nil {
		formationDir := filepath.Join(muxiDir, "formations", formationID)
		if err := os.RemoveAll(formationDir); err != nil {
			log.Warn().
				Err(err).
				Str("formation_id", formationID).
				Str("dir", formationDir).
				Msg("Failed to remove formation directory (continuing anyway)")
			// Don't fail - formation is already unregistered
		} else {
			log.Debug().
				Str("formation_id", formationID).
				Str("dir", formationDir).
				Msg("Removed formation directory")
		}
	}

	log.Info().
		Str("formation_id", formationID).
		Int("port", formation.Port).
		Msg("Formation deleted successfully")

	RespondSuccess(w, map[string]interface{}{
		"id":      formationID,
		"message": "Formation deleted successfully",
	})
}
