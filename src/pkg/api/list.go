package api

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/muxi-ai/server/pkg/formation"
)

// FormationListResponse represents the response for listing formations
type FormationListResponse struct {
	Formations []FormationInfo `json:"formations"`
	Total      int             `json:"total"`
}

// VersionInfo represents version information for API responses
type VersionInfo struct {
	Current  string `json:"current,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// FormationInfo represents formation info for API responses
type FormationInfo struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Port         int          `json:"port"`
	Status       string       `json:"status"`
	PID          int          `json:"pid"`
	Uptime       int64        `json:"uptime"`
	RestartCount int          `json:"restart_count"`
	Healthy      *bool        `json:"healthy"` // null when status is "starting"
	Version      *VersionInfo `json:"version,omitempty"`
}

// HandleList handles GET /formations
func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Msg("Listing formations")

	// Get all formations from registry
	formations := s.registry.List()

	// Convert to response format
	formationInfos := make([]FormationInfo, 0, len(formations))
	for _, f := range formations {
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

		// Only include health status when not starting (unknown during startup)
		var healthy *bool
		if f.Status != "starting" {
			healthy = &f.Healthy
		}

		formationInfos = append(formationInfos, FormationInfo{
			ID:           f.ID,
			Name:         f.Name,
			Port:         f.Port,
			Status:       f.Status,
			PID:          f.ProcessID,
			Uptime:       uptimeSeconds,
			RestartCount: f.RestartCount,
			Healthy:      healthy,
			Version:      versionInfo,
		})
	}

	response := FormationListResponse{
		Formations: formationInfos,
		Total:      len(formationInfos),
	}

	RespondSuccess(w, response)
}
