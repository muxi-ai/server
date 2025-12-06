package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog/log"
)

// HandleStop handles POST /formations/{id}/stop
// Stops a running formation without deleting it
func (s *Server) HandleStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Info().
		Str("formation_id", formationID).
		Msg("Stopping formation")

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

	// Check if already stopped
	if formation.Status == "stopped" {
		log.Warn().
			Str("formation_id", formationID).
			Msg("Formation already stopped")
		RespondError(w, http.StatusConflict, "Formation is already stopped")
		return
	}

	// Stop the process
	if err := s.processManager.Stop(formationID); err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to stop formation")
		RespondError(w, http.StatusInternalServerError, "Failed to stop formation")
		return
	}

	// Update registry status
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.Status = "stopped"
		f.Healthy = false
		f.ProcessID = 0
	})

	log.Info().
		Str("formation_id", formationID).
		Msg("Formation stopped successfully")

	RespondSuccess(w, map[string]interface{}{
		"id":      formationID,
		"status":  "stopped",
		"message": "Formation stopped successfully",
	})
}
