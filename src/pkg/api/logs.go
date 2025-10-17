package api

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleLogs handles GET /formations/{id}/logs
// Returns recent log lines from a formation
func (s *Server) HandleLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	// Parse query parameters
	linesParam := r.URL.Query().Get("lines")
	lines := 100 // default
	if linesParam != "" {
		if parsed, err := strconv.Atoi(linesParam); err == nil && parsed > 0 {
			lines = parsed
			if lines > 10000 {
				lines = 10000 // cap at 10k lines
			}
		}
	}

	log.Debug().
		Str("formation_id", formationID).
		Int("lines", lines).
		Msg("Getting formation logs")

	// Check if formation exists
	_, err := s.registry.Get(formationID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Get log file path
	// For now, formations log to their stdout which goes to the process manager
	// In future, this will read from ~/.muxi/server/logs/{formation_id}.log
	logPath := filepath.Join(s.config.Formations.LogsDir, formationID+".log")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		log.Warn().
			Str("formation_id", formationID).
			Str("log_path", logPath).
			Msg("Log file not found")

		// Return empty logs instead of error (formation might be new)
		RespondSuccess(w, map[string]interface{}{
			"id":          formationID,
			"logs":        []string{},
			"lines":       0,
			"total_lines": 0,
			"message":     "No logs available yet",
		})
		return
	}

	// Read log file
	logLines, totalLines, err := readLastNLines(logPath, lines)
	if err != nil {
		log.Error().
			Err(err).
			Str("formation_id", formationID).
			Str("log_path", logPath).
			Msg("Failed to read log file")
		RespondError(w, http.StatusInternalServerError, "Failed to read logs")
		return
	}

	log.Info().
		Str("formation_id", formationID).
		Int("returned_lines", len(logLines)).
		Int("total_lines", totalLines).
		Msg("Logs retrieved")

	RespondSuccess(w, map[string]interface{}{
		"id":          formationID,
		"logs":        logLines,
		"lines":       len(logLines),
		"total_lines": totalLines,
	})
}

// readLastNLines reads the last N lines from a file
func readLastNLines(filePath string, n int) ([]string, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Simple approach: read all lines, keep last N
	// For large files, we should use a more efficient approach (seek from end)
	// But this works for now
	var allLines []string
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size

	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("error reading log file: %w", err)
	}

	totalLines := len(allLines)

	// Return last N lines
	if totalLines <= n {
		return allLines, totalLines, nil
	}

	return allLines[totalLines-n:], totalLines, nil
}
