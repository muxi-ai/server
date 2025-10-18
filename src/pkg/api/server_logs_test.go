package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleServerLogs(t *testing.T) {
	t.Run("server logs with no log file", func(t *testing.T) {
		server := createTestServer(t)

		// Set up a temp directory for logs
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		server.config.Logging.AuditLog = filepath.Join(logsDir, "audit.log")

		req := httptest.NewRequest("GET", "/rpc/server/logs", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Should return empty response
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "" {
			t.Errorf("Body = %q, want empty string for nonexistent log", string(body))
		}

		// Content type should be text/plain
		if resp.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", resp.Header.Get("Content-Type"))
		}
	})

	t.Run("server logs with log file", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		// Write test log entries
		logContent := `{"time":"2025-01-01T00:00:00Z","level":"info","method":"GET","path":"/health","status":200}
{"time":"2025-01-01T00:01:00Z","level":"info","method":"POST","path":"/rpc/formations/deploy","status":201}
{"time":"2025-01-01T00:02:00Z","level":"info","method":"GET","path":"/rpc/formations","status":200}
`
		os.WriteFile(logPath, []byte(logContent), 0644)

		req := httptest.NewRequest("GET", "/rpc/server/logs", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Should contain all 3 log lines
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
		if len(lines) != 3 {
			t.Errorf("Got %d log lines, want 3", len(lines))
		}

		// Should be valid JSON lines
		if !strings.Contains(bodyStr, "GET") || !strings.Contains(bodyStr, "POST") {
			t.Error("Log content does not contain expected methods")
		}
	})

	t.Run("server logs with lines parameter", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		// Write 5 log entries
		logContent := `line 1
line 2
line 3
line 4
line 5
`
		os.WriteFile(logPath, []byte(logContent), 0644)

		// Request only 2 lines
		req := httptest.NewRequest("GET", "/rpc/server/logs?lines=2", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Should only contain last 2 lines
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
		if len(lines) != 2 {
			t.Errorf("Got %d log lines, want 2", len(lines))
		}

		// Should be the last 2 lines
		if !strings.Contains(bodyStr, "line 4") || !strings.Contains(bodyStr, "line 5") {
			t.Error("Should return last 2 lines (line 4 and line 5)")
		}
	})

	t.Run("server logs with max lines limit", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		// Write a small log file
		logContent := "line 1\nline 2\nline 3\n"
		os.WriteFile(logPath, []byte(logContent), 0644)

		// Request more than max (10000)
		req := httptest.NewRequest("GET", "/rpc/server/logs?lines=20000", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Should not error, and should cap at available lines
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")

		// Should return all 3 lines (capped by file size, not by max limit)
		if len(lines) != 3 {
			t.Errorf("Got %d log lines, want 3", len(lines))
		}
	})

	t.Run("server logs default lines parameter", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file with many lines
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		// Write 150 log entries
		var logLines strings.Builder
		for i := 1; i <= 150; i++ {
			logLines.WriteString("line ")
			logLines.WriteString(strings.Repeat("x", 10))
			logLines.WriteString("\n")
		}
		os.WriteFile(logPath, []byte(logLines.String()), 0644)

		// Request without lines parameter (default: 100)
		req := httptest.NewRequest("GET", "/rpc/server/logs", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Should return last 100 lines (default)
		lines := strings.Split(strings.TrimSpace(bodyStr), "\n")
		if len(lines) != 100 {
			t.Errorf("Got %d log lines, want 100 (default)", len(lines))
		}
	})

	t.Run("server logs with invalid lines parameter", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		logContent := "line 1\nline 2\nline 3\n"
		os.WriteFile(logPath, []byte(logContent), 0644)

		// Request with invalid lines parameter
		req := httptest.NewRequest("GET", "/rpc/server/logs?lines=invalid", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Should fall back to default (100) or all lines
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if bodyStr == "" {
			t.Error("Should return logs even with invalid lines parameter")
		}
	})

	t.Run("server logs with negative lines parameter", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		logContent := "line 1\nline 2\nline 3\n"
		os.WriteFile(logPath, []byte(logContent), 0644)

		// Request with negative lines parameter
		req := httptest.NewRequest("GET", "/rpc/server/logs?lines=-10", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Should fall back to default
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if bodyStr == "" {
			t.Error("Should return logs even with negative lines parameter")
		}
	})

	t.Run("server logs content type is text/plain", func(t *testing.T) {
		server := createTestServer(t)

		// Create a log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		logContent := "line 1\n"
		os.WriteFile(logPath, []byte(logContent), 0644)

		req := httptest.NewRequest("GET", "/rpc/server/logs", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/plain" {
			t.Errorf("Content-Type = %q, want text/plain", contentType)
		}
	})

	t.Run("server logs with empty file", func(t *testing.T) {
		server := createTestServer(t)

		// Create an empty log file
		logsDir := filepath.Join(t.TempDir(), "logs")
		os.MkdirAll(logsDir, 0755)
		logPath := filepath.Join(logsDir, "audit.log")
		server.config.Logging.AuditLog = logPath

		os.WriteFile(logPath, []byte(""), 0644)

		req := httptest.NewRequest("GET", "/rpc/server/logs", nil)
		w := httptest.NewRecorder()

		server.HandleServerLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "" {
			t.Error("Should return empty string for empty log file")
		}
	})
}
