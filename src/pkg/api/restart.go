package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleRestart handles POST /formations/{id}/restart
// Restarts a formation
func (s *Server) HandleRestart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Info().
		Str("formation_id", formationID).
		Msg("Restarting formation")

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

	// Restart the process
	if _, err := s.processManager.Restart(formationID); err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to restart formation")
		RespondError(w, http.StatusInternalServerError, "Failed to restart formation")
		return
	}

	// Get updated process info
	proc, err := s.processManager.Get(formationID)
	restartCount := 0
	if err == nil && proc != nil {
		restartCount = proc.RestartCount
		formation.UpdateFromProcess(proc)
	}

	log.Info().
		Str("formation_id", formationID).
		Int("restart_count", restartCount).
		Msg("Formation restarted successfully")

	RespondSuccess(w, map[string]interface{}{
		"id":            formationID,
		"status":        formation.Status,
		"message":       "Formation restarting",
		"restart_count": restartCount,
	})
}
