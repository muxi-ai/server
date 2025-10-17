package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleGet handles GET /formations/{id}
// Returns detailed information about a specific formation
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Debug().
		Str("formation_id", formationID).
		Msg("Getting formation details")

	// Get formation from registry
	formation, err := s.registry.Get(formationID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Get process info for additional details
	proc, err := s.processManager.Get(formationID)
	if err != nil {
		log.Warn().
			Str("formation_id", formationID).
			Msg("Formation in registry but process not found")
		// Formation exists in registry but process is gone
		// Update formation from what we know
	} else {
		// Update formation with latest process info
		formation.UpdateFromProcess(proc)
	}

	// Build detailed response
	response := map[string]interface{}{
		"id":         formation.ID,
		"name":       formation.Name,
		"status":     formation.Status,
		"port":       formation.Port,
		"pid":        formation.ProcessID,
		"url":        formatFormationURL(s.config.Server.Port, formation.ID),
		"command":    formation.Command,
		"args":       formation.Args,
		"healthy":    formation.Healthy,
		"created_at": formation.DeployedAt,
		"started_at": formation.StartedAt,
		"restart_count": formation.RestartCount,
	}

	// Add health check info if available
	if !formation.LastHealthCheck.IsZero() {
		response["last_health_check"] = formation.LastHealthCheck
	}

	log.Info().
		Str("formation_id", formationID).
		Str("status", formation.Status).
		Msg("Formation details retrieved")

	RespondSuccess(w, response)
}

// formatFormationURL creates the proxy URL for a formation
func formatFormationURL(serverPort int, formationID string) string {
	return fmt.Sprintf("http://localhost:%d/v1/%s", serverPort, formationID)
}
