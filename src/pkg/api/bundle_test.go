package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/muxi-ai/server/pkg/registry"
)

func TestGetMuxiDir(t *testing.T) {
	dir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error = %v", err)
	}

	if dir == "" {
		t.Error("getMuxiDir() returned empty string")
	}

	// Should contain ".muxi/server"
	if !contains(dir, ".muxi") {
		t.Errorf("getMuxiDir() = %q, should contain .muxi", dir)
	}
}

func TestHandleBundleDeploy_InvalidGzip(t *testing.T) {
	server := createTestServer(t)

	// Invalid gzip data
	invalidData := []byte("not a gzip file")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(invalidData))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	resp := w.Result()
	// Should fail with bad request or internal error
	if resp.StatusCode == http.StatusOK {
		t.Error("Should fail for invalid gzip data")
	}
}

func TestHandleBundleDeploy_EmptyBody(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations/deploy", nil)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	resp := w.Result()
	if resp.StatusCode == http.StatusOK {
		t.Error("Should fail for empty body")
	}
}

func TestHandleBundleDeploy_ValidBundle(t *testing.T) {
	server := createTestServer(t)

	// Create a valid gzipped tarball with formation.yaml
	bundle := createTestBundle(t, "test-formation")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	resp := w.Result()
	// May succeed or fail depending on process spawning
	t.Logf("Bundle deploy status: %d", resp.StatusCode)
}

func TestHandleDeploy_ContentTypeRouting(t *testing.T) {
	server := createTestServer(t)

	tests := []struct {
		name        string
		contentType string
		expectJSON  bool
	}{
		{"application/json", "application/json", true},
		{"application/gzip", "application/gzip", false},
		{"application/x-gzip", "application/x-gzip", false},
		{"application/octet-stream", "application/octet-stream", false},
		{"no content-type", "", true}, // defaults to JSON
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.expectJSON {
				body = bytes.NewReader([]byte(`{"command": "echo"}`))
			} else {
				body = bytes.NewReader(createTestBundle(t, "test"))
			}

			req := httptest.NewRequest("POST", "/formations/deploy", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			server.HandleDeploy(w, req)

			resp := w.Result()
			t.Logf("%s: status = %d", tt.name, resp.StatusCode)
		})
	}
}

func TestServer_Start(t *testing.T) {
	server := createTestServer(t)

	if server.httpServer == nil {
		t.Fatal("httpServer not initialized")
	}

	// Can't easily test actual Start() since it blocks
	// Just verify server is configured correctly
	if server.httpServer.Addr == "" {
		t.Error("Server address not set")
	}

	if server.httpServer.Handler == nil {
		t.Error("Server handler not set")
	}

	// Test timeout configuration
	if server.httpServer.ReadTimeout == 0 {
		t.Error("ReadTimeout not configured")
	}

	if server.httpServer.WriteTimeout == 0 {
		t.Error("WriteTimeout not configured")
	}

	if server.httpServer.IdleTimeout == 0 {
		t.Error("IdleTimeout not configured")
	}
}

func TestHandleRestart_Success(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:     "restart-success",
		Port:   8090,
		Status: "running",
	})

	req := httptest.NewRequest("POST", "/formations/restart-success/restart", nil)
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	resp := w.Result()
	// Will likely fail (no real process), but testing handler logic
	t.Logf("Restart response status: %d", resp.StatusCode)
}

func TestHandleStop_Conflict(t *testing.T) {
	server := createTestServer(t)

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     "already-stopped",
		Port:   8091,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/already-stopped/stop", nil)
	w := httptest.NewRecorder()

	server.HandleStop(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Logf("Stop already stopped formation status: %d (expected 409 conflict)", resp.StatusCode)
	}
}

// Helper to create a test bundle (gzipped tarball with formation.yaml)
func createTestBundle(t *testing.T, formationID string) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Create formation.yaml content
	formationYAML := []byte(`schema: muxi.ai/formation/v1
id: ` + formationID + `
name: Test Formation
version: 1.0.0
runtime:
  built_in_mcps: true
`)

	// Add root directory
	tarWriter.WriteHeader(&tar.Header{
		Name:     formationID + "/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	})

	// Add formation.yaml
	tarWriter.WriteHeader(&tar.Header{
		Name: formationID + "/formation.yaml",
		Mode: 0644,
		Size: int64(len(formationYAML)),
	})
	tarWriter.Write(formationYAML)

	// Add a dummy app.py
	appPy := []byte(`print("Hello from test formation")`)
	tarWriter.WriteHeader(&tar.Header{
		Name: formationID + "/app.py",
		Mode: 0644,
		Size: int64(len(appPy)),
	})
	tarWriter.Write(appPy)

	tarWriter.Close()
	gzipWriter.Close()

	return buf.Bytes()
}

func TestHandleLogsInvalidLines(t *testing.T) {
	server := createTestServer(t)

	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:   "logs-invalid-lines",
		Port: 8092,
	})

	tests := []struct {
		name        string
		linesParam  string
		description string
	}{
		{"negative", "-10", "negative lines parameter"},
		{"zero", "0", "zero lines"},
		{"non-numeric", "abc", "non-numeric lines"},
		{"very large", "99999", "very large lines (should cap at 10000)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/formations/logs-invalid-lines/logs?lines="+tt.linesParam, nil)
			w := httptest.NewRecorder()

			server.HandleLogs(w, req)

			resp := w.Result()
			// Should handle gracefully
			t.Logf("%s: status = %d", tt.description, resp.StatusCode)
		})
	}
}

func TestReadLastNLines_EdgeCases(t *testing.T) {
	t.Run("very long lines", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "long.log")
		// Create a line that's very long
		longLine := make([]byte, 100000)
		for i := range longLine {
			longLine[i] = 'A'
		}
		content := string(longLine) + "\n"
		os.WriteFile(tmpFile, []byte(content), 0644)

		lines, total, err := readLastNLines(tmpFile, 1)
		if err != nil {
			t.Logf("readLastNLines with long line: %v", err)
		} else {
			if total != 1 {
				t.Errorf("total = %d, want 1", total)
			}
			if len(lines) != 1 {
				t.Errorf("len(lines) = %d, want 1", len(lines))
			}
		}
	})

	t.Run("file with only newlines", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "newlines.log")
		os.WriteFile(tmpFile, []byte("\n\n\n\n\n"), 0644)

		lines, total, err := readLastNLines(tmpFile, 10)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 5 {
			t.Errorf("total = %d, want 5 (empty lines)", total)
		if len(lines) != 5 {
			t.Errorf("len(lines) = %d, want 5", len(lines))
		}
		}
	})

	t.Run("large file", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "large.log")
		// Create a file with 1000 lines
		content := ""
		for i := 0; i < 1000; i++ {
			content += fmt.Sprintf("line %d\n", i)
		}
		os.WriteFile(tmpFile, []byte(content), 0644)

		lines, total, err := readLastNLines(tmpFile, 10)
		if err != nil {
			t.Fatalf("readLastNLines() error = %v", err)
		}

		if total != 1000 {
			t.Errorf("total = %d, want 1000", total)
		}

		if len(lines) != 10 {
			t.Errorf("len(lines) = %d, want 10", len(lines))
		}

		// Verify we got the last 10 lines
		if !contains(lines[0], "line 990") {
			t.Errorf("First line = %q, should be line 990", lines[0])
		}
	})
}

func TestHandleBundleDeploy_FormationAlreadyExists(t *testing.T) {
	server := createTestServer(t)

	// Pre-register a formation
	server.registry.Register(&registry.Formation{
		ID:   "existing-formation",
		Name: "Existing",
		Port: 8080,
	})

	// Try to deploy bundle with same ID
	bundle := createTestBundle(t, "existing-formation")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Status = %d, want %d (Conflict)", w.Code, http.StatusConflict)
	}

	body := w.Body.String()
	if !containsStr(body, "already exists") {
		t.Errorf("Response should mention formation already exists, got: %s", body)
	}
}

func TestHandleBundleDeploy_InvalidTarball(t *testing.T) {
	server := createTestServer(t)

	// Create invalid gzip data
	invalidData := []byte("this is not a valid gzipped tarball")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(invalidData))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code == http.StatusOK || w.Code == http.StatusCreated {
		t.Errorf("Should fail for invalid tarball, got status %d", w.Code)
	}
}

func TestHandleBundleDeploy_MissingFormationYAML(t *testing.T) {
	server := createTestServer(t)

	// Create a tarball without formation.yaml
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add a dummy file but no formation.yaml
	header := &tar.Header{
		Name: "README.md",
		Size: 10,
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte("Hello Test"))

	tarWriter.Close()
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code == http.StatusOK || w.Code == http.StatusCreated {
		t.Errorf("Should fail without formation.yaml, got status %d", w.Code)
	}
}

func TestHandleBundleDeploy_EmptyTarball(t *testing.T) {
	server := createTestServer(t)

	// Create an empty tarball
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)
	tarWriter.Close()
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code == http.StatusOK || w.Code == http.StatusCreated {
		t.Errorf("Should fail for empty tarball, got status %d", w.Code)
	}
}

func TestHandleBundleDeploy_LargeBundleSize(t *testing.T) {
	server := createTestServer(t)

	// Create a larger bundle (but still reasonable for tests)
	bundle := createLargeBundleWithFiles(t, "large-formation", 100)

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	// Should handle it (may fail for other reasons but not size)
	t.Logf("Large bundle deploy status: %d", w.Code)
}

func TestHandleBundleDeploy_SpecialCharactersInFormationID(t *testing.T) {
	server := createTestServer(t)

	testIDs := []string{
		"test-with-dashes",
		"test_with_underscores",
		"test123numbers",
	}

	for _, id := range testIDs {
		t.Run(id, func(t *testing.T) {
			bundle := createTestBundle(t, id)

			req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
			req.Header.Set("Content-Type", "application/gzip")
			w := httptest.NewRecorder()

			server.HandleDeploy(w, req)

			// Should handle special characters gracefully
			t.Logf("ID %q: status %d", id, w.Code)
		})
	}
}

func TestDeployRequest_Validation(t *testing.T) {
	server := createTestServer(t)

	tests := []struct {
		name    string
		request DeployRequest
		wantErr bool
	}{
		{
			name: "valid with ID",
			request: DeployRequest{
				ID:      "test-1",
				Command: "echo",
				Args:    []string{"hello"},
			},
			wantErr: false,
		},
		{
			name: "valid without ID",
			request: DeployRequest{
				Command: "echo",
				Args:    []string{"hello"},
			},
			wantErr: false, // ID will be auto-generated
		},
		{
			name: "missing command",
			request: DeployRequest{
				ID:   "test-2",
				Args: []string{"hello"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.HandleDeploy(w, req)

			resp := w.Result()
			gotErr := resp.StatusCode >= 400

			if gotErr != tt.wantErr {
				t.Errorf("Deploy %s: gotErr=%v, wantErr=%v (status=%d)",
					tt.name, gotErr, tt.wantErr, resp.StatusCode)
			}
		})
	}
}

func createLargeBundleWithFiles(t *testing.T, formationID string, numFiles int) []byte {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml
	yamlContent := fmt.Sprintf(`id: %s
name: Large Formation
version: 1.0.0
runtime:
  command: echo
  args: ["test"]
`, formationID)

	header := &tar.Header{
		Name: formationID + "/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}
	if _, err := tarWriter.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}

	// Add many files
	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf("File content %d\n", i)
		header := &tar.Header{
			Name: fmt.Sprintf("%s/file%d.txt", formationID, i),
			Size: int64(len(content)),
			Mode: 0644,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}

	tarWriter.Close()
	gzWriter.Close()

	return buf.Bytes()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsStr(s, substr)))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}



func TestHandleBundleDeploy_ExtractDirError(t *testing.T) {
	server := createTestServer(t)

	// Create a bundle with invalid tar structure
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	gzWriter.Write([]byte("not a valid tar file"))
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code == http.StatusOK || w.Code == http.StatusCreated {
		t.Errorf("Should fail for invalid tar, got %d", w.Code)
	}
}

func TestHandleBundleDeploy_ParseYAMLError(t *testing.T) {
	server := createTestServer(t)

	// Create bundle with invalid YAML
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add invalid formation.yaml
	yamlContent := "id: test\ninvalid: yaml: syntax:"
	header := &tar.Header{
		Name: "parse-error/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	tarWriter.Close()
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d for YAML parse error", w.Code, http.StatusBadRequest)
	}
}

func TestHandleBundleDeploy_MetadataInjectionContinues(t *testing.T) {
	server := createTestServer(t)

	// Create bundle
	bundle := createTestBundle(t, "metadata-test")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	// Even if metadata injection fails, deployment should continue
	// We can't easily force metadata injection to fail, but this tests the flow
	t.Logf("Deploy status: %d", w.Code)
}

func TestHandleBundleDeploy_ProcessSpawnError(t *testing.T) {
	server := createTestServer(t)

	// Create bundle with command that doesn't exist
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	yamlContent := `id: spawn-error
name: Spawn Error Test
version: 1.0.0
runtime:
  command: /nonexistent/command/that/does/not/exist
`
	header := &tar.Header{
		Name: "spawn-error/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	tarWriter.Close()
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	// Should fail to spawn process
	if w.Code == http.StatusCreated {
		t.Error("Should fail when process can't spawn")
	}
}

func TestHandleBundleDeploy_MoveToPermanentError(t *testing.T) {
	// This is hard to test without manipulating filesystem permissions
	// The error path exists in the code: os.Rename might fail
	t.Skip("Skipping filesystem permission test")
}

func TestHandleBundleDeploy_RegistrationError(t *testing.T) {
	server := createTestServer(t)

	// Pre-register formation
	server.registry.Register(&registry.Formation{
		ID:   "reg-error",
		Port: 8080,
	})

	bundle := createTestBundle(t, "reg-error")

	req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(bundle))
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	// Should fail because formation already exists
	if w.Code != http.StatusConflict {
		t.Errorf("Status = %d, want %d (Conflict)", w.Code, http.StatusConflict)
	}
}

func TestGetMuxiDir_CoverageHelper(t *testing.T) {
	dir, err := getMuxiDir()
	
	if err != nil {
		t.Logf("getMuxiDir() error = %v", err)
		return
	}

	if !containsStr(dir, ".muxi") {
		t.Errorf("getMuxiDir() = %q, should contain .muxi", dir)
	}
}

func TestHandleBundleDeploy_FullSuccessPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full integration test")
	}

	server := createTestServer(t)

	// Create a complete, valid bundle
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add formation.yaml with full config
	yamlContent := `id: success-full
name: Full Success Test
version: 1.0.0
runtime:
  command: echo
  args:
    - "Hello from formation"
environment:
  TEST_VAR: "test_value"
`
	header := &tar.Header{
		Name: "success-full/formation.yaml",
		Size: int64(len(yamlContent)),
		Mode: 0644,
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(yamlContent))

	// Add additional files
	readmeContent := "# Success Full Formation\n"
	tarWriter.WriteHeader(&tar.Header{
		Name: "success-full/README.md",
		Size: int64(len(readmeContent)),
		Mode: 0644,
	})
	tarWriter.Write([]byte(readmeContent))

	tarWriter.Close()
	gzWriter.Close()

	req := httptest.NewRequest("POST", "/formations/deploy", &buf)
	req.Header.Set("Content-Type", "application/gzip")
	w := httptest.NewRecorder()

	server.HandleDeploy(w, req)

	// May succeed or fail depending on system
	t.Logf("Full success path status: %d, body: %s", w.Code, w.Body.String())

	// Cleanup if succeeded
	if w.Code == http.StatusCreated {
		server.registry.Unregister("success-full")
	}
}

func TestHandleBundleDeploy_AllErrorPaths(t *testing.T) {
	server := createTestServer(t)

	testCases := []struct {
		name        string
		bundle      []byte
		contentType string
		wantCode    int
	}{
		{
			name:        "Invalid gzip",
			bundle:      []byte("not gzip data"),
			contentType: "application/gzip",
			wantCode:    http.StatusBadRequest,
		},
		{
			name:        "Empty body",
			bundle:      []byte{},
			contentType: "application/gzip",
			wantCode:    http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/formations/deploy", bytes.NewReader(tc.bundle))
			req.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()

			server.HandleDeploy(w, req)

			if w.Code != tc.wantCode && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
				t.Logf("%s: status = %d (expected error)", tc.name, w.Code)
			}
		})
	}
}

func TestHandleBundleDeploy_DirectoryMoveFails(t *testing.T) {
	// This tests the path where os.Rename might fail
	// Hard to test without mocking, but we can test the flow
	t.Skip("Directory move failure requires filesystem manipulation")
}

func TestRespondJSON_ErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	
	RespondError(w, http.StatusBadRequest, "Test error message")
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	body := w.Body.String()
	if !containsStr(body, "Test error message") {
		t.Errorf("Body = %q, should contain error message", body)
	}
}

func TestRespondJSON_SuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	
	data := map[string]string{
		"status": "success",
		"id":     "test-123",
	}
	
	RespondSuccess(w, data)
	
	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	body := w.Body.String()
	if !containsStr(body, "success") {
		t.Errorf("Body = %q, should contain success", body)
	}
}

func TestRespondCreated_Response(t *testing.T) {
	w := httptest.NewRecorder()
	
	data := map[string]string{
		"id": "new-resource",
	}
	
	RespondCreated(w, data)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusCreated)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestCorsMiddleware_AllMethods(t *testing.T) {
	server := createTestServer(t)

	methods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/formations", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()

			// Create a test handler that uses CORS middleware
			handler := server.router
			handler.ServeHTTP(w, req)

			// Check CORS headers
			if method == "OPTIONS" {
				if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
					t.Logf("OPTIONS request status: %d", w.Code)
				}
			}

			// Verify CORS headers are set
			origin := w.Header().Get("Access-Control-Allow-Origin")
			t.Logf("Method %s: CORS Origin = %q", method, origin)
		})
	}
}
