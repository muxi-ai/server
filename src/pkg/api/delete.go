package api

import (
	"net/http"

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
		log.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
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

	log.Info().
		Str("formation_id", formationID).
		Int("port", formation.Port).
		Msg("Formation deleted successfully")

	RespondSuccess(w, map[string]interface{}{
		"id":      formationID,
		"message": "Formation deleted successfully",
	})
}
