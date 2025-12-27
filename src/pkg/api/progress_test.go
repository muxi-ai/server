package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSSEWriter_Success(t *testing.T) {
	rec := httptest.NewRecorder()

	sse, ok := NewSSEWriter(rec)
	if !ok {
		t.Fatal("NewSSEWriter should succeed with ResponseRecorder")
	}
	if sse == nil {
		t.Fatal("SSEWriter should not be nil")
	}
}

func TestSSEWriter_Init(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)

	sse.Init()

	// Check headers
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	if conn := rec.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
}

func TestSSEWriter_SendProgress(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)
	sse.Init()

	event := ProgressEvent{
		Stage:   StageExtracting,
		Message: "Extracting bundle...",
	}
	err := sse.SendProgress(event)
	if err != nil {
		t.Fatalf("SendProgress() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: progress") {
		t.Error("Response should contain 'event: progress'")
	}
	if !strings.Contains(body, "extracting") {
		t.Error("Response should contain stage name")
	}
	if !strings.Contains(body, "Extracting bundle") {
		t.Error("Response should contain message")
	}
}

func TestSSEWriter_SendComplete(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)
	sse.Init()

	event := CompleteEvent{
		FormationID: "test-formation",
		Port:        8080,
		Status:      "running",
		PID:         12345,
	}
	err := sse.SendComplete(event)
	if err != nil {
		t.Fatalf("SendComplete() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: complete") {
		t.Error("Response should contain 'event: complete'")
	}
	if !strings.Contains(body, "test-formation") {
		t.Error("Response should contain formation ID")
	}
	if !strings.Contains(body, "8080") {
		t.Error("Response should contain port")
	}
}

func TestSSEWriter_SendError(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)
	sse.Init()

	event := ErrorEvent{
		Error:   "DeploymentFailed",
		Message: "Health check timed out",
		Stage:   StageHealthCheck,
	}
	err := sse.SendError(event)
	if err != nil {
		t.Fatalf("SendError() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Error("Response should contain 'event: error'")
	}
	if !strings.Contains(body, "DeploymentFailed") {
		t.Error("Response should contain error type")
	}
}

func TestProgressEmitter_Disabled(t *testing.T) {
	emitter := NewProgressEmitter(nil)

	if emitter.IsEnabled() {
		t.Error("Emitter should be disabled when SSE is nil")
	}

	// These should not panic when disabled
	emitter.Emit(ProgressEvent{Stage: StageExtracting})
	emitter.Complete(CompleteEvent{FormationID: "test"})
	emitter.Error(ErrorEvent{Error: "test"})
}

func TestProgressEmitter_Enabled(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)
	sse.Init()

	emitter := NewProgressEmitter(sse)

	if !emitter.IsEnabled() {
		t.Error("Emitter should be enabled when SSE is provided")
	}

	emitter.Emit(ProgressEvent{Stage: StageValidating, Message: "Validating..."})
	emitter.Complete(CompleteEvent{FormationID: "test", Port: 8080})

	body := rec.Body.String()
	if !strings.Contains(body, "progress") {
		t.Error("Should contain progress event")
	}
	if !strings.Contains(body, "complete") {
		t.Error("Should contain complete event")
	}
}

func TestProgressEmitter_Error(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, _ := NewSSEWriter(rec)
	sse.Init()

	emitter := NewProgressEmitter(sse)
	emitter.Error(ErrorEvent{Error: "TestError", Message: "Test message", Stage: StageSpawning})

	body := rec.Body.String()
	if !strings.Contains(body, "error") {
		t.Error("Should contain error event")
	}
	if !strings.Contains(body, "TestError") {
		t.Error("Should contain error type")
	}
}
