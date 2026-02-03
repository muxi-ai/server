package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/auth"
	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog"
)

func setupDraftTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "draft-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Override getMuxiDir for tests by changing HOME
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
		os.RemoveAll(tmpDir)
	})

	// Create server directory structure
	serverDir := filepath.Join(tmpDir, ".muxi", "server")
	os.MkdirAll(serverDir, 0755)

	cfg := config.DefaultConfig()
	cfg.Formations.PortRangeStart = 19100
	cfg.Formations.PortRangeEnd = 19200

	reg, err := registry.NewRegistry(cfg.Formations.PortRangeStart, cfg.Formations.PortRangeEnd)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	logger := zerolog.Nop()
	pm, err := process.NewManager(serverDir, &logger)
	if err != nil {
		t.Fatalf("Failed to create process manager: %v", err)
	}

	accessKey, secretKey, err := auth.GenerateCredentials()
	if err != nil {
		t.Fatalf("Failed to generate credentials: %v", err)
	}

	authConfig := &config.AuthConfig{
		Enabled:            true,
		Key:                accessKey,
		Secret:             secretKey,
		TimestampTolerance: 300,
	}
	authMiddleware := auth.NewMiddleware(authConfig, &logger)

	server := NewServer(cfg, pm, reg, authMiddleware, &logger, "test-version")

	return server, tmpDir
}

func draftRequest(t *testing.T, server *Server, formationID string, req DraftRequest) *httptest.ResponseRecorder {
	t.Helper()

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/rpc/formations/"+formationID+"/draft/files", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	// Add mux vars
	httpReq = mux.SetURLVars(httpReq, map[string]string{"id": formationID})

	w := httptest.NewRecorder()
	server.HandleDraft(w, httpReq)
	return w
}

func TestDraftInit_New(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "init",
		Mode:   "new",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Success {
		t.Errorf("Expected success=true, got false: %s", resp.Message)
	}
	if !resp.Draft.Exists {
		t.Error("Expected draft.exists=true")
	}
}

func TestDraftInit_AlreadyExists(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Create first draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Try to create again
	w := draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "DraftAlreadyExists" {
		t.Errorf("Expected error=DraftAlreadyExists, got %s", resp.Error)
	}
}

func TestDraftInit_Clone_NoLive(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "init",
		Mode:   "clone",
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "LiveNotFound" {
		t.Errorf("Expected error=LiveNotFound, got %s", resp.Error)
	}
}

func TestDraftList(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Create some files
	draftDir := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft")
	os.MkdirAll(filepath.Join(draftDir, "agents"), 0755)
	os.WriteFile(filepath.Join(draftDir, "agents", "main.yaml"), []byte("test"), 0644)

	// List root
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "list",
		Path:   "/",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp.Data.(map[string]interface{})
	entries := data["entries"].([]interface{})

	// Should have at least formation.afs and agents/
	if len(entries) < 2 {
		t.Errorf("Expected at least 2 entries, got %d", len(entries))
	}
}

func TestDraftList_NoDraft(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "list",
		Path:   "/",
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "DraftNotFound" {
		t.Errorf("Expected error=DraftNotFound, got %s", resp.Error)
	}
}

func TestDraftRead(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Read formation.afs
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "read",
		Path:   "formation.afs",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp.Data.(map[string]interface{})
	content := data["content"].(string)

	if content == "" {
		t.Error("Expected non-empty content")
	}
	if data["encoding"] != "utf-8" {
		t.Errorf("Expected encoding=utf-8, got %s", data["encoding"])
	}
}

func TestDraftRead_Base64(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Write binary file
	draftDir := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft")
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	os.WriteFile(filepath.Join(draftDir, "binary.bin"), binaryData, 0644)

	// Read as base64
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action:   "read",
		Path:     "binary.bin",
		Encoding: "base64",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp.Data.(map[string]interface{})
	decoded, _ := base64.StdEncoding.DecodeString(data["content"].(string))

	if !bytes.Equal(decoded, binaryData) {
		t.Error("Base64 decoded content doesn't match original")
	}
}

func TestDraftRead_NotFound(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Try to read non-existent file
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "read",
		Path:   "nonexistent.txt",
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDraftWrite(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Write file
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action:  "write",
		Path:    "agents/main.yaml",
		Content: "name: test-agent\nmodel: gpt-4",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was created
	filePath := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft", "agents", "main.yaml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}
	if string(content) != "name: test-agent\nmodel: gpt-4" {
		t.Errorf("File content mismatch: %s", string(content))
	}
}

func TestDraftWrite_Base64(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Write binary file
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action:   "write",
		Path:     "data.bin",
		Content:  base64.StdEncoding.EncodeToString(binaryData),
		Encoding: "base64",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify file content
	filePath := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft", "data.bin")
	content, _ := os.ReadFile(filePath)
	if !bytes.Equal(content, binaryData) {
		t.Error("Binary content mismatch")
	}
}

func TestDraftDelete(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft and file
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})
	draftRequest(t, server, "test-formation", DraftRequest{
		Action:  "write",
		Path:    "test.txt",
		Content: "test",
	})

	// Delete file
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "delete",
		Path:   "test.txt",
	})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify file was deleted
	filePath := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft", "test.txt")
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File should have been deleted")
	}
}

func TestDraftDelete_NotFound(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Delete non-existent file
	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "delete",
		Path:   "nonexistent.txt",
	})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDraftDiscard(t *testing.T) {
	server, tmpDir := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	// Discard
	w := draftRequest(t, server, "test-formation", DraftRequest{Action: "discard"})

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify draft was removed
	draftDir := filepath.Join(tmpDir, ".muxi", "server", "formations", "test-formation", "draft")
	if _, err := os.Stat(draftDir); !os.IsNotExist(err) {
		t.Error("Draft directory should have been deleted")
	}
}

func TestDraftDiscard_NoDraft(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "test-formation", DraftRequest{Action: "discard"})

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDraftPathTraversal(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Create draft
	draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	testCases := []struct {
		name   string
		action string
		path   string
	}{
		{"read_traversal", "read", "../../../etc/passwd"},
		{"write_traversal", "write", "../../../tmp/evil"},
		{"delete_traversal", "delete", "../../other-formation/draft"},
		{"list_traversal", "list", "../.."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := draftRequest(t, server, "test-formation", DraftRequest{
				Action:  tc.action,
				Path:    tc.path,
				Content: "evil",
			})

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for path traversal, got %d", w.Code)
			}

			var resp DraftResponse
			json.Unmarshal(w.Body.Bytes(), &resp)

			if resp.Error != "InvalidPath" {
				t.Errorf("Expected error=InvalidPath, got %s", resp.Error)
			}
		})
	}
}

func TestDraftInvalidAction(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "test-formation", DraftRequest{
		Action: "invalid",
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "InvalidAction" {
		t.Errorf("Expected error=InvalidAction, got %s", resp.Error)
	}
}

func TestDraftInvalidFormationID(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	w := draftRequest(t, server, "INVALID_ID!", DraftRequest{
		Action: "init",
		Mode:   "new",
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Error != "InvalidFormationID" {
		t.Errorf("Expected error=InvalidFormationID, got %s", resp.Error)
	}
}

func TestDraftStatusInResponse(t *testing.T) {
	server, _ := setupDraftTestServer(t)

	// Before draft exists
	w := draftRequest(t, server, "test-formation", DraftRequest{Action: "init", Mode: "new"})

	var resp DraftResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// After init, draft should exist
	if !resp.Draft.Exists {
		t.Error("Expected draft.exists=true after init")
	}
	if resp.Draft.CreatedAt == nil {
		t.Error("Expected draft.created_at to be set")
	}
}
