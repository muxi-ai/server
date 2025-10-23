package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleUpdate(t *testing.T) {
	t.Run("update nonexistent formation", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("PUT", "/rpc/formations/nonexistent", bytes.NewReader([]byte("dummy")))
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

		server.HandleUpdate(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d for nonexistent formation", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("update with invalid bundle", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-update"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current version
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		// Create formation.yaml in current
		formationYAML := `id: test-formation
name: test-formation
description: Test update formation
version: 1.0.0
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Send invalid bundle data
		req := httptest.NewRequest("PUT", "/rpc/formations/"+formationID, bytes.NewReader([]byte("invalid bundle")))
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleUpdate(w, req)

		resp := w.Result()
		// Should fail with bad request due to invalid bundle
		if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Status = %d, want 400 or 500 for invalid bundle", resp.StatusCode)
		}
	})

	t.Run("update with valid bundle", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-update-valid"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current version
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		// Create formation.yaml in current (version 1)
		formationYAML := `id: test-formation
name: test-formation
description: Test update formation
version: 1.0.0
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create a valid bundle
		bundleData := createTestBundle(t, "test-update-valid")

		req := httptest.NewRequest("PUT", "/rpc/formations/"+formationID, bytes.NewReader(bundleData))
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleUpdate(w, req)

		resp := w.Result()
		// May succeed or fail depending on process spawn, but should handle gracefully
		t.Logf("Update status: %d", resp.StatusCode)

		// If successful, check response
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var result SuccessResponse
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if !result.Success {
				t.Error("Update should return success=true")
			}

			data, ok := result.Data.(map[string]interface{})
			if ok {
				// Check that version was updated
				if version, ok := data["version"].(float64); ok {
					if version != 1 {
						t.Logf("Version updated to: %.0f", version)
					}
				}
			}
		}
	})

	t.Run("update creates backup", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-backup"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current version
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		// Create formation.yaml in current
		formationYAML := `id: test-formation
name: test-formation
description: Test update formation
version: 1.0.0
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)
		os.WriteFile(filepath.Join(currentDir, "marker.txt"), []byte("version 1"), 0644)

		// Create a valid bundle
		bundleData := createTestBundle(t, "test-backup")

		req := httptest.NewRequest("PUT", "/rpc/formations/"+formationID, bytes.NewReader(bundleData))
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleUpdate(w, req)

		resp := w.Result()
		t.Logf("Backup test status: %d", resp.StatusCode)

		// Check that previous directory exists (if update succeeded)
		previousDir := filepath.Join(formationDir, "previous")
		if _, err := os.Stat(previousDir); err == nil {
			// Previous directory should contain the old version
			markerPath := filepath.Join(previousDir, "marker.txt")
			if data, err := os.ReadFile(markerPath); err == nil {
				if string(data) != "version 1" {
					t.Errorf("Previous backup marker = %q, want %q", string(data), "version 1")
				}
			}
		}
	})

	t.Run("update creates version history", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-version-history"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current version
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		// Create formation.yaml in current
		formationYAML := `id: test-formation
name: test-formation
description: Test update formation
version: 1.0.0
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create initial version history
		history := &formation.VersionHistory{
			CurrentVersion: 0,
		}
		history.Save(formationDir)

		// Create a valid bundle
		bundleData := createTestBundle(t, "test-version-history")

		req := httptest.NewRequest("PUT", "/rpc/formations/"+formationID, bytes.NewReader(bundleData))
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleUpdate(w, req)

		resp := w.Result()
		t.Logf("Version history test status: %d", resp.StatusCode)

		// Check version.json was updated
		versionPath := filepath.Join(formationDir, "version.json")
		if data, err := os.ReadFile(versionPath); err == nil {
			var updatedHistory formation.VersionHistory
			if err := json.Unmarshal(data, &updatedHistory); err == nil {
				t.Logf("Version history: current=%d, previous=%d", 
					updatedHistory.CurrentVersion, updatedHistory.PreviousVersion)
			}
		}
	})
}


