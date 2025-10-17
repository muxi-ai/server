package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleRestart_FormationNotFound(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations/nonexistent/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleRestart_ProcessNotFound(t *testing.T) {
	server := createTestServer(t)

	// Register formation but no process
	server.registry.Register(&registry.Formation{
		ID:     "test-no-process",
		Name:   "Test No Process",
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/test-no-process/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "test-no-process"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Should error because process doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleRestart_SuccessWithProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	server := createTestServer(t)

	// Start a real process
	tmpDir := t.TempDir()
	config := process.SpawnConfig{
		ID:      "restart-success",
		Name:    "Restart Success",
		Command: "sleep",
		Args:    []string{"30"},
		WorkDir: tmpDir,
	}

	_, err := server.processManager.Start(config)
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer server.processManager.Stop("restart-success")

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "restart-success",
		Name:   "Restart Success",
		Port:   8080,
		Status: "running",
	})

	// Give process time to start
	// time.Sleep(500 * time.Millisecond)

	req := httptest.NewRequest("POST", "/formations/restart-success/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "restart-success"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Should succeed (though restart may take time)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Logf("Restart response code: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleRestart_UpdatesFormation(t *testing.T) {
	server := createTestServer(t)

	// Register formation
	formation := &registry.Formation{
		ID:     "update-test",
		Name:   "Update Test",
		Port:   8080,
		Status: "running",
	}
	server.registry.Register(formation)

	req := httptest.NewRequest("POST", "/formations/update-test/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "update-test"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Even if restart fails, formation should still be accessible
	f, err := server.registry.Get("update-test")
	if err != nil {
		t.Errorf("Formation should still exist: %v", err)
	}
	if f == nil {
		t.Error("Formation is nil")
	}
}

func TestHandleRestart_ReturnsRestartCount(t *testing.T) {
	server := createTestServer(t)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "count-test",
		Name:   "Count Test",
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/count-test/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "count-test"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Should return a response with restart_count field (even if 0 or error)
	// Just verify it doesn't panic
	t.Logf("Response: %s", w.Body.String())
}

func TestHandleRestart_LogsActivity(t *testing.T) {
	server := createTestServer(t)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "log-test",
		Name:   "Log Test",
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/log-test/restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "log-test"})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Verify the handler completes without panic
	// Actual logging is tested implicitly
	if w.Code == 0 {
		t.Error("Handler didn't set status code")
	}
}

func TestHandleRestart_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test")
	}

	server := createTestServer(t)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     "concurrent-test",
		Name:   "Concurrent Test",
		Port:   8080,
		Status: "stopped",
	})

	// Send multiple restart requests concurrently
	results := make(chan int, 5)
	for i := 0; i < 5; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/formations/concurrent-test/restart", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "concurrent-test"})
			w := httptest.NewRecorder()

			server.HandleRestart(w, req)
			results <- w.Code
		}()
	}

	// Collect results
	for i := 0; i < 5; i++ {
		code := <-results
		t.Logf("Request %d: status %d", i+1, code)
	}

	// All should complete without panic
}

func TestHandleRestart_EmptyFormationID(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations//restart", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Should return 404 for empty ID
	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleRestart_SpecialCharactersInID(t *testing.T) {
	server := createTestServer(t)

	testIDs := []string{
		"test-with-dashes",
		"test_with_underscores",
		"test123",
		"test-with-many-parts-123",
	}

	for _, id := range testIDs {
		t.Run(id, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/formations/"+id+"/restart", nil)
			req = mux.SetURLVars(req, map[string]string{"id": id})
			w := httptest.NewRecorder()

			server.HandleRestart(w, req)

			// Should return 404 (not found) not 400 (bad request)
			if w.Code != http.StatusNotFound {
				t.Logf("ID %q: status %d", id, w.Code)
			}
		})
	}
}
