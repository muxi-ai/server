package api

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
)

func TestHandleDownload(t *testing.T) {
	t.Run("download nonexistent formation", func(t *testing.T) {
		server := createTestServer(t)

		req := httptest.NewRequest("GET", "/rpc/formations/nonexistent/download", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
		w := httptest.NewRecorder()

		server.HandleDownload(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("download formation without current directory", func(t *testing.T) {
		server := createTestServer(t)
		formationID := "test-download-no-dir"

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Register formation but don't create current/ directory
		server.registry.Register(&registry.Formation{
			ID:     formationID,
			Port:   8080,
			Status: "running",
		})

		req := httptest.NewRequest("GET", "/rpc/formations/"+formationID+"/download", nil)
		req = mux.SetURLVars(req, map[string]string{"id": formationID})
		w := httptest.NewRecorder()

		server.HandleDownload(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("download formation with files", func(t *testing.T) {
		server := createTestServer(t)
		formationID := "test-download"

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Create formation directory structure
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		if err := os.MkdirAll(currentDir, 0755); err != nil {
			t.Fatalf("Failed to create current dir: %v", err)
		}

		// Create test files
		testFiles := map[string]string{
			"formation.yaml":       "id: test-download\nversion: 1.0.0",
			"agents/main.yaml":     "name: main-agent",
			"knowledge/readme.txt": "Knowledge base readme",
		}

		for path, content := range testFiles {
			fullPath := filepath.Join(currentDir, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("Failed to create dir for %s: %v", path, err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write %s: %v", path, err)
			}
		}

		// Register formation
		server.registry.Register(&registry.Formation{
			ID:     formationID,
			Port:   8080,
			Status: "running",
		})

		req := httptest.NewRequest("GET", "/rpc/formations/"+formationID+"/download", nil)
		req = mux.SetURLVars(req, map[string]string{"id": formationID})
		w := httptest.NewRecorder()

		server.HandleDownload(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, body)
		}

		// Check content type
		contentType := resp.Header.Get("Content-Type")
		if contentType != "application/zip" {
			t.Errorf("Content-Type = %q, want %q", contentType, "application/zip")
		}

		// Check content disposition
		disposition := resp.Header.Get("Content-Disposition")
		expectedDisposition := `attachment; filename="test-download.zip"`
		if disposition != expectedDisposition {
			t.Errorf("Content-Disposition = %q, want %q", disposition, expectedDisposition)
		}

		// Verify zip contents
		body, _ := io.ReadAll(resp.Body)
		zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		// Check that all expected files are in the zip
		foundFiles := make(map[string]bool)
		for _, file := range zipReader.File {
			foundFiles[file.Name] = true

			// Verify content of formation.yaml
			if file.Name == "formation.yaml" {
				rc, err := file.Open()
				if err != nil {
					t.Fatalf("Failed to open formation.yaml in zip: %v", err)
				}
				content, _ := io.ReadAll(rc)
				rc.Close()
				if string(content) != testFiles["formation.yaml"] {
					t.Errorf("formation.yaml content = %q, want %q", content, testFiles["formation.yaml"])
				}
			}
		}

		// Verify expected files exist
		for path := range testFiles {
			if !foundFiles[path] {
				t.Errorf("Expected file %q not found in zip", path)
			}
		}
	})

	t.Run("download excludes hidden files except .env", func(t *testing.T) {
		server := createTestServer(t)
		formationID := "test-download-hidden"

		// Setup formations directory
		formationsDir := filepath.Join(t.TempDir(), "formations")
		os.MkdirAll(formationsDir, 0755)
		server.config.Formations.FormationsDir = formationsDir

		// Create formation directory structure
		formationDir := filepath.Join(formationsDir, formationID)
		currentDir := filepath.Join(formationDir, "current")
		if err := os.MkdirAll(currentDir, 0755); err != nil {
			t.Fatalf("Failed to create current dir: %v", err)
		}

		// Create test files including hidden ones
		testFiles := map[string]string{
			"formation.yaml": "id: test",
			".env":           "SECRET=value",   // Should be included
			".git/config":    "git config",     // Should be excluded
			".hidden":        "hidden content", // Should be excluded
		}

		for path, content := range testFiles {
			fullPath := filepath.Join(currentDir, path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("Failed to create dir for %s: %v", path, err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write %s: %v", path, err)
			}
		}

		// Register formation
		server.registry.Register(&registry.Formation{
			ID:     formationID,
			Port:   8080,
			Status: "running",
		})

		req := httptest.NewRequest("GET", "/rpc/formations/"+formationID+"/download", nil)
		req = mux.SetURLVars(req, map[string]string{"id": formationID})
		w := httptest.NewRecorder()

		server.HandleDownload(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Verify zip contents
		body, _ := io.ReadAll(resp.Body)
		zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatalf("Failed to read zip: %v", err)
		}

		foundFiles := make(map[string]bool)
		for _, file := range zipReader.File {
			foundFiles[file.Name] = true
		}

		// .env should be included
		if !foundFiles[".env"] {
			t.Error(".env should be included in zip")
		}

		// formation.yaml should be included
		if !foundFiles["formation.yaml"] {
			t.Error("formation.yaml should be included in zip")
		}

		// .hidden should be excluded
		if foundFiles[".hidden"] {
			t.Error(".hidden should be excluded from zip")
		}

		// .git/config should be excluded
		if foundFiles[".git/config"] {
			t.Error(".git/config should be excluded from zip")
		}
	})
}
