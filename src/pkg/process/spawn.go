package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

// SpawnConfig contains configuration for spawning a new process
type SpawnConfig struct {
	ID          string            // Formation ID
	Name        string            // Display name
	Command     string            // Executable (e.g., "python")
	Args        []string          // Arguments (e.g., ["test/dummy_app.py", "--port", "8001"])
	WorkDir     string            // Working directory
	Env         map[string]string // Environment variables
	Port        int               // Port number (for health checks)
	LogsDir     string            // Directory for logs
	PIDsDir     string            // Directory for PID files
	AutoRestart bool              // Enable auto-restart
	Logger      *zerolog.Logger   // Logger instance
}

// Spawn creates and starts a new process based on the configuration
// Returns a Process struct representing the running process
func Spawn(config SpawnConfig) (*Process, error) {
	logger := config.Logger
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	// Validate required fields
	if config.ID == "" {
		return nil, fmt.Errorf("process ID is required")
	}
	if config.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set defaults
	if config.Name == "" {
		config.Name = config.ID
	}
	if config.WorkDir == "" {
		config.WorkDir, _ = os.Getwd()
	}

	// Resolve executable path
	execPath, err := exec.LookPath(config.Command)
	if err != nil {
		return nil, fmt.Errorf("executable not found: %s: %w", config.Command, err)
	}

	logger.Debug().
		Str("id", config.ID).
		Str("command", execPath).
		Strs("args", config.Args).
		Msg("Spawning process")

	// Create log files
	logFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s-out.log", config.ID))
	errFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s-err.log", config.ID))
	pidFile := filepath.Join(config.PIDsDir, fmt.Sprintf("%s.pid", config.ID))

	// Open log files
	stdout, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout log: %w", err)
	}
	defer stdout.Close()

	stderr, err := os.OpenFile(errFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open stderr log: %w", err)
	}
	defer stderr.Close()

	nullFile, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer nullFile.Close()

	// Warn about Python buffering
	if strings.HasSuffix(execPath, "python") || strings.HasSuffix(execPath, "python3") {
		if len(config.Args) == 0 || config.Args[0] != "-u" {
			logger.Warn().
				Str("id", config.ID).
				Msg("Python processes should use -u flag to prevent output buffering")
		}
	}

	// Create command
	cmd := exec.Command(execPath, config.Args...)
	cmd.Dir = config.WorkDir
	cmd.Stdin = nullFile
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Set environment
	cmd.Env = os.Environ()
	for key, value := range config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set process group (allows killing entire process tree)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	pid := cmd.Process.Pid

	// Write PID to file
	if err := writePIDFile(pidFile, pid); err != nil {
		logger.Error().
			Err(err).
			Str("id", config.ID).
			Int("pid", pid).
			Msg("Failed to write PID file")
		// Don't fail the spawn for this
	}

	logger.Info().
		Str("id", config.ID).
		Str("name", config.Name).
		Int("pid", pid).
		Msg("✓ Process spawned")

	// Build health check URL if port provided
	healthCheckURL := ""
	if config.Port > 0 {
		healthCheckURL = fmt.Sprintf("http://localhost:%d/health", config.Port)
	}

	// Create Process struct
	process := &Process{
		ID:             config.ID,
		Name:           config.Name,
		PID:            pid,
		Command:        execPath,
		Args:           config.Args,
		WorkDir:        config.WorkDir,
		Status:         StatusStarting,
		StartedAt:      now(),
		RestartCount:   0,
		StopSignal:     false,
		PIDFile:        pidFile,
		LogFile:        logFile,
		ErrorFile:      errFile,
		AutoRestart:    config.AutoRestart,
		MaxRestarts:    10, // Default
		HealthCheckURL: healthCheckURL,
		cmd:            cmd,
	}

	return process, nil
}

// Stop stops a running process gracefully
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

	proc.Status = StatusStopping
	proc.StopSignal = true

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

	proc.Status = StatusStopped
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

// IsProcessRunning checks if a process with the given PID is running
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

// Helper functions

func writePIDFile(path string, pid int) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write PID
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pidStr := strings.TrimSpace(string(data))
	return strconv.Atoi(pidStr)
}

// now returns current time (abstracted for testing)
var now = func() time.Time {
	return time.Now()
}
