package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// SpawnConfig contains configuration for spawning a new process
type SpawnConfig struct {
	ID          string            // Formation ID
	Name        string            // Display name
	Command     string            // Executable (e.g., "python" or "singularity")
	Args        []string          // Arguments (e.g., ["test/dummy_app.py", "--port", "8001"])
	WorkDir     string            // Working directory
	Env         map[string]string // Environment variables
	Port        int               // Port number (for health checks)
	LogsDir     string            // Directory for logs
	PIDsDir     string            // Directory for PID files
	AutoRestart bool              // Enable auto-restart
	Logger      *zerolog.Logger   // Logger instance

	// SIF Runtime support
	RuntimeType string // "native" or "singularity"
	SIFPath     string // Path to SIF file (if RuntimeType is "singularity")

	// Skip initial health check in monitor (used when deploy does its own health check)
	SkipInitialHealthCheck bool

	// Truncate log files on start (true for new deploys, false for updates/restarts)
	TruncateLogs bool
}

// Spawn creates and starts a new process based on the configuration
// Returns a Process struct representing the running process
//
// This is the main entry point. Platform-specific implementations are in:
// - spawn_unix.go (Linux, macOS, BSD)
// - spawn_windows.go (Windows)
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

	// For singularity runtime, we don't need to resolve the command on the host
	// since it will run inside the container
	var execPath string
	if config.RuntimeType != "singularity" {
		var err error
		execPath, err = exec.LookPath(config.Command)
		if err != nil {
			return nil, fmt.Errorf("executable not found: %s: %w", config.Command, err)
		}
	}

	logger.Debug().
		Str("id", config.ID).
		Str("command", config.Command).
		Strs("args", config.Args).
		Str("runtime", config.RuntimeType).
		Msg("Spawning process")

	// Create log files
	logFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s-out.log", config.ID))
	errFile := filepath.Join(config.LogsDir, fmt.Sprintf("%s-err.log", config.ID))
	pidFile := filepath.Join(config.PIDsDir, fmt.Sprintf("%s.pid", config.ID))

	// Open log files (truncate on new deploy, append on update/restart)
	logFlags := os.O_WRONLY | os.O_CREATE
	if config.TruncateLogs {
		logFlags |= os.O_TRUNC
	} else {
		logFlags |= os.O_APPEND
	}
	stdout, err := os.OpenFile(logFile, logFlags, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout log: %w", err)
	}
	defer stdout.Close()

	stderr, err := os.OpenFile(errFile, logFlags, 0640)
	if err != nil {
		return nil, fmt.Errorf("failed to open stderr log: %w", err)
	}
	defer stderr.Close()

	nullFile, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("failed to open null device: %w", err)
	}
	defer nullFile.Close()

	// Warn about Python buffering (only for native Python, not container runtimes)
	if config.RuntimeType != "singularity" {
		if strings.HasSuffix(execPath, "python") || strings.HasSuffix(execPath, "python3") {
			if len(config.Args) == 0 || config.Args[0] != "-u" {
				logger.Warn().
					Str("id", config.ID).
					Msg("Python processes should use -u flag to prevent output buffering")
			}
		}
	}

	// Build command based on runtime type
	var cmd *exec.Cmd
	if config.RuntimeType == "singularity" {
		// Validate SIF path
		if config.SIFPath == "" {
			return nil, fmt.Errorf("SIF path required for singularity runtime")
		}
		if _, err := os.Stat(config.SIFPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SIF file not found: %s", config.SIFPath)
		}

		// Platform-specific execution
		if runtime.GOOS == "linux" {
			// Native Singularity execution on Linux
			cmd = buildNativeSingularityCommand(config, logger)
		} else {
			// Docker wrapper for macOS/Windows
			// Clean up any existing containers first (handles orphans from crashes/restarts)
			CleanupDockerContainer(config.ID, config.Port, logger)
			cmd = buildDockerSingularityCommand(config, logger)
		}
	} else {
		// Native execution (original behavior)
		cmd = exec.Command(execPath, config.Args...)

		logger.Debug().
			Str("id", config.ID).
			Str("command", execPath).
			Strs("args", config.Args).
			Msg("Spawning native process")
	}

	cmd.Dir = config.WorkDir
	cmd.Stdin = nullFile
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Set environment (inherit parent environment)
	cmd.Env = os.Environ()

	// For native processes, add custom env vars
	// For singularity, env vars are already passed via --env flags
	if config.RuntimeType != "singularity" {
		for key, value := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Platform-specific process setup (process groups, job objects, etc.)
	if err := setupPlatformProcess(cmd); err != nil {
		return nil, fmt.Errorf("failed to setup platform-specific process: %w", err)
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
	// Runtime exposes health at /v1/health
	healthCheckURL := ""
	if config.Port > 0 {
		healthCheckURL = fmt.Sprintf("http://localhost:%d/v1/health", config.Port)
	}

	// Create Process struct
	process := &Process{
		ID:                     config.ID,
		Name:                   config.Name,
		PID:                    pid,
		Command:                execPath,
		Args:                   config.Args,
		WorkDir:                config.WorkDir,
		Status:                 StatusStarting,
		StartedAt:              now(),
		RestartCount:           0,
		StopSignal:             false,
		PIDFile:                pidFile,
		LogFile:                logFile,
		ErrorFile:              errFile,
		AutoRestart:            config.AutoRestart,
		MaxRestarts:            10, // Default
		HealthCheckURL:         healthCheckURL,
		SkipInitialHealthCheck: config.SkipInitialHealthCheck,
		Port:                   config.Port,
		RuntimeType:            config.RuntimeType,
		SIFPath:                config.SIFPath,
		cmd:                    cmd,
	}

	return process, nil
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

// buildNativeSingularityCommand builds a command for native Singularity execution on Linux
func buildNativeSingularityCommand(config SpawnConfig, logger *zerolog.Logger) *exec.Cmd {
	args := []string{"exec"}

	// Add environment variables as --env flags (for inside container)
	for key, value := range config.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	// Add bind mount for formation directory as /formation
	args = append(args, "--bind", fmt.Sprintf("%s:/formation", config.WorkDir))

	// Add bind mount for /tmp (allows formations to write temporary files)
	args = append(args, "--bind", "/tmp")

	// Add SIF file path
	args = append(args, config.SIFPath)

	// Add the command to run inside container
	args = append(args, config.Command)

	// Add command arguments
	args = append(args, config.Args...)

	logger.Debug().
		Str("id", config.ID).
		Str("platform", "linux").
		Str("sif_path", config.SIFPath).
		Strs("singularity_args", args).
		Msg("Spawning native Singularity process")

	return exec.Command("singularity", args...)
}

// buildDockerSingularityCommand builds a command for Docker-wrapped Singularity on macOS/Windows
// Uses runtime-runner (Docker image with Singularity) to execute the SIF file
func buildDockerSingularityCommand(config SpawnConfig, logger *zerolog.Logger) *exec.Cmd {
	// runtime-runner is a Docker image that contains Singularity
	// It allows running SIF files on platforms where Singularity isn't available natively
	runtimeRunnerImage := "ghcr.io/muxi-ai/runtime-runner:latest"

	// Container name for easy cleanup
	containerName := fmt.Sprintf("muxi-%s", config.ID)

	args := []string{
		"run",
		"--rm",                  // Remove container after exit
		"--name", containerName, // Named container for cleanup
		"--privileged", // Required for Singularity user namespaces

		// Mount SIF file from host into Docker container
		"-v", fmt.Sprintf("%s:/sif/runtime.sif:ro", config.SIFPath),

		// Mount formation directory from host into Docker container
		// Note: Not read-only because runtime may need to create .key file for secrets
		"-v", fmt.Sprintf("%s:/formation", config.WorkDir),

		// Mount /etc/localtime for Singularity (it expects this to exist)
		"-v", "/etc/localtime:/etc/localtime:ro",

		// Expose formation port
		"-p", fmt.Sprintf("%d:%d", config.Port, config.Port),
	}

	// Pass environment variables to Docker container
	// These will be inherited by Singularity inside the container
	for key, value := range config.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add the runtime-runner image
	args = append(args, runtimeRunnerImage)

	// runtime-runner's ENTRYPOINT is "singularity", so we append the exec command
	args = append(args, "exec")

	// Bind /formation inside the SIF (Singularity needs explicit bind)
	// Not read-only because runtime may need to write .key file for secrets
	args = append(args, "--bind", "/formation")

	// Bind /tmp for temporary files
	args = append(args, "--bind", "/tmp")

	// The SIF file path (as mounted in Docker at /sif/runtime.sif)
	args = append(args, "/sif/runtime.sif")

	// Command to run inside the SIF
	args = append(args, "python", "-m", "muxi.runtime.utils.run_formation")
	args = append(args, "/formation")
	args = append(args, "--port", strconv.Itoa(config.Port))
	args = append(args, "--host", "0.0.0.0")

	logger.Debug().
		Str("id", config.ID).
		Str("platform", runtime.GOOS).
		Str("sif_path", config.SIFPath).
		Str("runtime_runner", runtimeRunnerImage).
		Str("container_name", containerName).
		Strs("docker_args", args).
		Msg("Spawning formation via runtime-runner (Docker + Singularity + SIF)")

	return exec.Command("docker", args...)
}

// CleanupDockerContainer stops and removes any existing Docker container for a formation.
// This handles orphaned containers from crashes, restarts, or server restarts.
// It first tries to remove by container name (muxi-{id}), then as fallback kills anything on the port.
// Returns true if a container was found and cleaned up.
func CleanupDockerContainer(formationID string, port int, logger *zerolog.Logger) bool {
	containerName := fmt.Sprintf("muxi-%s", formationID)
	cleaned := false

	// Check if container exists (running or stopped) using docker ps -a
	checkCmd := exec.Command("docker", "ps", "-aq", "--filter", fmt.Sprintf("name=^%s$", containerName))
	if out, err := checkCmd.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		containerID := strings.TrimSpace(string(out))
		logger.Info().
			Str("container", containerName).
			Str("container_id", containerID).
			Msg("Found existing container, removing...")

		// Use rm -f which stops (SIGKILL) and removes in one operation
		// This is faster and more reliable than stop + rm
		rmCmd := exec.Command("docker", "rm", "-f", containerName)
		if rmOut, rmErr := rmCmd.CombinedOutput(); rmErr != nil {
			logger.Warn().
				Err(rmErr).
				Str("container", containerName).
				Str("output", strings.TrimSpace(string(rmOut))).
				Msg("Failed to remove container, will retry")

			// Retry with container ID instead of name
			if retryOut, retryErr := exec.Command("docker", "rm", "-f", containerID).CombinedOutput(); retryErr != nil {
				logger.Error().
					Err(retryErr).
					Str("container_id", containerID).
					Str("output", strings.TrimSpace(string(retryOut))).
					Msg("Failed to remove container by ID")
			} else {
				logger.Info().Str("container_id", containerID).Msg("Container removed by ID")
				cleaned = true
			}
		} else {
			logger.Info().Str("container", containerName).Msg("Container removed")
			cleaned = true
		}

		// Give Docker a moment to fully release the name
		time.Sleep(500 * time.Millisecond)
	}

	// Fallback: kill any container using this port (handles edge cases like renamed containers)
	portFilter := fmt.Sprintf("publish=%d", port)
	out, err := exec.Command("docker", "ps", "-q", "--filter", portFilter).Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		containerIDs := strings.TrimSpace(string(out))
		logger.Warn().
			Str("containers", containerIDs).
			Int("port", port).
			Msg("Found orphaned containers on port, removing")

		for _, id := range strings.Split(containerIDs, "\n") {
			id = strings.TrimSpace(id)
			if id != "" {
				if rmOut, rmErr := exec.Command("docker", "rm", "-f", id).CombinedOutput(); rmErr != nil {
					logger.Error().
						Err(rmErr).
						Str("container_id", id).
						Str("output", strings.TrimSpace(string(rmOut))).
						Msg("Failed to remove orphaned container")
				} else {
					logger.Info().Str("container_id", id).Msg("Orphaned container removed")
					cleaned = true
				}
			}
		}

		// Give Docker a moment to fully release the port
		time.Sleep(500 * time.Millisecond)
	}

	return cleaned
}
