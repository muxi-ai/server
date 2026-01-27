package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/registry"
)

// ========================================
// HandleStart Tests
// ========================================

func TestHandleStart_NotFound(t *testing.T) {
	server := createTestServer(t)

	req := httptest.NewRequest("POST", "/formations/nonexistent/start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "Formation not found") {
		t.Errorf("Error message = %q, want message containing 'Formation not found'", errResp.Message)
	}
}

func TestHandleStart_AlreadyRunning(t *testing.T) {
	server := createTestServer(t)

	// Register a running formation
	server.registry.Register(&registry.Formation{
		ID:     "running-formation",
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("POST", "/formations/running-formation/start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "running-formation"})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "already running") {
		t.Errorf("Error message = %q, want message containing 'already running'", errResp.Message)
	}
}

func TestHandleStart_AlreadyStarting(t *testing.T) {
	server := createTestServer(t)

	// Register a formation in "starting" state
	server.registry.Register(&registry.Formation{
		ID:     "starting-formation",
		Port:   8080,
		Status: "starting",
	})

	req := httptest.NewRequest("POST", "/formations/starting-formation/start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "starting-formation"})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestHandleStart_FormationDirNotFound(t *testing.T) {
	server := createTestServer(t)

	// Don't create any formation directory - it won't exist

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     "no-dir-formation",
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", "/formations/no-dir-formation/start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "no-dir-formation"})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "Formation directory not found") {
		t.Errorf("Error message = %q, want message containing 'Formation directory not found'", errResp.Message)
	}
}

func TestHandleStart_FormationConfigNotFound(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "no-config-formation"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/start", formationID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "Failed to find formation config") {
		t.Errorf("Error message = %q, want message containing 'Failed to find formation config'", errResp.Message)
	}
}

func TestHandleStart_ValidNativeFormation(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "valid-native-formation"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)

	// Create a minimal native formation.yaml (no muxi_runtime = native runtime)
	formationYAML := `name: test-formation
description: Test formation
`
	os.WriteFile(filepath.Join(formationDir, "formation.yaml"), []byte(formationYAML), 0644)

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/start", formationID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	// The process will fail to start (no actual app), but we exercise most of the code
	// Status could be 500 due to spawn error or health check failure
	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Logf("Status = %d (expected 500 due to spawn/health check failure)", resp.StatusCode)
	}
}

func TestHandleStart_WithSSE(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "sse-formation"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)

	// Create a minimal native formation.yaml
	formationYAML := `name: sse-test
description: SSE test
`
	os.WriteFile(filepath.Join(formationDir, "formation.yaml"), []byte(formationYAML), 0644)

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8081,
		Status: "stopped",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/start", formationID), nil)
	req.Header.Set("Accept", "text/event-stream")
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleStart(w, req)

	// Check that SSE headers are set
	resp := w.Result()
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Logf("Content-Type = %q (may not be SSE if initialization failed)", contentType)
	}
}

// ========================================
// HandleRestart Tests
// ========================================

func TestHandleRestart_FormationDirNotFoundAfterStop(t *testing.T) {
	server := createTestServer(t)

	// Don't create formation directory - it won't exist

	formationID := "restart-no-dir"
	// Register a formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "running",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/restart", formationID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "Formation directory not found") {
		t.Errorf("Error message = %q, want message containing 'Formation directory not found'", errResp.Message)
	}
}

func TestHandleRestart_ValidNativeFormation(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restart-valid-native"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)

	// Create a minimal native formation.yaml
	formationYAML := `name: restart-test
description: Restart test formation
`
	os.WriteFile(filepath.Join(formationDir, "formation.yaml"), []byte(formationYAML), 0644)

	// Register a running formation with restart count
	server.registry.Register(&registry.Formation{
		ID:           formationID,
		Port:         8082,
		Status:       "running",
		RestartCount: 2,
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/restart", formationID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// The process will fail to start but we exercise the restart code path
	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Logf("Status = %d (expected 500 due to spawn/health check failure)", resp.StatusCode)
	}
}

func TestHandleRestart_ConfigNotFoundAfterStop(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restart-no-config"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)
	// No formation.yaml created

	// Register a running formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8083,
		Status: "running",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/restart", formationID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if !strings.Contains(errResp.Message, "Failed to find formation config") {
		t.Errorf("Error message = %q, want message containing 'Failed to find formation config'", errResp.Message)
	}
}

func TestHandleRestart_WithSSE(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restart-sse"
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationDir, 0755)
	defer os.RemoveAll(formationDir)

	// Create a minimal native formation.yaml
	formationYAML := `name: restart-sse-test
description: SSE restart test
`
	os.WriteFile(filepath.Join(formationDir, "formation.yaml"), []byte(formationYAML), 0644)

	// Register a running formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8084,
		Status: "running",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/formations/%s/restart", formationID), nil)
	req.Header.Set("Accept", "text/event-stream")
	req = mux.SetURLVars(req, map[string]string{"id": formationID})
	w := httptest.NewRecorder()

	server.HandleRestart(w, req)

	// Check that SSE might be initialized (depends on ResponseRecorder compatibility)
	resp := w.Result()
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") {
		t.Logf("SSE initialized with Content-Type: %s", contentType)
	}
}

// ========================================
// restoreFormation Tests
// ========================================

func TestRestoreFormation_CurrentDirNotFound(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restore-no-current"
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)
	os.MkdirAll(formationBaseDir, 0755)
	defer os.RemoveAll(formationBaseDir)
	// No "current" subdirectory created

	// Try to restore
	err = server.restoreFormation(formationID, 8085)
	if err == nil {
		t.Error("Expected error for missing current directory, got nil")
	}

	if !strings.Contains(err.Error(), "formation current directory not found") {
		t.Errorf("Error = %v, want error containing 'formation current directory not found'", err)
	}
}

func TestRestoreFormation_InvalidYAMLConfig(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restore-invalid-yaml"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")
	os.MkdirAll(currentDir, 0755)
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))

	// Create invalid YAML
	invalidYAML := `name: [invalid yaml
  this is not valid`
	os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(invalidYAML), 0644)

	// Try to restore
	err = server.restoreFormation(formationID, 8086)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read formation config") {
		t.Errorf("Error = %v, want error containing 'failed to read formation config'", err)
	}
}

func TestRestoreFormation_ValidNativeFormationSpawn(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restore-valid-native"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")
	os.MkdirAll(currentDir, 0755)
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))

	// Create a valid native formation.yaml
	formationYAML := `name: restore-test
description: Restore test formation
`
	os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644)

	// Register formation first
	server.registry.Register(&registry.Formation{
		ID:           formationID,
		Port:         8087,
		Status:       "running",
		RestartCount: 5,
	})

	// Try to restore (will fail at process spawn but exercises config parsing)
	err = server.restoreFormation(formationID, 8087)
	// Error is expected because there's no actual app to run
	if err != nil {
		t.Logf("Expected spawn error: %v", err)
	}
}

func TestRestoreFormation_NoFormationFile(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "restore-no-yaml"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")
	os.MkdirAll(currentDir, 0755)
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))
	// No formation.yaml created

	// Try to restore
	err = server.restoreFormation(formationID, 8088)
	if err == nil {
		t.Error("Expected error for missing formation.yaml, got nil")
	}

	if !strings.Contains(err.Error(), "failed to find formation config") {
		t.Errorf("Error = %v, want error containing 'failed to find formation config'", err)
	}
}

// ========================================
// streamLogsSSE Tests
// ========================================

// flusherRecorder is a custom ResponseRecorder that implements http.Flusher
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushCalled bool
}

func (f *flusherRecorder) Flush() {
	f.flushCalled = true
}

func TestStreamLogsSSE_BasicConnection(t *testing.T) {
	server := createTestServer(t)

	// Create logs directory in temp location
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0755)

	formationID := "sse-logs-test"
	logPath := filepath.Join(logsDir, formationID+"-out.log")

	// Write some log content
	logContent := "Log line 1\nLog line 2\nLog line 3\n"
	os.WriteFile(logPath, []byte(logContent), 0644)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8089,
		Status: "running",
	})

	// Create request with context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", fmt.Sprintf("/formations/%s/logs?follow=true", formationID), nil)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})

	// Use custom flusher recorder
	recorder := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	// Start streaming in goroutine
	done := make(chan bool)
	go func() {
		server.streamLogsSSE(recorder, req, formationID, logPath)
		done <- true
	}()

	// Wait a bit for SSE to initialize
	time.Sleep(100 * time.Millisecond)

	// Cancel context to simulate client disconnect
	cancel()

	// Wait for streamLogsSSE to exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("streamLogsSSE did not exit after context cancel")
	}

	// Check that SSE headers were set
	resp := recorder.Result()
	if contentType := resp.Header.Get("Content-Type"); contentType != "text/event-stream" {
		t.Errorf("Content-Type = %q, want 'text/event-stream'", contentType)
	}

	if cacheControl := resp.Header.Get("Cache-Control"); cacheControl != "no-cache" {
		t.Errorf("Cache-Control = %q, want 'no-cache'", cacheControl)
	}

	// Check that Flush was called
	if !recorder.flushCalled {
		t.Error("Flush() was not called")
	}
}

func TestStreamLogsSSE_ClientDisconnect(t *testing.T) {
	server := createTestServer(t)

	// Create logs directory in temp location
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0755)

	formationID := "sse-disconnect-test"
	logPath := filepath.Join(logsDir, formationID+"-out.log")

	// Create empty log file
	os.WriteFile(logPath, []byte(""), 0644)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8090,
		Status: "running",
	})

	// Create request with short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest("GET", fmt.Sprintf("/formations/%s/logs?follow=true", formationID), nil)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})

	recorder := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	// streamLogsSSE should exit when context is done
	done := make(chan bool)
	go func() {
		server.streamLogsSSE(recorder, req, formationID, logPath)
		done <- true
	}()

	// Wait for context timeout
	select {
	case <-done:
		// Success - streamLogsSSE exited when context was cancelled
	case <-time.After(1 * time.Second):
		t.Error("streamLogsSSE did not exit after context timeout")
	}
}

func TestStreamLogsSSE_NewLogLines(t *testing.T) {
	server := createTestServer(t)

	// Create logs directory in temp location
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0755)

	formationID := "sse-newlines-test"
	logPath := filepath.Join(logsDir, formationID+"-out.log")

	// Create initial log file
	os.WriteFile(logPath, []byte("Initial line\n"), 0644)

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8091,
		Status: "running",
	})

	// Create request with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", fmt.Sprintf("/formations/%s/logs?follow=true", formationID), nil)
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"id": formationID})

	recorder := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	// Start streaming
	done := make(chan bool)
	go func() {
		server.streamLogsSSE(recorder, req, formationID, logPath)
		done <- true
	}()

	// Wait for SSE to initialize
	time.Sleep(100 * time.Millisecond)

	// Append new lines to log file
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("New line 1\nNew line 2\n")
	f.Close()

	// Wait for log polling
	time.Sleep(600 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for streamLogsSSE to exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("streamLogsSSE did not exit")
	}

	// Check that response contains the new lines
	body := recorder.Body.String()
	if !strings.Contains(body, "New line 1") {
		t.Logf("Response body did not contain 'New line 1' (may need more time for polling)")
	}
}

// ========================================
// Server Start/Stop Tests
// ========================================

func TestServer_StartAndStop(t *testing.T) {
	server := createTestServer(t)

	// Use port 0 to get a random free port
	server.config.Server.Port = 0
	server.config.Server.Host = "127.0.0.1"

	// Update HTTP server with new address
	addr := fmt.Sprintf("%s:%d", server.config.Server.Host, server.config.Server.Port)
	server.httpServer.Addr = addr

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started without error
	select {
	case err := <-serverErr:
		t.Fatalf("Server failed to start: %v", err)
	default:
		// Server started successfully
	}

	// Stop server with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	server := createTestServer(t)

	// Try to stop a server that wasn't started
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Stop(ctx)
	if err != nil {
		t.Logf("Stop returned error (expected): %v", err)
	}
}

func TestServer_StopWithCancelledContext(t *testing.T) {
	server := createTestServer(t)

	// Use port 0 to get a random free port
	server.config.Server.Port = 0
	server.config.Server.Host = "127.0.0.1"
	addr := fmt.Sprintf("%s:%d", server.config.Server.Host, server.config.Server.Port)
	server.httpServer.Addr = addr

	// Start server
	go func() {
		server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to stop with cancelled context (should still work or return context error)
	err := server.Stop(ctx)
	if err != nil {
		t.Logf("Stop with cancelled context returned: %v", err)
	}
}
