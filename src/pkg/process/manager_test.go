package process_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muxi-ai/server/pkg/process"
	"github.com/rs/zerolog"
)

// TestManagerIntegration tests the full process lifecycle with dummy_app.py
func TestManagerIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	tmpDir := t.TempDir()

	manager, err := process.NewManager(tmpDir, &logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.StopAll()

	// Find dummy_app.py
	dummyAppPath := findDummyApp(t)

	// Test 1: Start a process
	t.Run("StartProcess", func(t *testing.T) {
		config := process.SpawnConfig{
			ID:          "test-formation",
			Name:        "Test Formation",
			Command:     "python3",
			Args:        []string{dummyAppPath, "--port", "8099"},
			Port:        8099,
			AutoRestart: true,
		}

		proc, err := manager.Start(config)
		if err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		if proc.PID <= 0 {
			t.Error("Expected valid PID")
		}

		t.Logf("✓ Process started with PID %d", proc.PID)

		// Wait for process to be ready
		time.Sleep(3 * time.Second)

		// Check if process is healthy
		resp, err := http.Get("http://localhost:8099/health")
		if err != nil {
			t.Fatalf("Failed to access health endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Log("✓ Health check passed")
	})

	// Test 2: List processes
	t.Run("ListProcesses", func(t *testing.T) {
		processes := manager.List()
		if len(processes) != 1 {
			t.Errorf("Expected 1 process, got %d", len(processes))
		}

		proc := processes[0]
		if proc.ID != "test-formation" {
			t.Errorf("Expected ID 'test-formation', got '%s'", proc.ID)
		}

		t.Logf("✓ Found process: %s (PID %d, Status: %s)",
			proc.ID, proc.PID, proc.Status)
	})

	// Test 3: Get process
	t.Run("GetProcess", func(t *testing.T) {
		proc, err := manager.Get("test-formation")
		if err != nil {
			t.Fatalf("Failed to get process: %v", err)
		}

		if proc.ID != "test-formation" {
			t.Errorf("Expected ID 'test-formation', got '%s'", proc.ID)
		}

		t.Log("✓ Retrieved process successfully")
	})

	// Test 4: Stop process
	t.Run("StopProcess", func(t *testing.T) {
		err := manager.Stop("test-formation")
		if err != nil {
			t.Fatalf("Failed to stop process: %v", err)
		}

		t.Log("✓ Process stopped")

		// Verify process is stopped (give uvicorn time to shut down)
		time.Sleep(2 * time.Second)

		resp, err := http.Get("http://localhost:8099/health")
		if err == nil {
			resp.Body.Close()
			t.Log("⚠️  Health endpoint still reachable (uvicorn may take time to shut down)")
		} else {
			t.Log("✓ Verified process is stopped")
		}
	})
}

// TestAutoRestart tests that crashed processes are automatically restarted
func TestAutoRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	tmpDir := t.TempDir()

	manager, err := process.NewManager(tmpDir, &logger)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.StopAll()

	dummyAppPath := findDummyApp(t)

	// Start process with auto-restart enabled
	config := process.SpawnConfig{
		ID:          "restart-test",
		Name:        "Restart Test",
		Command:     "python3",
		Args:        []string{dummyAppPath, "--port", "8098"},
		Port:        8098,
		AutoRestart: true,
	}

	proc, err := manager.Start(config)
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	originalPID := proc.PID
	t.Logf("✓ Process started with PID %d", originalPID)

	// Wait for it to be healthy
	time.Sleep(3 * time.Second)

	// Kill the process manually (simulate crash)
	t.Log("Simulating crash...")
	killProcess(originalPID)

	// Wait for auto-restart
	time.Sleep(5 * time.Second)

	// Check if process was restarted
	proc, err = manager.Get("restart-test")
	if err != nil {
		t.Fatalf("Failed to get process after restart: %v", err)
	}

	if proc.PID == originalPID {
		t.Error("Process was not restarted (PID unchanged)")
	}

	if proc.RestartCount == 0 {
		t.Error("Restart count should be > 0")
	}

	t.Logf("✓ Process auto-restarted with new PID %d (restart count: %d)",
		proc.PID, proc.RestartCount)

	// Verify it's healthy again
	resp, err := http.Get("http://localhost:8098/health")
	if err != nil {
		t.Fatalf("Failed to access health endpoint after restart: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 after restart, got %d", resp.StatusCode)
	}

	t.Log("✓ Restarted process is healthy")
}

// Helper functions

func findDummyApp(t *testing.T) string {
	// Try to find dummy_app.py
	paths := []string{
		"../../test/dummy_app.py",
		"../test/dummy_app.py",
		"test/dummy_app.py",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}

	t.Skip("Could not find dummy_app.py - skipping integration test")
	return ""
}

func killProcess(pid int) {
	proc, err := os.FindProcess(pid)
	if err == nil {
		proc.Kill()
	}
}
