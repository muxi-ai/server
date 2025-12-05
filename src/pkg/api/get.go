package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/rs/zerolog/log"
)

// formatFormationURL creates the proxy URL for a formation
func formatFormationURL(serverPort int, formationID string) string {
	return fmt.Sprintf("http://localhost:%d/api/%s", serverPort, formationID)
}

// HandleGet handles GET /formations/{id}
// Returns detailed information about a specific formation
func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	log.Debug().
		Str("formation_id", formationID).
		Msg("Getting formation details")

	// Get formation from registry
	f, err := s.registry.Get(formationID)
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
		f.UpdateFromProcess(proc)
	}

	// Calculate uptime in seconds
	var uptimeSeconds int64
	if !f.StartedAt.IsZero() && f.Status == "running" {
		uptimeSeconds = int64(time.Since(f.StartedAt).Seconds())
	}

	// Load version info if available
	var versionInfo *VersionInfo
	formationDir := filepath.Join(s.config.Formations.FormationsDir, f.ID)
	if history, err := formation.LoadVersionHistory(formationDir); err == nil && history.Current != nil {
		versionInfo = &VersionInfo{
			Current: history.Current.BundleHash,
		}
		if history.Previous != nil {
			versionInfo.Previous = history.Previous.BundleHash
		}
	}

	// Build detailed response aligned with API spec
	response := map[string]interface{}{
		"id":            f.ID,
		"name":          f.Name,
		"status":        f.Status,
		"port":          f.Port,
		"pid":           f.ProcessID,
		"restart_count": f.RestartCount,
		"uptime":        uptimeSeconds,
		"created_at":    f.DeployedAt,
		"deployed_at":   f.DeployedAt,
		"updated_at":    f.StartedAt, // Use StartedAt as last update time
	}

	// Health is null when starting (unknown during startup)
	if f.Status != "starting" {
		response["healthy"] = f.Healthy
	} else {
		response["healthy"] = nil
	}

	// Add version info if available
	if versionInfo != nil {
		response["version"] = versionInfo
	}

	// Add health check info if available
	if !f.LastHealthCheck.IsZero() {
		response["last_health_check"] = f.LastHealthCheck
	}

	log.Info().
		Str("formation_id", formationID).
		Str("status", f.Status).
		Msg("Formation details retrieved")

	RespondSuccess(w, response)
}
