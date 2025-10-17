package process

import (
	"os/exec"
	"time"
)

// Process represents a managed process (formation runtime)
type Process struct {
	// Identification
	ID   string // Formation ID (e.g., "my-api")
	Name string // Display name
	PID  int    // Process ID

	// Command details
	Command string   // Executable path (e.g., "python")
	Args    []string // Arguments (e.g., ["test/dummy_app.py", "--port", "8001"])
	WorkDir string   // Working directory

	// State
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
	AutoRestart    bool // Auto-restart on crash
	MaxRestarts    int  // Max restart attempts (default: 10)
	RestartDelay   time.Duration
	HealthCheckURL string // URL for health checks (e.g., "http://localhost:8001/health")

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

// IsRunning returns true if the process is in a running state
func (p *Process) IsRunning() bool {
	return p.Status == StatusRunning || p.Status == StatusStarting
}

// IsStopped returns true if the process is stopped
func (p *Process) IsStopped() bool {
	return p.Status == StatusStopped || p.Status == StatusCrashed
}

// ShouldRestart returns true if the process should be auto-restarted
func (p *Process) ShouldRestart() bool {
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
	uptime := "0s"
	if !p.StartedAt.IsZero() && p.IsRunning() {
		uptime = p.Uptime().Round(time.Second).String()
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
