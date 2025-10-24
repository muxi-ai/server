//go:build unix

package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/rs/zerolog"
)

// setupPlatformProcess configures platform-specific process attributes for Unix systems
func setupPlatformProcess(cmd interface{}) error {
	// Type assertion to *exec.Cmd (we know it is, but need to import os/exec)
	if c, ok := cmd.(*exec.Cmd); ok {
		// Set process group (allows killing entire process tree)
		c.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		return nil
	}
	return fmt.Errorf("invalid command type")
}

// Stop stops a running process gracefully on Unix systems
func Stop(proc *Process, logger *zerolog.Logger) error {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	logger.Info().
		Str("id", proc.ID).
		Int("pid", proc.PID).
		Msg("Stopping process")

	proc.SetStatus(StatusStopping)
	proc.SetStopSignal(true)

	// Send SIGTERM for graceful shutdown
	if err := proc.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logger.Warn().
			Err(err).
			Str("id", proc.ID).
			Msg("Failed to send SIGTERM, forcing kill")
		// Try SIGKILL
		if err := proc.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	// Wait for process to exit (with timeout handled by caller)
	if err := proc.cmd.Wait(); err != nil {
		// Exit error is expected
		logger.Debug().
			Err(err).
			Str("id", proc.ID).
			Msg("Process exited")
	}

	proc.SetStatus(StatusStopped)
	proc.PID = 0
	proc.cmd = nil

	// Clean up PID file
	if err := os.Remove(proc.PIDFile); err != nil {
		logger.Debug().
			Err(err).
			Str("id", proc.ID).
			Msg("Failed to remove PID file")
	}

	logger.Info().
		Str("id", proc.ID).
		Msg("✓ Process stopped")

	return nil
}

// IsProcessRunning checks if a process with the given PID is running on Unix systems
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
