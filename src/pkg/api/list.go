package api

import (
	"net/http"
)

// FormationListResponse represents the response for listing formations
type FormationListResponse struct {
	Formations []FormationInfo `json:"formations"`
	Count      int             `json:"count"`
}

// FormationInfo represents formation info for API responses
type FormationInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Port         int    `json:"port"`
	Status       string `json:"status"`
	PID          int    `json:"pid"`
	Uptime       string `json:"uptime"`
	RestartCount int    `json:"restart_count"`
	Healthy      bool   `json:"healthy"`
}

// HandleList handles GET /formations
func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Msg("Listing formations")

	// Get all formations from registry
	formations := s.registry.List()

	// Convert to response format
	formationInfos := make([]FormationInfo, 0, len(formations))
	for _, f := range formations {
		formationInfos = append(formationInfos, FormationInfo{
			ID:           f.ID,
			Name:         f.Name,
			Port:         f.Port,
			Status:       f.Status,
			PID:          f.ProcessID,
			Uptime:       f.ToProcessInfo().Uptime,
			RestartCount: f.RestartCount,
			Healthy:      f.Healthy,
		})
	}

	response := FormationListResponse{
		Formations: formationInfos,
		Count:      len(formationInfos),
	}

	RespondSuccess(w, response)
}
