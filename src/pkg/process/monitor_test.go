package process

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewMonitor(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:   "test",
		PID:  12345,
		Name: "Test Process",
	}

	monitor := NewMonitor(proc, &logger)

	if monitor == nil {
		t.Fatal("NewMonitor() returned nil")
	}

	if monitor.process != proc {
		t.Error("Monitor process not set correctly")
	}

	if monitor.logger == nil {
		t.Error("Monitor logger not set")
	}

	if monitor.stopChan == nil {
		t.Error("Monitor stopChan not initialized")
	}
}

func TestNewMonitor_NilLogger(t *testing.T) {
	proc := &Process{ID: "test"}

	// Should handle nil logger gracefully
	monitor := NewMonitor(proc, nil)

	if monitor == nil {
		t.Fatal("NewMonitor() with nil logger returned nil")
	}

	if monitor.logger == nil {
		t.Error("Monitor should have a logger even when nil is passed")
	}
}

func TestMonitor_OnCrash(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{ID: "test"}
	monitor := NewMonitor(proc, &logger)

	called := false
	monitor.OnCrash(func(p *Process) {
		called = true
	})

	if monitor.onCrash == nil {
		t.Error("OnCrash callback not set")
	}

	// Test the callback
	if monitor.onCrash != nil {
		monitor.onCrash(proc)
		if !called {
			t.Error("OnCrash callback was not called")
		}
	}
}

func TestMonitor_OnHealthy(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{ID: "test"}
	monitor := NewMonitor(proc, &logger)

	called := false
	monitor.OnHealthy(func(p *Process) {
		called = true
	})

	if monitor.onHealthy == nil {
		t.Error("OnHealthy callback not set")
	}

	// Test the callback
	if monitor.onHealthy != nil {
		monitor.onHealthy(proc)
		if !called {
			t.Error("OnHealthy callback was not called")
		}
	}
}

func TestMonitor_Start_Stop(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:     "test",
		PID:    99999, // Non-existent PID
		Status: StatusRunning,
	}

	monitor := NewMonitor(proc, &logger)

	// Start monitoring
	monitor.Start()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop monitoring
	monitor.Stop()

	// Should not panic or hang
	time.Sleep(100 * time.Millisecond)
}

func TestMonitor_WithHealthCheck(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:             "test",
		PID:            99999,
		Status:         StatusStarting,
		HealthCheckURL: "http://localhost:9999/nonexistent",
	}

	monitor := NewMonitor(proc, &logger)

	healthyCalled := false
	monitor.OnHealthy(func(p *Process) {
		healthyCalled = true
	})

	monitor.Start()

	// Wait for initial health check attempt
	time.Sleep(3 * time.Second)

	// Health check should fail (no server at that URL)
	// So healthyCalled should be false
	if healthyCalled {
		t.Log("Health callback was called (unexpected for nonexistent server)")
	}

	monitor.Stop()
}

func TestMonitor_ProcessCrash(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:     "test",
		PID:    99999, // Non-existent PID
		Status: StatusRunning,
	}

	monitor := NewMonitor(proc, &logger)

	var mu sync.Mutex
	crashCalled := false
	crashedProc := (*Process)(nil)
	monitor.OnCrash(func(p *Process) {
		mu.Lock()
		defer mu.Unlock()
		crashCalled = true
		crashedProc = p
	})

	monitor.Start()

	// Wait for monitor to detect the "crash"
	time.Sleep(6 * time.Second)

	monitor.Stop()

	// Should have detected crash
	mu.Lock()
	called := crashCalled
	crashed := crashedProc
	mu.Unlock()

	if !called {
		t.Error("OnCrash callback should have been called for non-running process")
	}

	if crashed != proc {
		t.Error("Crashed process should match original process")
	}

	if proc.GetStatus() != StatusCrashed {
		t.Errorf("Process status = %s, want %s", proc.GetStatus(), StatusCrashed)
	}
}

func TestMonitor_IntentionalStop(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:         "test",
		PID:        99999,
		Status:     StatusRunning,
		StopSignal: true, // Marked as intentionally stopped
	}

	monitor := NewMonitor(proc, &logger)

	var mu sync.Mutex
	crashCalled := false
	monitor.OnCrash(func(p *Process) {
		mu.Lock()
		defer mu.Unlock()
		crashCalled = true
	})

	monitor.Start()

	// Wait for monitor check
	time.Sleep(6 * time.Second)

	monitor.Stop()

	// Should NOT call crash callback for intentional stop
	mu.Lock()
	called := crashCalled
	mu.Unlock()

	if called {
		t.Error("OnCrash should not be called for intentional stop")
	}

	if proc.GetStatus() != StatusStopped {
		t.Logf("Process status = %s (expected stopped for intentional stop)", proc.GetStatus())
	}
}

func TestWaitForExit_NilCmd(t *testing.T) {
	proc := &Process{
		ID:  "test",
		cmd: nil,
	}

	err := WaitForExit(proc)
	if err == nil {
		t.Error("WaitForExit() should return error for nil cmd")
	}

	if !containsStr(err.Error(), "not running") {
		t.Errorf("Error = %q, want error containing 'not running'", err.Error())
	}
}

func TestMonitor_Multiple(t *testing.T) {
	// Test multiple monitors running concurrently
	logger := zerolog.Nop()

	monitors := make([]*Monitor, 5)
	for i := 0; i < 5; i++ {
		proc := &Process{
			ID:     fmt.Sprintf("test-%d", i),
			PID:    99990 + i,
			Status: StatusRunning,
		}
		monitors[i] = NewMonitor(proc, &logger)
		monitors[i].Start()
	}

	// Let them run
	time.Sleep(100 * time.Millisecond)

	// Stop all
	for _, mon := range monitors {
		mon.Stop()
	}

	// Should not cause data races or panics
	time.Sleep(100 * time.Millisecond)
}

func TestMonitor_Callbacks(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{ID: "test"}
	monitor := NewMonitor(proc, &logger)

	// Test setting multiple callbacks
	count := 0

	monitor.OnCrash(func(p *Process) {
		count++
	})

	monitor.OnHealthy(func(p *Process) {
		count++
	})

	// Simulate calling both callbacks
	if monitor.onCrash != nil {
		monitor.onCrash(proc)
	}

	if monitor.onHealthy != nil {
		monitor.onHealthy(proc)
	}

	if count != 2 {
		t.Errorf("Callbacks called %d times, want 2", count)
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Test with obviously invalid PIDs
	tests := []struct {
		name string
		pid  int
		want bool
	}{
		{"zero PID", 0, false},
		{"negative PID", -1, false},
		{"very large PID", 999999999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProcessRunning(tt.pid)
			if got != tt.want {
				t.Errorf("IsProcessRunning(%d) = %v, want %v", tt.pid, got, tt.want)
			}
		})
	}

	// Test with current process PID (should be running)
	currentPID := os.Getpid()
	if !IsProcessRunning(currentPID) {
		t.Error("IsProcessRunning() should return true for current process")
	}
}

func TestMonitor_NoHealthCheck(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:             "test-no-health",
		PID:            99999,
		Status:         StatusStarting,
		HealthCheckURL: "", // No health check URL
	}

	monitor := NewMonitor(proc, &logger)

	monitor.Start()

	// Without health check URL, should immediately transition to running
	time.Sleep(100 * time.Millisecond)

	if proc.GetStatus() != StatusRunning {
		t.Errorf("Status = %s, want %s (should auto-transition without health check)", proc.GetStatus(), StatusRunning)
	}

	monitor.Stop()
}

func TestMonitor_HealthCheck_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check test")
	}

	// Start a simple HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	proc := &Process{
		ID:             "health-test",
		HealthCheckURL: server.URL,
		Status:         StatusStarting,
	}

	monitor := NewMonitor(proc, &logger)
	err := monitor.healthCheck()

	if err != nil {
		t.Errorf("healthCheck() error = %v, want nil", err)
	}
}

func TestMonitor_HealthCheck_ServerError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check test")
	}

	// Server returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	proc := &Process{
		ID:             "health-error",
		HealthCheckURL: server.URL,
	}

	monitor := NewMonitor(proc, &logger)
	err := monitor.healthCheck()

	if err == nil {
		t.Error("healthCheck() should return error for 500 status")
	}

	if !contains(err.Error(), "500") {
		t.Errorf("Error = %q, should mention status code", err.Error())
	}
}

func TestMonitor_HealthCheck_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check test")
	}

	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Longer than health check timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	proc := &Process{
		ID:             "health-timeout",
		HealthCheckURL: server.URL,
	}

	monitor := NewMonitor(proc, &logger)

	start := time.Now()
	err := monitor.healthCheck()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("healthCheck() should timeout")
	}

	// Should timeout in ~5 seconds, not wait for 10
	if elapsed > 7*time.Second {
		t.Errorf("healthCheck took %v, should timeout around 5s", elapsed)
	}
}

func TestMonitor_HealthCheck_InvalidURL(t *testing.T) {
	logger := zerolog.Nop()
	proc := &Process{
		ID:             "health-invalid",
		HealthCheckURL: "http://invalid-host-that-does-not-exist-12345.com",
	}

	monitor := NewMonitor(proc, &logger)
	err := monitor.healthCheck()

	if err == nil {
		t.Error("healthCheck() should error for invalid host")
	}
}

func TestMonitor_HealthCheck_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health check test")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	proc := &Process{
		ID:             "health-404",
		HealthCheckURL: server.URL,
	}

	monitor := NewMonitor(proc, &logger)
	err := monitor.healthCheck()

	if err == nil {
		t.Error("healthCheck() should error for 404 status")
	}
}
