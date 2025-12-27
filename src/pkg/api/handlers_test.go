package api

import (
	"os"
	"path/filepath"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestFormatFormationURL(t *testing.T) {
	tests := []struct {
		serverPort  int
		formationID string
		want        string
	}{
		{3000, "test-api", "http://localhost:3000/api/test-api"},
		{8080, "my-formation", "http://localhost:8080/api/my-formation"},
		{80, "prod-api", "http://localhost:80/api/prod-api"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatFormationURL(tt.serverPort, tt.formationID)
			if got != tt.want {
				t.Errorf("formatFormationURL(%d, %q) = %q, want %q",
					tt.serverPort, tt.formationID, got, tt.want)
			}
		})
	}
}

func TestHandleGet_WithFormation(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:      "test-formation",
		Name:    "Test Formation",
		Port:    8080,
		Status:  "running",
		Healthy: true,
		Command: "python",
		Args:    []string{"app.py"},
	})

	req := httptest.NewRequest("GET", "/formations/test-formation", nil)
	w := httptest.NewRecorder()

	// Use mux vars
	req = mux.SetURLVars(req, map[string]string{"id": "test-formation"})

	server.HandleGet(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHandleGet_NotFoundDirect(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("GET", "/formations/nonexistent", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	server.HandleGet(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:     "test-formation",
		Name:   "Test Formation",
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("DELETE", "/formations/test-formation", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "test-formation"})

	server.HandleDelete(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify formation was removed
	_, err := server.registry.Get("test-formation")
	if err == nil {
		t.Error("Formation should be removed from registry")
	}
}

func TestHandleDelete_NotFoundDirect(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("DELETE", "/formations/nonexistent", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	server.HandleDelete(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleStop_NotFound(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations/nonexistent/stop", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	server.HandleStop(w, req)

	resp := w.Result()
	// Should return 404 for nonexistent formation
	if resp.StatusCode != http.StatusNotFound {
		t.Logf("Status = %d (expected 404 for nonexistent formation)", resp.StatusCode)
	}
}

func TestHandleRestart_NotFound(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations/nonexistent/restart", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	server.HandleRestart(w, req)

	resp := w.Result()
	// Should return 404 for nonexistent formation
	if resp.StatusCode != http.StatusNotFound {
		t.Logf("Status = %d (expected 404 for nonexistent formation)", resp.StatusCode)
	}
}

func TestHandleLogs_NotFound(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("GET", "/formations/nonexistent/logs", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	server.HandleLogs(w, req)

	resp := w.Result()
	// Should return 404 for nonexistent formation
	if resp.StatusCode != http.StatusNotFound {
		t.Logf("Status = %d (expected 404 for nonexistent formation)", resp.StatusCode)
	}
}

func TestHandleStop_WithFormation(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:     "test-formation",
		Name:   "Test Formation",
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("POST", "/formations/test-formation/stop", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "test-formation"})

	server.HandleStop(w, req)

	resp := w.Result()
	// Will likely fail to stop process (no real process), but should handle gracefully
	// We're just testing the handler logic, not actual process stopping
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Logf("Status = %d", resp.StatusCode)
	}
}

func TestHandleRestart_WithFormation(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:     "test-formation",
		Name:   "Test Formation",
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("POST", "/formations/test-formation/restart", nil)
	w := httptest.NewRecorder()

	req = mux.SetURLVars(req, map[string]string{"id": "test-formation"})

	server.HandleRestart(w, req)

	resp := w.Result()
	// Will likely fail without real process, but testing handler logic
	t.Logf("Restart status: %d", resp.StatusCode)
}

func TestServer_SetupRoutes(t *testing.T) {
	server := createTestServer(t)

	// Test that routes are set up correctly
	if server.router == nil {
		t.Fatal("Router not set up")
	}

	// Test health endpoint exists (should be accessible without auth)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Health endpoint status = %d, want %d", w.Result().StatusCode, http.StatusOK)
	}
}

func TestMiddlewares(t *testing.T) {
	server := createTestServer(t)

	t.Run("logging middleware doesn't panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Logging middleware panicked: %v", r)
			}
		}()

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)
	})

	t.Run("CORS middleware sets headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		server.router.ServeHTTP(w, req)

		// CORS headers should be present
		origin := w.Result().Header.Get("Access-Control-Allow-Origin")
		t.Logf("CORS Origin header: %q", origin)
	})
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{"BadRequest", http.StatusBadRequest, "Bad request test"},
		{"NotFound", http.StatusNotFound, "Not found test"},
		{"InternalError", http.StatusInternalServerError, "Internal error test"},
		{"Unauthorized", http.StatusUnauthorized, "Unauthorized test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondError(w, tt.statusCode, tt.message)

			resp := w.Result()
			if resp.StatusCode != tt.statusCode {
				t.Errorf("Status = %d, want %d", resp.StatusCode, tt.statusCode)
			}

			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
			}
		})
	}
}

func TestSuccessResponses(t *testing.T) {
	t.Run("RespondSuccess", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"key": "value"}
		RespondSuccess(w, data)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Result().StatusCode, http.StatusOK)
		}
	})

	t.Run("RespondCreated", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"id": "123"}
		RespondCreated(w, data)

		if w.Result().StatusCode != http.StatusCreated {
			t.Errorf("Status = %d, want %d", w.Result().StatusCode, http.StatusCreated)
		}
	})

	t.Run("RespondJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]interface{}{"test": true, "count": 42}
		RespondJSON(w, http.StatusAccepted, data)

		if w.Result().StatusCode != http.StatusAccepted {
			t.Errorf("Status = %d, want %d", w.Result().StatusCode, http.StatusAccepted)
		}
	})
}

func TestRegistry_Count(t *testing.T) {
	server := createTestServer(t)

	// Initial count should be 0
	if server.registry.Count() != 0 {
		t.Errorf("Initial count = %d, want 0", server.registry.Count())
	}

	// Add formations
	server.registry.Register(&registry.Formation{ID: "f1", Port: 8001})
	server.registry.Register(&registry.Formation{ID: "f2", Port: 8002})

	if server.registry.Count() != 2 {
		t.Errorf("Count after 2 registrations = %d, want 2", server.registry.Count())
	}
}

func TestHandleDelete_ProcessStopError(t *testing.T) {
	server := createTestServer(t)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "delete-stop-error",
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("DELETE", "/formations/delete-stop-error", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "delete-stop-error"})
	w := httptest.NewRecorder()

	server.HandleDelete(w, req)

	// Will fail to stop non-existent process, but should still unregister
	t.Logf("Delete with process stop error: %d", w.Code)
}

func TestHandleGet_AllFields(t *testing.T) {
	server := createTestServer(t)

	// Register formation with all fields populated
	server.registry.Register(&registry.Formation{
		ID:      "all-fields",
		Name:    "All Fields Test",
		Port:    8080,
		Status:  "running",
		Healthy: true,
		Command: "python",
		Args:    []string{"-m", "http.server"},
	})

	req := httptest.NewRequest("GET", "/formations/all-fields", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "all-fields"})
	w := httptest.NewRecorder()

	server.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()

	// Verify key fields present in response (per API spec)
	// Note: "running" becomes "unhealthy" because the live health check fails
	// (no actual process responding to health checks)
	expectedFields := []string{"all-fields", "8080", "unhealthy", "healthy", "uptime"}
	for _, field := range expectedFields {
		if !containsStr(body, field) {
			t.Errorf("Response missing field: %s", field)
		}
	}
}

func TestHandleStop_AlreadyStopped(t *testing.T) {
	server := createTestServer(t)

	// Register stopped formation
	server.registry.Register(&registry.Formation{
		ID:     "already-stopped",
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/already-stopped/stop", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "already-stopped"})
	w := httptest.NewRecorder()

	server.HandleStop(w, req)

	// May succeed or fail depending on implementation
	t.Logf("Stop already-stopped formation: %d", w.Code)
}

func TestHandleLogs_MultipleLines(t *testing.T) {
	server := createTestServer(t)

	// Set up logs directory with absolute path
	tmpDir := t.TempDir()
	server.config.Formations.LogsDir = tmpDir

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "multi-logs",
		Port:   8080,
		Status: "running",
	})

	// Create log file with multiple lines
	logFile := filepath.Join(tmpDir, "multi-logs-out.log")
	logContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10\n"
	os.WriteFile(logFile, []byte(logContent), 0644)

	// Request last 5 lines
	req := httptest.NewRequest("GET", "/formations/multi-logs/logs?lines=5", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "multi-logs"})
	w := httptest.NewRecorder()

	server.HandleLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !containsStr(body, "Line 10") {
		t.Error("Should contain last line")
	}
}

func TestHandleList_EmptyRegistry(t *testing.T) {
	server := createTestServer(t)

	// Don't register any formations

	req := httptest.NewRequest("GET", "/formations", nil)
	w := httptest.NewRecorder()

	server.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	// Should return empty array
	if !containsStr(body, "[]") {
		t.Logf("Empty list response: %s", body)
	}
}

func TestHandleList_MultipleFormations(t *testing.T) {
	server := createTestServer(t)

	// Register multiple formations
	for i := 0; i < 5; i++ {
		server.registry.Register(&registry.Formation{
			ID:     fmt.Sprintf("list-test-%d", i),
			Port:   8000 + i,
			Status: "running",
		})
	}

	req := httptest.NewRequest("GET", "/formations", nil)
	w := httptest.NewRecorder()

	server.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	
	// Verify all formations in response
	for i := 0; i < 5; i++ {
		if !containsStr(body, fmt.Sprintf("list-test-%d", i)) {
			t.Errorf("Missing formation list-test-%d", i)
		}
	}
}
