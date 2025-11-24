package api

import (
	"net/http"
	"runtime"
	"time"
)

var serverStartTime = time.Now()

// HandleServerStatus handles GET /rpc/server/status
// Returns server information and statistics
func (s *Server) HandleServerStatus(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(serverStartTime)

	// Get formation counts
	formations := s.registry.List()

	runningCount := 0
	stoppedCount := 0
	crashedCount := 0

	for _, f := range formations {
		switch f.Status {
		case "running":
			runningCount++
		case "stopped":
			stoppedCount++
		case "crashed":
			crashedCount++
		}
	}

	// Get port pool status
	available, allocated, total := s.registry.PortPoolStatus()

	status := map[string]interface{}{
		"server": map[string]interface{}{
			"id":      s.config.ServerID,
			"version": s.version, // Version from build-time injection
			"uptime":  int(uptime.Seconds()),
		},
		"formations": map[string]interface{}{
			"total":   len(formations),
			"running": runningCount,
			"stopped": stoppedCount,
			"crashed": crashedCount,
		},
		"port_pool": map[string]interface{}{
			"total":     total,
			"available": available,
			"allocated": allocated,
		},
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"go_version": runtime.Version(),
		},
	}

	RespondSuccess(w, status)
}
