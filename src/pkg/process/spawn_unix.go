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

	// For singularity runtime (Docker-wrapped on macOS), clean up container
	// This handles cases where the docker run process is killed but container remains
	if proc.RuntimeType == "singularity" && proc.Port > 0 {
		CleanupDockerContainer(proc.ID, proc.Port, logger)
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

// ForceKill forcefully kills a process using SIGKILL on Unix systems
func ForceKill(proc *Process, logger *zerolog.Logger) error {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	logger.Warn().
		Str("id", proc.ID).
		Int("pid", proc.PID).
		Msg("Force killing process with SIGKILL")

	proc.SetStatus(StatusStopping)
	proc.SetStopSignal(true)

	// Send SIGKILL to process group (kills entire process tree)
	pgid := -proc.PID // Negative PID targets process group
	if err := syscall.Kill(pgid, syscall.SIGKILL); err != nil {
		// If process group kill fails, try direct kill
		logger.Warn().Err(err).Msg("Failed to kill process group, trying direct kill")
		if err := proc.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to force kill process: %w", err)
		}
	}

	// Wait for process to exit
	if err := proc.cmd.Wait(); err != nil {
		// Exit error is expected with SIGKILL
		logger.Debug().Err(err).Str("id", proc.ID).Msg("Process killed")
	}

	// For singularity runtime (Docker-wrapped on macOS), clean up container
	// This handles cases where the docker run process is killed but container remains
	if proc.RuntimeType == "singularity" && proc.Port > 0 {
		CleanupDockerContainer(proc.ID, proc.Port, logger)
	}

	proc.SetStatus(StatusStopped)
	proc.PID = 0
	proc.cmd = nil

	// Clean up PID file
	if err := os.Remove(proc.PIDFile); err != nil {
		logger.Debug().Err(err).Str("id", proc.ID).Msg("Failed to remove PID file")
	}

	logger.Info().Str("id", proc.ID).Msg("✓ Process force killed")

	return nil
}

// IsProcessRunning checks if a process with the given PID is running on Unix systems
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// First check if process exists using signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	// Check if it's a zombie by trying to wait for it with WNOHANG
	// If Wait4 returns the pid, the process has exited (zombie or otherwise)
	var status syscall.WaitStatus
	wpid, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err == nil && wpid == pid {
		// Process has exited (was zombie, now reaped)
		return false
	}
	// wpid == 0 means process is still running
	// wpid == -1 or error means it's not our child (can't wait on it), assume running
	return true
}
