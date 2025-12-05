package registry

import (
	"time"

	"github.com/muxi-ai/server/pkg/process"
)

// Formation represents a deployed formation with its metadata
type Formation struct {
	// Core identification
	ID      string `json:"id"`                // Formation ID (unique)
	Name    string `json:"name"`              // Display name
	Version string `json:"version,omitempty"` // Semantic version from formation.yaml

	// Process details
	ProcessID int    `json:"process_id"` // PID of running process
	Port      int    `json:"port"`       // Allocated port (active/serving traffic)
	Status    string `json:"status"`     // "running", "stopped", "crashed"

	// Zero-downtime deployment (blue-green)
	StagingPort int  `json:"staging_port,omitempty"` // Port for staging version (0 if none)
	Deploying   bool `json:"deploying,omitempty"`    // True during blue-green deployment

	// Configuration
	ConfigPath string   `json:"config_path"` // Path to formation YAML (future)
	Command    string   `json:"command"`     // Command used to start
	Args       []string `json:"args"`        // Command arguments

	// Timestamps
	StartedAt    time.Time `json:"started_at"`
	DeployedAt   time.Time `json:"deployed_at"`
	RestartCount int       `json:"restart_count"`

	// Health
	HealthURL       string    `json:"health_url,omitempty"`
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`
	Healthy         bool      `json:"healthy"`
}

// FromProcess creates a Formation from a process.Process
func FromProcess(proc *process.Process, port int) *Formation {
	status := string(proc.GetStatus())

	return &Formation{
		ID:           proc.ID,
		Name:         proc.Name,
		ProcessID:    proc.PID,
		Port:         port,
		Status:       status,
		Command:      proc.Command,
		Args:         proc.Args,
		StartedAt:    proc.StartedAt,
		DeployedAt:   time.Now(),
		RestartCount: proc.GetRestartCount(),
		HealthURL:    proc.HealthCheckURL,
		Healthy:      proc.GetStatus() == process.StatusRunning,
	}
}

// UpdateFromProcess updates formation fields from a process
func (f *Formation) UpdateFromProcess(proc *process.Process) {
	f.ProcessID = proc.PID
	f.Status = string(proc.GetStatus())
	f.StartedAt = proc.StartedAt
	f.RestartCount = proc.GetRestartCount()
	f.Healthy = proc.GetStatus() == process.StatusRunning
}

// ToProcessInfo converts Formation to process.ProcessInfo for API responses
func (f *Formation) ToProcessInfo() process.ProcessInfo {
	uptime := "0s"
	if !f.StartedAt.IsZero() && f.Status == "running" {
		uptime = time.Since(f.StartedAt).Round(time.Second).String()
	}

	return process.ProcessInfo{
		ID:           f.ID,
		Name:         f.Name,
		PID:          f.ProcessID,
		Status:       process.ProcessStatus(f.Status),
		Port:         f.Port,
		Uptime:       uptime,
		RestartCount: f.RestartCount,
		StartedAt:    f.StartedAt,
	}
}

// FormationList represents a list of formations for API responses
type FormationList struct {
	Formations []*Formation `json:"formations"`
	Count      int          `json:"count"`
}
