package process

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Manager orchestrates process lifecycle management
// Handles spawning, monitoring, auto-restart, and stopping processes
type Manager struct {
	processes map[string]*managedProcess
	mu        sync.RWMutex
	logger    *zerolog.Logger
	baseDir   string // Base directory for logs, PIDs, etc.
}

// managedProcess wraps a Process with its monitor
type managedProcess struct {
	process *Process
	monitor *Monitor
}

// NewManager creates a new process manager
func NewManager(baseDir string, logger *zerolog.Logger) (*Manager, error) {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	// Create base directory structure
	dirs := []string{
		filepath.Join(baseDir, "logs"),
		filepath.Join(baseDir, "pids"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Process manager initialized silently

	return &Manager{
		processes: make(map[string]*managedProcess),
		logger:    logger,
		baseDir:   baseDir,
	}, nil
}

// Start spawns and monitors a new process
func (m *Manager) Start(config SpawnConfig) (*Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if process already exists
	if _, exists := m.processes[config.ID]; exists {
		return nil, fmt.Errorf("process with ID %s already exists", config.ID)
	}

	// Set up directories
	config.LogsDir = filepath.Join(m.baseDir, "logs")
	config.PIDsDir = filepath.Join(m.baseDir, "pids")
	config.Logger = m.logger

	// Spawn the process
	proc, err := Spawn(config)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn process: %w", err)
	}

	// Create monitor
	monitor := NewMonitor(proc, m.logger)

	// Set up crash handler
	monitor.OnCrash(func(p *Process) {
		m.handleCrash(p)
	})

	// Set up healthy callback
	monitor.OnHealthy(func(p *Process) {
		m.logger.Info().
			Str("id", p.ID).
			Msg("Process is healthy and ready")
	})

	// Start monitoring
	monitor.Start()

	// Store managed process
	m.processes[config.ID] = &managedProcess{
		process: proc,
		monitor: monitor,
	}

	m.logger.Info().
		Str("id", proc.ID).
		Int("pid", proc.PID).
		Msg("Process started and monitoring")

	return proc, nil
}

// Stop stops a running process
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	managed, exists := m.processes[id]
	if !exists {
		return fmt.Errorf("process %s not found", id)
	}

	// Stop monitor first
	managed.monitor.Stop()

	// Stop the process
	if err := Stop(managed.process, m.logger); err != nil {
		return fmt.Errorf("failed to stop process: %w", err)
	}

	// Remove from managed processes
	delete(m.processes, id)

	return nil
}

// ForceKill forcefully terminates a process with SIGKILL (Unix) or TerminateProcess (Windows)
// Used when graceful shutdown fails or during zero-downtime deployments
// Always removes from managed processes even if kill fails (process may already be dead)
func (m *Manager) ForceKill(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	managed, exists := m.processes[id]
	if !exists {
		return fmt.Errorf("process %s not found", id)
	}

	// Stop monitor first
	managed.monitor.Stop()

	// Force kill the process (ignore errors - process may already be dead)
	var killErr error
	if err := ForceKill(managed.process, m.logger); err != nil {
		killErr = fmt.Errorf("failed to force kill process: %w", err)
		m.logger.Warn().Err(err).Str("id", id).Msg("Force kill failed (process may already be dead)")
	}

	// Always remove from managed processes
	delete(m.processes, id)

	return killErr
}

// Restart stops and restarts a process
func (m *Manager) Restart(id string) (*Process, error) {
	m.mu.Lock()

	managed, exists := m.processes[id]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("process %s not found", id)
	}

	// Get the original config
	oldProc := managed.process
	config := SpawnConfig{
		ID:          oldProc.ID,
		Name:        oldProc.Name,
		Command:     oldProc.Command,
		Args:        oldProc.Args,
		WorkDir:     oldProc.WorkDir,
		Port:        extractPortFromURL(oldProc.HealthCheckURL),
		AutoRestart: oldProc.AutoRestart,
		RuntimeType: oldProc.RuntimeType,
		SIFPath:     oldProc.SIFPath,
	}

	// Stop monitor and process
	managed.monitor.Stop()
	Stop(managed.process, m.logger)
	delete(m.processes, id)

	m.mu.Unlock()

	// Small delay before restart
	time.Sleep(1 * time.Second)

	// Start new process
	return m.Start(config)
}

// Get retrieves a process by ID
func (m *Manager) Get(id string) (*Process, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	managed, exists := m.processes[id]
	if !exists {
		return nil, fmt.Errorf("process %s not found", id)
	}

	return managed.process, nil
}

// List returns all managed processes
func (m *Manager) List() []*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	processes := make([]*Process, 0, len(m.processes))
	for _, managed := range m.processes {
		processes = append(processes, managed.process)
	}

	return processes
}

// StopAll stops all managed processes
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info().Msg("Stopping all processes")

	var lastErr error
	for id, managed := range m.processes {
		managed.monitor.Stop()

		if err := Stop(managed.process, m.logger); err != nil {
			m.logger.Error().
				Err(err).
				Str("id", id).
				Msg("Failed to stop process")
			lastErr = err
		}
	}

	// Clear all processes
	m.processes = make(map[string]*managedProcess)

	return lastErr
}

// handleCrash handles a crashed process (auto-restart logic)
func (m *Manager) handleCrash(proc *Process) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Warn().
		Str("id", proc.ID).
		Int("restart_count", proc.RestartCount).
		Int("max_restarts", proc.MaxRestarts).
		Bool("auto_restart", proc.AutoRestart).
		Msg("Handling crashed process")

	if !proc.ShouldRestart() {
		m.logger.Error().
			Str("id", proc.ID).
			Msg("Process will not be restarted (max restarts reached or auto-restart disabled)")
		return
	}

	// Increment restart count
	restartCount := proc.IncrementRestartCount()

	m.logger.Info().
		Str("id", proc.ID).
		Int("restart_count", restartCount).
		Msg("Auto-restarting process...")

	// Get original config (including runtime info for SIF-based formations)
	config := SpawnConfig{
		ID:          proc.ID,
		Name:        proc.Name,
		Command:     proc.Command,
		Args:        proc.Args,
		WorkDir:     proc.WorkDir,
		Port:        extractPortFromURL(proc.HealthCheckURL),
		LogsDir:     filepath.Join(m.baseDir, "logs"),
		PIDsDir:     filepath.Join(m.baseDir, "pids"),
		AutoRestart: proc.AutoRestart,
		RuntimeType: proc.RuntimeType,
		SIFPath:     proc.SIFPath,
		Logger:      m.logger,
	}

	// Small delay before restart
	if proc.RestartDelay > 0 {
		time.Sleep(proc.RestartDelay)
	} else {
		time.Sleep(1 * time.Second)
	}

	// Spawn new process
	newProc, err := Spawn(config)
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("id", proc.ID).
			Msg("Failed to restart process")
		return
	}

	// Preserve restart count
	newProc.RestartCount = proc.RestartCount

	// Update monitor
	managed := m.processes[proc.ID]
	managed.process = newProc

	// Create new monitor
	newMonitor := NewMonitor(newProc, m.logger)
	newMonitor.OnCrash(func(p *Process) {
		m.handleCrash(p)
	})
	newMonitor.OnHealthy(func(p *Process) {
		m.logger.Info().
			Str("id", p.ID).
			Msg("Restarted process is healthy")
	})
	newMonitor.Start()

	// Stop old monitor and replace
	managed.monitor.Stop()
	managed.monitor = newMonitor

	m.logger.Info().
		Str("id", newProc.ID).
		Int("pid", newProc.PID).
		Msg("✓ Process restarted successfully")
}

// Helper function to extract port from health check URL
func extractPortFromURL(_ string) int {
	// Simple extraction: http://localhost:8001/health -> 8001
	// For now, we'll return 0 if we can't parse it
	// TODO: Implement proper URL parsing
	return 0
}
