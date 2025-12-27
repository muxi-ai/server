package process

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHealthChecker_Success(t *testing.T) {
	// Mock HTTP server returning 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Errorf("Expected path /v1/health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	// Extract port from server URL
	port := server.Listener.Addr().(*net.TCPAddr).Port

	// Create health checker
	hc := NewHealthChecker(5*time.Second, 100*time.Millisecond)

	// Check health - should succeed quickly
	err := hc.WaitForHealthy(port, "test-formation")
	if err != nil {
		t.Fatalf("Expected health check to succeed, got error: %v", err)
	}
}

func TestHealthChecker_Timeout(t *testing.T) {
	// Mock HTTP server returning 500 (unhealthy)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status": "unhealthy"}`))
	}))
	defer server.Close()

	// Extract port from server URL
	port := server.Listener.Addr().(*net.TCPAddr).Port

	// Create health checker with short timeout
	hc := NewHealthChecker(1*time.Second, 100*time.Millisecond)

	// Check health - should timeout
	err := hc.WaitForHealthy(port, "test-formation")
	if err == nil {
		t.Fatal("Expected health check to fail, but it succeeded")
	}

	if !strings.Contains(err.Error(), "health check failed") {
		t.Errorf("Expected error about health check failure, got: %v", err)
	}
}

func TestHealthChecker_SlowStart(t *testing.T) {
	// Mock HTTP server that becomes healthy after a delay
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 5 {
			// First 4 requests fail
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// 5th request succeeds
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	// Extract port from server URL
	port := server.Listener.Addr().(*net.TCPAddr).Port

	// Create health checker
	hc := NewHealthChecker(5*time.Second, 100*time.Millisecond)

	// Check health - should eventually succeed
	err := hc.WaitForHealthy(port, "test-formation")
	if err != nil {
		t.Fatalf("Expected health check to succeed after retries, got error: %v", err)
	}

	if requestCount < 5 {
		t.Errorf("Expected at least 5 requests, got %d", requestCount)
	}
}

func TestHealthChecker_ServerDown(t *testing.T) {
	// Use a port that's not listening
	port := 55555

	// Create health checker with short timeout
	hc := NewHealthChecker(1*time.Second, 100*time.Millisecond)

	// Check health - should fail immediately (connection refused)
	err := hc.WaitForHealthy(port, "test-formation")
	if err == nil {
		t.Fatal("Expected health check to fail when server is down")
	}
}

func TestHealthChecker_CustomEndpoint(t *testing.T) {
	// Mock HTTP server with custom health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			t.Errorf("Expected path /api/health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract port from server URL
	port := server.Listener.Addr().(*net.TCPAddr).Port

	// Create health checker with custom endpoint
	hc := NewHealthChecker(5*time.Second, 100*time.Millisecond)
	hc.Endpoint = "/api/health"

	// Check health
	err := hc.WaitForHealthy(port, "test-formation")
	if err != nil {
		t.Fatalf("Expected health check to succeed, got error: %v", err)
	}
}

func TestHealthChecker_201Created(t *testing.T) {
	// Some services return 201 Created (e.g., after initialization)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port

	hc := NewHealthChecker(5*time.Second, 100*time.Millisecond)

	// Should accept 201 as healthy (any 2xx)
	err := hc.WaitForHealthy(port, "test-formation")
	if err != nil {
		t.Fatalf("Expected 201 to be considered healthy, got error: %v", err)
	}
}

func TestHealthChecker_404NotFound(t *testing.T) {
	// Service is running but health endpoint doesn't exist
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port

	hc := NewHealthChecker(1*time.Second, 100*time.Millisecond)

	// Should fail (404 is not healthy)
	err := hc.WaitForHealthy(port, "test-formation")
	if err == nil {
		t.Fatal("Expected 404 to be considered unhealthy")
	}
}

func TestTrimWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"\t\nhello\n\t", "hello"},
		{"  \t\n  ", ""},
		{"\r\n  test  \r\n", "test"},
	}

	for _, tt := range tests {
		got := trimWhitespace(tt.input)
		if got != tt.want {
			t.Errorf("trimWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestReadLastLines(t *testing.T) {
	// Create temp file
	tmpFile := t.TempDir() + "/test.log"
	content := "line1\nline2\nline3\nline4\nline5"
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read last 3 lines
	result, err := readLastLines(tmpFile, 3)
	if err != nil {
		t.Fatalf("readLastLines error: %v", err)
	}

	if !strings.Contains(result, "line3") || !strings.Contains(result, "line4") || !strings.Contains(result, "line5") {
		t.Errorf("readLastLines(3) = %q, should contain last 3 lines", result)
	}

	// Read more lines than exist
	result, err = readLastLines(tmpFile, 10)
	if err != nil {
		t.Fatalf("readLastLines error: %v", err)
	}
	if !strings.Contains(result, "line1") {
		t.Error("Should contain all lines when n > total lines")
	}

	// Non-existent file
	_, err = readLastLines("/nonexistent/file.log", 5)
	if err == nil {
		t.Error("Should error on non-existent file")
	}
}

func TestExtractErrorSection(t *testing.T) {
	tmpFile := t.TempDir() + "/error.log"

	// Test with FAIL marker
	content := "Starting...\n[ FAIL ] Error message here\nMore details"
	writeTestFile(tmpFile, content)

	result, err := extractErrorSection(tmpFile)
	if err != nil {
		t.Fatalf("extractErrorSection error: %v", err)
	}
	if !strings.Contains(result, "Error message here") {
		t.Errorf("Should extract content after FAIL marker, got: %q", result)
	}

	// Test with separator
	content2 := "Old run\n====================================================================\nNew run\n[ FAIL ] New error"
	writeTestFile(tmpFile, content2)

	result, err = extractErrorSection(tmpFile)
	if err != nil {
		t.Fatalf("extractErrorSection error: %v", err)
	}
	if strings.Contains(result, "Old run") {
		t.Error("Should only look at content after last separator")
	}

	// Test with no markers
	content3 := "Just normal logs\nNo errors here"
	writeTestFile(tmpFile, content3)

	result, err = extractErrorSection(tmpFile)
	if err != nil {
		t.Fatalf("extractErrorSection error: %v", err)
	}
	if result != "" {
		t.Errorf("Should return empty when no error marker, got: %q", result)
	}

	// Non-existent file
	_, err = extractErrorSection("/nonexistent/file.log")
	if err == nil {
		t.Error("Should error on non-existent file")
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
