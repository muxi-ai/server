package api

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// HandleServerLogs handles GET /rpc/server/logs
// Returns server audit logs
func (s *Server) HandleServerLogs(w http.ResponseWriter, r *http.Request) {
	// Get lines parameter (default: 100, max: 10000)
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if n, err := strconv.Atoi(linesStr); err == nil && n > 0 {
			lines = n
			if lines > 10000 {
				lines = 10000
			}
		}
	}

	// Get audit log path from config
	auditLogPath := s.config.Logging.AuditLog
	if auditLogPath == "" {
		auditLogPath = "logs/audit.log"
	}

	// If relative path, make it absolute from formations dir
	if !filepath.IsAbs(auditLogPath) {
		baseDir := s.config.Formations.FormationsDir
		if baseDir == "" {
			baseDir = "formations"
		}
		auditLogPath = filepath.Join(filepath.Dir(baseDir), auditLogPath)
	}

	// Check if log file exists
	if _, err := os.Stat(auditLogPath); os.IsNotExist(err) {
		// Return empty logs if file doesn't exist yet
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
		return
	}

	// Read log file
	file, err := os.Open(auditLogPath)
	if err != nil {
		s.logger.Error().Err(err).Str("path", auditLogPath).Msg("Failed to open audit log")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open audit log: %v", err))
		return
	}
	defer file.Close()

	// Read last N lines
	var logLines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
		// Keep only last N lines (circular buffer)
		if len(logLines) > lines {
			logLines = logLines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to read audit log")
		RespondError(w, http.StatusInternalServerError, "Failed to read audit log")
		return
	}

	// Return as plain text
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for _, line := range logLines {
		w.Write([]byte(line + "\n"))
	}
}
