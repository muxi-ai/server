package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog/log"
)

// HandleRestart handles POST /formations/{id}/restart
// Restarts a formation by stopping current process and starting fresh
// Re-reads formation config to ensure runtime type is correct
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

	// Get old restart count before stopping
	oldRestartCount := formation.RestartCount
	proc, _ := s.processManager.Get(formationID)
	if proc != nil {
		oldRestartCount = proc.RestartCount
	}

	// Stop the current process
	if err := s.processManager.Stop(formationID); err != nil {
		log.Warn().Err(err).Str("id", formationID).Msg("Failed to stop process (may already be stopped)")
	}

	// Restore using full formation config (handles runtime type correctly)
	if err := s.restoreFormation(formationID, formation.Port); err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Msg("Failed to restart formation")
		RespondError(w, http.StatusInternalServerError, "Failed to restart formation: "+err.Error())
		return
	}

	// Preserve and increment restart count
	newRestartCount := oldRestartCount + 1
	if newProc, err := s.processManager.Get(formationID); err == nil {
		newProc.RestartCount = newRestartCount
	}
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.RestartCount = newRestartCount
	})

	log.Info().
		Str("formation_id", formationID).
		Int("restart_count", newRestartCount).
		Msg("Formation restarted successfully")

	RespondSuccess(w, map[string]interface{}{
		"id":            formationID,
		"status":        "starting",
		"message":       "Formation restarting",
		"restart_count": newRestartCount,
	})
}
