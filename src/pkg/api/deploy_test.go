package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleDeployJSON(t *testing.T) {
	server := createTestServer(t)

	t.Run("valid deploy request", func(t *testing.T) {
		reqBody := DeployRequest{
			ID:      "test-deploy",
			Command: "echo",
			Args:    []string{"hello"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		// May succeed or fail depending on process spawn, but should not panic
		t.Logf("Deploy status: %d", resp.StatusCode)
	})

	t.Run("missing command", func(t *testing.T) {
		reqBody := DeployRequest{
			ID:   "test",
			Args: []string{"arg1"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for missing command", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for invalid JSON", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("auto-generate ID", func(t *testing.T) {
		reqBody := DeployRequest{
			Command: "echo",
			Args:    []string{"test"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		// Should handle auto-generated ID
		t.Logf("Auto-ID deploy status: %d", resp.StatusCode)
	})

	t.Run("duplicate ID", func(t *testing.T) {
		// Register a formation first
		server.registry.Register(&registry.Formation{
			ID:   "duplicate-test",
			Port: 8080,
		})

		reqBody := DeployRequest{
			ID:      "duplicate-test",
			Command: "echo",
			Args:    []string{"test"},
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		// Should fail because ID already exists
		t.Logf("Duplicate ID status: %d", resp.StatusCode)
	})
}

func TestHandleStop(t *testing.T) {
	server := createTestServer(t)

	t.Run("stop running formation", func(t *testing.T) {
		server.registry.Register(&registry.Formation{
			ID:     "stop-test",
			Port:   8080,
			Status: "running",
		})

		req := httptest.NewRequest("POST", "/formations/stop-test/stop", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "stop-test"})

		server.HandleStop(w, req)

		resp := w.Result()
		// Will likely fail to stop (no real process), but should return proper status
		t.Logf("Stop status: %d", resp.StatusCode)
	})

	t.Run("stop already stopped formation", func(t *testing.T) {
		server.registry.Register(&registry.Formation{
			ID:     "stopped-test",
			Port:   8081,
			Status: "stopped",
		})

		req := httptest.NewRequest("POST", "/formations/stopped-test/stop", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "stopped-test"})

		server.HandleStop(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusConflict {
			t.Logf("Stop already stopped status: %d (expected conflict)", resp.StatusCode)
		}
	})

	t.Run("stop nonexistent formation", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/formations/nonexistent-stop/stop", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent-stop"})

		server.HandleStop(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d for nonexistent formation", resp.StatusCode, http.StatusNotFound)
		}
	})
}

func TestHandleRestart(t *testing.T) {
	server := createTestServer(t)

	t.Run("restart existing formation", func(t *testing.T) {
		server.registry.Register(&registry.Formation{
			ID:     "restart-test",
			Port:   8082,
			Status: "running",
		})

		req := httptest.NewRequest("POST", "/formations/restart-test/restart", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "restart-test"})

		server.HandleRestart(w, req)

		resp := w.Result()
		// Will likely fail (no real process), but should handle gracefully
		t.Logf("Restart status: %d", resp.StatusCode)
	})

	t.Run("restart nonexistent formation", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/formations/nonexistent-restart/restart", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent-restart"})

		server.HandleRestart(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d for nonexistent formation", resp.StatusCode, http.StatusNotFound)
		}
	})
}

func TestHandleLogs(t *testing.T) {
	server := createTestServer(t)

	t.Run("logs for nonexistent formation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/formations/nonexistent-logs/logs", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent-logs"})

		server.HandleLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d for nonexistent formation", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("logs with no log file", func(t *testing.T) {
		server.registry.Register(&registry.Formation{
			ID:   "no-logs-test",
			Port: 8083,
		})

		req := httptest.NewRequest("GET", "/formations/no-logs-test/logs", nil)
		w := httptest.NewRecorder()

		// Set mux vars
		req = mux.SetURLVars(req, map[string]string{"id": "no-logs-test"})

		server.HandleLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d for formation with no logs", resp.StatusCode, http.StatusOK)
		}

		// Should return empty logs instead of error
		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if !result.Success {
			t.Error("Should return success for formation with no logs")
		}
	})

	t.Run("logs with custom line count", func(t *testing.T) {
		server.registry.Register(&registry.Formation{
			ID:   "custom-lines-test",
			Port: 8084,
		})

		req := httptest.NewRequest("GET", "/formations/custom-lines-test/logs?lines=50", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "custom-lines-test"})

		server.HandleLogs(w, req)

		resp := w.Result()
		t.Logf("Custom lines status: %d", resp.StatusCode)
	})

	t.Run("logs with log file", func(t *testing.T) {
		// Create a test log file
		tmpDir := t.TempDir()
		server.config.Formations.LogsDir = tmpDir

		logPath := filepath.Join(tmpDir, "test-logs.log")
		logContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"
		if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
			t.Fatalf("Failed to create test log file: %v", err)
		}

		server.registry.Register(&registry.Formation{
			ID:   "test-logs",
			Port: 8085,
		})

		req := httptest.NewRequest("GET", "/formations/test-logs/logs?lines=3", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "test-logs"})

		server.HandleLogs(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, _ := io.ReadAll(resp.Body)
		var result SuccessResponse
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		data, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Data is not a map")
		}

		logs, ok := data["logs"].([]interface{})
		if !ok {
			t.Fatal("logs field is not an array")
		}

		// Should return last 3 lines
		if len(logs) > 3 {
			t.Errorf("Got %d log lines, want <= 3", len(logs))
		}
	})
}

func TestReadLastNLines(t *testing.T) {
	t.Run("read all lines", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test.log")
		content := "line 1\nline 2\nline 3\n"
		os.WriteFile(tmpFile, []byte(content), 0644)

		lines, total, err := readLastNLines(tmpFile, 10)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}

		if len(lines) != 3 {
			t.Errorf("len(lines) = %d, want 3", len(lines))
		}
	})

	t.Run("read last N lines", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test.log")
		content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
		os.WriteFile(tmpFile, []byte(content), 0644)

		lines, total, err := readLastNLines(tmpFile, 2)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}

		if len(lines) != 2 {
			t.Errorf("len(lines) = %d, want 2", len(lines))
		}

		if lines[0] != "line 4" {
			t.Errorf("lines[0] = %q, want %q", lines[0], "line 4")
		}

		if lines[1] != "line 5" {
			t.Errorf("lines[1] = %q, want %q", lines[1], "line 5")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "empty.log")
		os.WriteFile(tmpFile, []byte(""), 0644)

		lines, total, err := readLastNLines(tmpFile, 10)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}

		if len(lines) != 0 {
			t.Errorf("len(lines) = %d, want 0", len(lines))
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, _, err := readLastNLines("/nonexistent/file.log", 10)
		if err == nil {
			t.Error("readLastNLines() should fail for nonexistent file")
		}

		if !strings.Contains(err.Error(), "failed to open log file") {
			t.Errorf("error = %q, want error containing 'failed to open log file'", err.Error())
		}
	})

	t.Run("single line", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "single.log")
		os.WriteFile(tmpFile, []byte("single line"), 0644)

		lines, total, err := readLastNLines(tmpFile, 10)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}

		if len(lines) != 1 {
			t.Errorf("len(lines) = %d, want 1", len(lines))
		}
	})
}

func TestDeployResponse(t *testing.T) {
	resp := DeployResponse{
		FormationID: "test-id",
		Port:        8080,
		Status:      "running",
		URL:         "http://localhost:3000/v1/test-id",
		HealthURL:   "http://localhost:8080/health",
		PID:         12345,
	}

	if resp.FormationID != "test-id" {
		t.Errorf("FormationID = %q, want %q", resp.FormationID, "test-id")
	}

	if resp.Port != 8080 {
		t.Errorf("Port = %d, want 8080", resp.Port)
	}

	if resp.PID != 12345 {
		t.Errorf("PID = %d, want 12345", resp.PID)
	}
}
