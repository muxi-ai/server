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

func TestHandleDeploy_RequiresBundle(t *testing.T) {
	server := createTestServer(t)

	t.Run("rejects JSON content type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader([]byte(`{"id": "test"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for JSON content type", resp.StatusCode, http.StatusBadRequest)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "application/gzip") {
			t.Error("Error message should mention application/gzip")
		}
	})

	t.Run("rejects no content type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader([]byte("data")))
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for missing content type", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestHandleDeploy_HeaderValidation(t *testing.T) {
	server := createTestServer(t)

	t.Run("rejects missing X-Formation-ID header", func(t *testing.T) {
		bundle := createTestBundle(t, "test-missing-header")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		// No X-Formation-ID header
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for missing X-Formation-ID", resp.StatusCode, http.StatusBadRequest)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "X-Formation-ID") {
			t.Error("Error message should mention X-Formation-ID header")
		}
	})

	t.Run("rejects invalid formation ID format", func(t *testing.T) {
		bundle := createTestBundle(t, "test")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "INVALID_ID") // uppercase not allowed
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for invalid ID format", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("rejects conflicting formation ID", func(t *testing.T) {
		// Pre-register a formation
		server.registry.Register(&registry.Formation{
			ID:   "existing-formation",
			Port: 8080,
		})

		bundle := createTestBundle(t, "existing-formation")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "existing-formation")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Status = %d, want %d for existing formation", resp.StatusCode, http.StatusConflict)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "already exists") {
			t.Error("Error message should mention formation already exists")
		}
	})

	t.Run("rejects ID mismatch between header and bundle", func(t *testing.T) {
		bundle := createTestBundle(t, "bundle-id")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "header-id") // Different from bundle
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for ID mismatch", resp.StatusCode, http.StatusBadRequest)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "mismatch") {
			t.Error("Error message should mention ID mismatch")
		}
	})

	t.Run("accepts matching header and bundle ID", func(t *testing.T) {
		bundle := createTestBundle(t, "matching-id")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "matching-id")
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		// Should not fail due to header validation
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			if strings.Contains(string(body), "X-Formation-ID") || strings.Contains(string(body), "mismatch") {
				t.Error("Should not fail header validation when IDs match")
			}
		}
		t.Logf("Deploy with matching IDs status: %d", resp.StatusCode)
	})

	t.Run("rejects version mismatch between header and bundle", func(t *testing.T) {
		bundle := createTestBundleWithVersion(t, "version-mismatch", "2.0.0")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "version-mismatch")
		req.Header.Set("X-Formation-Version", "1.0.0") // Different from bundle
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for version mismatch", resp.StatusCode, http.StatusBadRequest)
		}

		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "version") && !strings.Contains(string(body), "mismatch") {
			t.Error("Error message should mention version mismatch")
		}
	})

	t.Run("uses default version when header not provided", func(t *testing.T) {
		// Bundle with version 1.0.0 (the default)
		bundle := createTestBundleWithVersion(t, "default-version", "1.0.0")
		req := httptest.NewRequest("POST", "/rpc/formations", bytes.NewReader(bundle))
		req.Header.Set("Content-Type", "application/gzip")
		req.Header.Set("X-Formation-ID", "default-version")
		// No X-Formation-Version header - should default to 1.0.0
		w := httptest.NewRecorder()

		server.HandleDeploy(w, req)

		resp := w.Result()
		// Should not fail due to version validation
		if resp.StatusCode == http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			if strings.Contains(string(body), "version") {
				t.Error("Should not fail version validation when using default")
			}
		}
		t.Logf("Deploy with default version status: %d", resp.StatusCode)
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
		URL:         "http://localhost:3000/api/test-id",
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
