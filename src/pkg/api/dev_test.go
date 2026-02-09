package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevRunRouteRegistered(t *testing.T) {
	server := createTestServer(t)

	// Test that the route is registered (not 404)
	req := httptest.NewRequest("POST", "/rpc/dev/run", strings.NewReader(`{"path":"/tmp"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	t.Logf("Status: %d, Body: %s", w.Code, w.Body.String())

	// Should NOT be 404 - route should be registered
	// Will be 400 (bad request) since /tmp doesn't have formation.afs
	if w.Code == http.StatusNotFound {
		t.Errorf("Route /rpc/dev/run returned 404 - route not registered!")
	}
}

func TestDevStopRouteRegistered(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/rpc/dev/stop", strings.NewReader(`{"formation_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	t.Logf("Status: %d, Body: %s", w.Code, w.Body.String())

	if w.Code == http.StatusNotFound {
		t.Errorf("Route /rpc/dev/stop returned 404 - route not registered!")
	}
}

func TestHandleDevRun_MissingParams(t *testing.T) {
	server := createTestServer(t)

	t.Run("empty request body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDevRun(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var result DevRunResponse
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Success {
			t.Error("Expected success=false")
		}
	})

	t.Run("relative path", func(t *testing.T) {
		body := `{"path": "relative/path"}`
		req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDevRun(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		body := `{"path": "/nonexistent/path/that/does/not/exist"}`
		req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.HandleDevRun(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestHandleDevRun_MissingFormationAFS(t *testing.T) {
	server := createTestServer(t)

	// Create a temp directory without formation.afs
	tmpDir := t.TempDir()

	body, _ := json.Marshal(DevRunRequest{Path: tmpDir})
	req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleDevRun(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result DevRunResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("Expected success=false for missing formation.afs")
	}
}

func TestHandleDevRun_InvalidFormationAFS(t *testing.T) {
	server := createTestServer(t)

	// Create a temp directory with invalid formation.afs
	tmpDir := t.TempDir()
	afsPath := filepath.Join(tmpDir, "formation.afs")
	os.WriteFile(afsPath, []byte("not: valid: yaml: {{{"), 0644)

	body, _ := json.Marshal(DevRunRequest{Path: tmpDir})
	req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleDevRun(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleDevRun_MissingFormationID(t *testing.T) {
	server := createTestServer(t)

	// Create a temp directory with formation.afs without id
	tmpDir := t.TempDir()
	afsPath := filepath.Join(tmpDir, "formation.afs")
	os.WriteFile(afsPath, []byte("name: Test Formation\nversion: 1.0.0\n"), 0644)

	body, _ := json.Marshal(DevRunRequest{Path: tmpDir})
	req := httptest.NewRequest("POST", "/rpc/dev/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleDevRun(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var result DevRunResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Error == "" {
		t.Error("Expected error message for missing formation ID")
	}
}

func TestHandleDevStop_MissingFormationID(t *testing.T) {
	server := createTestServer(t)

	body := `{}`
	req := httptest.NewRequest("POST", "/rpc/dev/stop", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleDevStop(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleDevStop_NotFound(t *testing.T) {
	server := createTestServer(t)

	body := `{"formation_id": "nonexistent-formation"}`
	req := httptest.NewRequest("POST", "/rpc/dev/stop", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleDevStop(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var result DevStopResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Success {
		t.Error("Expected success=false for nonexistent formation")
	}
}
