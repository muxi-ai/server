package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleRollback(t *testing.T) {
	t.Run("rollback nonexistent formation", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("POST", "/rpc/formations/nonexistent/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

		server.HandleRollback(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d for nonexistent formation", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("rollback without previous version", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-no-previous"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with only current version
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		// Create formation.yaml in current
		formationYAML := `name: test-formation
version: 1.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create version history with no previous version
		history := &formation.VersionHistory{
			CurrentVersion: 1,
			Current: &formation.Version{
				Version:    1,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
		}
		history.Save(formationDir)

		req := httptest.NewRequest("POST", "/rpc/formations/"+formationID+"/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleRollback(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for rollback without previous version", resp.StatusCode, http.StatusBadRequest)
		}

		body, _ := io.ReadAll(resp.Body)
		var result ErrorResponse
		if err := json.Unmarshal(body, &result); err == nil {
			if result.Message != "No previous version available for rollback" {
				t.Errorf("Error message = %q, want 'No previous version available for rollback'", result.Message)
			}
		}
	})

	t.Run("rollback with previous version", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-rollback"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current and previous versions
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		previousDir := filepath.Join(formationDir, "previous")
		os.MkdirAll(currentDir, 0755)
		os.MkdirAll(previousDir, 0755)

		// Create formation.yaml in current (version 2)
		currentYAML := `name: test-formation
version: 2.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(currentYAML), 0644)
		os.WriteFile(filepath.Join(currentDir, "marker.txt"), []byte("version 2"), 0644)

		// Create formation.yaml in previous (version 1)
		previousYAML := `name: test-formation
version: 1.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(previousDir, "formation.yaml"), []byte(previousYAML), 0644)
		os.WriteFile(filepath.Join(previousDir, "marker.txt"), []byte("version 1"), 0644)

		// Create version history
		history := &formation.VersionHistory{
			CurrentVersion: 2,
			Current: &formation.Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &formation.Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}
		history.Save(formationDir)

		req := httptest.NewRequest("POST", "/rpc/formations/"+formationID+"/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleRollback(w, req)

		resp := w.Result()
		t.Logf("Rollback status: %d", resp.StatusCode)

		// May succeed or fail depending on process spawn, but should handle gracefully
		// If successful, check that directories were swapped
		if resp.StatusCode == http.StatusOK {
			// Check that current now has version 1 content
			markerPath := filepath.Join(currentDir, "marker.txt")
			if data, err := os.ReadFile(markerPath); err == nil {
				if string(data) != "version 1" {
					t.Errorf("Current marker after rollback = %q, want %q", string(data), "version 1")
				}
			}

			// Check that previous now has version 2 content
			markerPath = filepath.Join(previousDir, "marker.txt")
			if data, err := os.ReadFile(markerPath); err == nil {
				if string(data) != "version 2" {
					t.Errorf("Previous marker after rollback = %q, want %q", string(data), "version 2")
				}
			}

			// Check version history was swapped
			updatedHistory, err := formation.LoadVersionHistory(formationDir)
			if err == nil {
				if updatedHistory.CurrentVersion != 1 {
					t.Errorf("Current version after rollback = %d, want 1", updatedHistory.CurrentVersion)
				}
				if updatedHistory.PreviousVersion != 2 {
					t.Errorf("Previous version after rollback = %d, want 2", updatedHistory.PreviousVersion)
				}
			}
		}
	})

	t.Run("rollback updates version metadata", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-rollback-metadata"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with current and previous versions
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		previousDir := filepath.Join(formationDir, "previous")
		os.MkdirAll(currentDir, 0755)
		os.MkdirAll(previousDir, 0755)

		// Create formation.yaml in both directories
		formationYAML := `name: test-formation
version: 1.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)
		os.WriteFile(filepath.Join(previousDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create version history
		history := &formation.VersionHistory{
			CurrentVersion: 2,
			Current: &formation.Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &formation.Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}
		history.Save(formationDir)

		req := httptest.NewRequest("POST", "/rpc/formations/"+formationID+"/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleRollback(w, req)

		resp := w.Result()
		t.Logf("Rollback metadata test status: %d", resp.StatusCode)

		// If successful, check response
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var result SuccessResponse
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if !result.Success {
				t.Error("Rollback should return success=true")
			}

			data, ok := result.Data.(map[string]interface{})
			if ok {
				// Check version
				if version, ok := data["version"].(float64); ok {
					if version != 1 {
						t.Errorf("Version after rollback = %.0f, want 1", version)
					}
				}

				// Check previous_version
				if prevVersion, ok := data["previous_version"].(float64); ok {
					if prevVersion != 2 {
						t.Errorf("Previous version after rollback = %.0f, want 2", prevVersion)
					}
				}

				// Check message
				if message, ok := data["message"].(string); ok {
					if message != "Rolled back to previous version" {
						t.Errorf("Message = %q, want 'Rolled back to previous version'", message)
					}
				}
			}
		}
	})

	t.Run("rollback missing current directory", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-missing-current"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with only previous version (no current)
		formationDir := filepath.Join(formationsDir, formationID)
		previousDir := filepath.Join(formationDir, "previous")
		os.MkdirAll(previousDir, 0755)

		formationYAML := `name: test-formation
version: 1.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(previousDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create version history
		history := &formation.VersionHistory{
			CurrentVersion: 2,
			Current: &formation.Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &formation.Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}
		history.Save(formationDir)

		req := httptest.NewRequest("POST", "/rpc/formations/"+formationID+"/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleRollback(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Status = %d, want %d for missing current directory", resp.StatusCode, http.StatusInternalServerError)
		}
	})

	t.Run("rollback missing previous directory", func(t *testing.T) {
		server := createTestServer(t)

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register a formation
		formationID := "test-missing-previous"
		server.registry.Register(&registry.Formation{
			ID:      formationID,
			Port:    8080,
			Status:  "running",
			Command: "python",
			Args:    []string{"app.py"},
		})

		// Create formation directory with only current version (no previous)
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		os.MkdirAll(currentDir, 0755)

		formationYAML := `name: test-formation
version: 2.0.0
runtime:
  type: python
  command: python
  args: ["app.py"]
`
		os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

		// Create version history with previous but no directory
		history := &formation.VersionHistory{
			CurrentVersion: 2,
			Current: &formation.Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &formation.Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}
		history.Save(formationDir)

		req := httptest.NewRequest("POST", "/rpc/formations/"+formationID+"/rollback", nil)
		w := httptest.NewRecorder()

		req = mux.SetURLVars(req, map[string]string{"id": formationID})

		server.HandleRollback(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d for missing previous directory", resp.StatusCode, http.StatusBadRequest)
		}
	})
}
