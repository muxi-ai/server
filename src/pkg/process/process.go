package process

import (
	"os/exec"
	"sync"
	"time"
)

// Process represents a managed process (formation runtime)
type Process struct {
	mu sync.Mutex // Protects concurrent access to process state

	// Identification
	ID   string // Formation ID (e.g., "my-api")
	Name string // Display name
	PID  int    // Process ID

	// Command details
	Command string   // Executable path (e.g., "python")
	Args    []string // Arguments (e.g., ["test/dummy_app.py", "--port", "8001"])
	WorkDir string   // Working directory

	// State (protected by mu)
	Status       ProcessStatus // Current status
	StartedAt    time.Time     // When process started
	RestartCount int           // Number of restarts
	StopSignal   bool          // Flag indicating intentional stop

	// Files
	PIDFile    string // Path to PID file
	LogFile    string // Path to stdout log
	ErrorFile  string // Path to stderr log
	ConfigPath string // Path to formation YAML (future)

	// Configuration
	AutoRestart            bool // Auto-restart on crash
	MaxRestarts            int  // Max restart attempts (default: 10)
	RestartDelay           time.Duration
	HealthCheckURL         string // URL for health checks (e.g., "http://localhost:8001/health")
	SkipInitialHealthCheck bool   // Skip initial health check (deploy does its own)

	// Runtime configuration (for auto-restart)
	RuntimeType string // "native" or "singularity"
	SIFPath     string // Path to SIF file (if RuntimeType is "singularity")

	// Internal
	cmd *exec.Cmd // Running command (nil if stopped)
}

// ProcessStatus represents the current state of a process
type ProcessStatus string

const (
	StatusStopped    ProcessStatus = "stopped"    // Not running
	StatusStarting   ProcessStatus = "starting"   // Being started
	StatusRunning    ProcessStatus = "running"    // Running normally
	StatusStopping   ProcessStatus = "stopping"   // Being stopped
	StatusCrashed    ProcessStatus = "crashed"    // Exited unexpectedly
	StatusRestarting ProcessStatus = "restarting" // Being restarted after crash
)

// GetStatus returns the current status (thread-safe)
func (p *Process) GetStatus() ProcessStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status
}

// SetStatus sets the current status (thread-safe)
func (p *Process) SetStatus(status ProcessStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
}

// GetStopSignal returns the stop signal flag (thread-safe)
func (p *Process) GetStopSignal() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.StopSignal
}

// SetStopSignal sets the stop signal flag (thread-safe)
func (p *Process) SetStopSignal(signal bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.StopSignal = signal
}

// GetRestartCount returns the restart count (thread-safe)
func (p *Process) GetRestartCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.RestartCount
}

// IncrementRestartCount increments the restart count (thread-safe)
func (p *Process) IncrementRestartCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.RestartCount++
	return p.RestartCount
}

// IsRunning returns true if the process is in a running state
func (p *Process) IsRunning() bool {
	status := p.GetStatus()
	return status == StatusRunning || status == StatusStarting
}

// IsStopped returns true if the process is stopped
func (p *Process) IsStopped() bool {
	status := p.GetStatus()
	return status == StatusStopped || status == StatusCrashed
}

// ShouldRestart returns true if the process should be auto-restarted
func (p *Process) ShouldRestart() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.AutoRestart &&
		p.RestartCount < p.MaxRestarts &&
		!p.StopSignal &&
		p.Status == StatusCrashed
}

// Uptime returns how long the process has been running
func (p *Process) Uptime() time.Duration {
	if p.StartedAt.IsZero() {
		return 0
	}
	return time.Since(p.StartedAt)
}

// ProcessInfo contains summary information about a process
// Used for API responses
type ProcessInfo struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	PID          int           `json:"pid"`
	Status       ProcessStatus `json:"status"`
	Port         int           `json:"port,omitempty"` // Will be added by registry
	Uptime       string        `json:"uptime"`
	RestartCount int           `json:"restart_count"`
	StartedAt    time.Time     `json:"started_at"`
}

// ToInfo converts a Process to ProcessInfo for API responses
func (p *Process) ToInfo() ProcessInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	uptime := "0s"
	if !p.StartedAt.IsZero() && (p.Status == StatusRunning || p.Status == StatusStarting) {
		uptime = time.Since(p.StartedAt).Round(time.Second).String()
	}

	return ProcessInfo{
		ID:           p.ID,
		Name:         p.Name,
		PID:          p.PID,
		Status:       p.Status,
		Uptime:       uptime,
		RestartCount: p.RestartCount,
		StartedAt:    p.StartedAt,
	}
}
