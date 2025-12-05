package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// HandleLogs handles GET /formations/{id}/logs
// Returns recent log lines from a formation
// Parameters:
//   - lines: number of lines to return (default 100, max 10000)
//   - stream: which log stream (stdout, stderr, all) - default all
//   - follow: true to stream new logs via SSE (like tail -f)
func (s *Server) HandleLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	// Parse query parameters
	linesParam := r.URL.Query().Get("lines")
	streamParam := r.URL.Query().Get("stream") // stdout, stderr, all
	followParam := r.URL.Query().Get("follow") // true for SSE streaming
	wantsFollow := followParam == "true" || followParam == "1"

	if streamParam == "" {
		streamParam = "all"
	}

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
		Str("stream", streamParam).
		Bool("follow", wantsFollow).
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

	// Determine log file paths based on stream parameter
	// LogsDir is relative to ~/.muxi/server/, so we need to construct the full path
	logsDir := s.config.Formations.LogsDir
	if !filepath.IsAbs(logsDir) {
		// Get base dir from FormationsDir (e.g., ~/.muxi/server/formations -> ~/.muxi/server)
		baseDir := filepath.Dir(s.config.Formations.FormationsDir)
		if baseDir == "." {
			baseDir = filepath.Join(os.Getenv("HOME"), ".muxi", "server")
		}
		logsDir = filepath.Join(baseDir, logsDir)
	}
	stdoutPath := filepath.Join(logsDir, formationID+"-out.log")
	stderrPath := filepath.Join(logsDir, formationID+"-err.log")

	// Handle SSE follow mode
	if wantsFollow {
		logPath := stdoutPath // Default to stdout for following
		if streamParam == "stderr" {
			logPath = stderrPath
		}
		s.streamLogsSSE(w, r, formationID, logPath)
		return
	}

	// Read logs based on stream parameter
	var stdoutLines, stderrLines []string
	var stdoutTotal, stderrTotal int

	if streamParam == "stdout" || streamParam == "all" {
		if _, err := os.Stat(stdoutPath); err == nil {
			stdoutLines, stdoutTotal, _ = readLastNLines(stdoutPath, lines)
		}
	}

	if streamParam == "stderr" || streamParam == "all" {
		if _, err := os.Stat(stderrPath); err == nil {
			stderrLines, stderrTotal, _ = readLastNLines(stderrPath, lines)
		}
	}

	// Build response matching API spec
	logsData := map[string]interface{}{}
	if streamParam == "stdout" || streamParam == "all" {
		logsData["stdout"] = stdoutLines
	}
	if streamParam == "stderr" || streamParam == "all" {
		logsData["stderr"] = stderrLines
	}

	log.Info().
		Str("formation_id", formationID).
		Int("stdout_lines", len(stdoutLines)).
		Int("stderr_lines", len(stderrLines)).
		Msg("Logs retrieved")

	RespondSuccess(w, map[string]interface{}{
		"formation_id": formationID,
		"logs":         logsData,
		"total_lines": map[string]int{
			"stdout": stdoutTotal,
			"stderr": stderrTotal,
		},
	})
}

// streamLogsSSE streams log lines via Server-Sent Events
func (s *Server) streamLogsSSE(w http.ResponseWriter, r *http.Request, formationID, logPath string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		RespondError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	log.Info().
		Str("formation_id", formationID).
		Str("log_path", logPath).
		Msg("Starting log stream")

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"formation_id\":\"%s\"}\n\n", formationID)
	flusher.Flush()

	// Track file position
	var lastPos int64 = 0
	var file *os.File
	var err error

	// If file exists, seek to end (only stream new lines)
	if _, statErr := os.Stat(logPath); statErr == nil {
		file, err = os.Open(logPath)
		if err == nil {
			lastPos, _ = file.Seek(0, io.SeekEnd)
			file.Close()
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Str("formation_id", formationID).Msg("Log stream client disconnected")
			return
		case <-ticker.C:
			// Check if file exists
			fileInfo, statErr := os.Stat(logPath)
			if statErr != nil {
				continue // File doesn't exist yet, keep waiting
			}

			currentSize := fileInfo.Size()
			if currentSize <= lastPos {
				// File was truncated or no new data
				if currentSize < lastPos {
					lastPos = 0 // Reset position if file was truncated
				}
				continue
			}

			// Read new content
			file, err = os.Open(logPath)
			if err != nil {
				continue
			}

			file.Seek(lastPos, io.SeekStart)
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				line := scanner.Text()
				// Send log line as SSE event
				data, _ := json.Marshal(map[string]string{
					"line":         line,
					"formation_id": formationID,
				})
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", data)
				flusher.Flush()
			}

			lastPos, _ = file.Seek(0, io.SeekCurrent)
			file.Close()
		}
	}
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
