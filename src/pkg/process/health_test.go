package process

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthChecker_Success(t *testing.T) {
	// Mock HTTP server returning 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Expected path /health, got %s", r.URL.Path)
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
