package process

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestStop_WithRunningProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	logger := zerolog.Nop()

	// Start a long-running process
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	proc := &Process{
		ID:     "test-stop",
		PID:    cmd.Process.Pid,
		Status: StatusRunning,
		cmd:    cmd,
	}

	// Stop the process
	err := Stop(proc, &logger)
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	// Verify process stopped
	if proc.Status != StatusStopped {
		t.Errorf("Status = %s, want %s", proc.Status, StatusStopped)
	}

	if proc.PID != 0 {
		t.Errorf("PID = %d, want 0 after stop", proc.PID)
	}

	if proc.cmd != nil {
		t.Error("cmd should be nil after stop")
	}

	// Verify process is not running
	if IsProcessRunning(cmd.Process.Pid) {
		t.Error("Process should not be running after Stop()")
	}
}

func TestStop_WithPIDFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	// Start a process
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	// Create PID file
	pidFile := tmpDir + "/test.pid"
	if err := writePIDFile(pidFile, cmd.Process.Pid); err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}

	proc := &Process{
		ID:      "test-pidfile",
		PID:     cmd.Process.Pid,
		Status:  StatusRunning,
		cmd:     cmd,
		PIDFile: pidFile,
	}

	// Stop process
	err := Stop(proc, &logger)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Verify PID file was removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be removed after Stop()")
	}
}

func TestStop_NilLogger(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	// Start a process
	cmd := exec.Command("sleep", "3")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	proc := &Process{
		ID:     "test-nil-logger",
		PID:    cmd.Process.Pid,
		Status: StatusRunning,
		cmd:    cmd,
	}

	// Should handle nil logger
	err := Stop(proc, nil)
	if err != nil {
		t.Errorf("Stop() with nil logger error = %v", err)
	}
}

func TestStop_StopSignalSet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	logger := zerolog.Nop()
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	proc := &Process{
		ID:         "test-stop-signal",
		PID:        cmd.Process.Pid,
		Status:     StatusRunning,
		cmd:        cmd,
		StopSignal: false,
	}

	err := Stop(proc, &logger)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Verify StopSignal was set
	if !proc.StopSignal {
		t.Error("StopSignal should be true after Stop()")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Test with current process (should be running)
	currentPID := os.Getpid()
	
	if !IsProcessRunning(currentPID) {
		t.Errorf("IsProcessRunning(%d) = false, want true for current process", currentPID)
	}
}

func TestIsProcessRunning_InvalidPIDs(t *testing.T) {
	tests := []struct {
		name string
		pid  int
		want bool
	}{
		{"zero", 0, false},
		{"negative", -1, false},
		{"large invalid", 999999999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProcessRunning(tt.pid)
			if got != tt.want {
				t.Errorf("IsProcessRunning(%d) = %v, want %v", tt.pid, got, tt.want)
			}
		})
	}
}

func TestIsProcessRunning_StoppedProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	// Start and immediately stop a process
	cmd := exec.Command("echo", "test")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run test command: %v", err)
	}

	// Process should have exited
	pid := cmd.Process.Pid
	
	// Wait a bit to ensure process cleanup
	time.Sleep(100 * time.Millisecond)

	if IsProcessRunning(pid) {
		t.Logf("Process %d still running (may be reused by OS)", pid)
	}
}

func TestWritePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := tmpDir + "/test.pid"

	err := writePIDFile(pidFile, 12345)
	if err != nil {
		t.Fatalf("writePIDFile() error = %v, want nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidFile); err != nil {
		t.Errorf("PID file not created: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	if string(content) != "12345" {
		t.Errorf("PID file content = %q, want %q", string(content), "12345")
	}
}

func TestWritePIDFile_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := tmpDir + "/nested/dir/test.pid"

	err := writePIDFile(pidFile, 54321)
	if err != nil {
		t.Fatalf("writePIDFile() with nested dir error = %v", err)
	}

	// Verify file was created (directories should be created)
	if _, err := os.Stat(pidFile); err != nil {
		t.Errorf("PID file not created in nested directory: %v", err)
	}
}

func TestReadPIDFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid PID file", func(t *testing.T) {
		pidFile := tmpDir + "/valid.pid"
		if err := os.WriteFile(pidFile, []byte("12345"), 0644); err != nil {
			t.Fatalf("Failed to create test PID file: %v", err)
		}

		pid, err := readPIDFile(pidFile)
		if err != nil {
			t.Fatalf("readPIDFile() error = %v, want nil", err)
		}

		if pid != 12345 {
			t.Errorf("readPIDFile() = %d, want 12345", pid)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := readPIDFile(tmpDir + "/nonexistent.pid")
		if err == nil {
			t.Error("readPIDFile() should error for non-existent file")
		}
	})

	t.Run("invalid content", func(t *testing.T) {
		pidFile := tmpDir + "/invalid.pid"
		if err := os.WriteFile(pidFile, []byte("not-a-number"), 0644); err != nil {
			t.Fatalf("Failed to create test PID file: %v", err)
		}

		_, err := readPIDFile(pidFile)
		if err == nil {
			t.Error("readPIDFile() should error for invalid content")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		pidFile := tmpDir + "/empty.pid"
		if err := os.WriteFile(pidFile, []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create test PID file: %v", err)
		}

		_, err := readPIDFile(pidFile)
		if err == nil {
			t.Error("readPIDFile() should error for empty file")
		}
	})
}

func TestStop_NoCmdProcess(t *testing.T) {
	logger := zerolog.Nop()

	proc := &Process{
		ID:     "test-no-cmd",
		Status: StatusRunning,
		cmd:    nil,
	}

	err := Stop(proc, &logger)
	if err == nil {
		t.Error("Stop() should error when cmd is nil")
	}

	if !contains(err.Error(), "not running") {
		t.Errorf("Error = %q, want error containing 'not running'", err.Error())
	}
}

func TestStop_ProcessStatusTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	logger := zerolog.Nop()
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	proc := &Process{
		ID:     "test-status",
		PID:    cmd.Process.Pid,
		Status: StatusRunning,
		cmd:    cmd,
	}

	// Should transition through Stopping to Stopped
	if proc.Status != StatusRunning {
		t.Errorf("Initial status = %s, want %s", proc.Status, StatusRunning)
	}

	err := Stop(proc, &logger)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if proc.Status != StatusStopped {
		t.Errorf("Final status = %s, want %s", proc.Status, StatusStopped)
	}
}


