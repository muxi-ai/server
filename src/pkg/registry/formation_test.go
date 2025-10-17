package registry

import (
	"testing"
	"time"

	"github.com/muxi-ai/server/pkg/process"
)

func TestFormation_FromProcess(t *testing.T) {
	proc := &process.Process{
		ID:           "test-formation",
		Name:         "Test Formation",
		PID:          12345,
		Status:       process.StatusRunning,
		StartedAt:    time.Now(),
		RestartCount: 3,
	}

	formation := &Formation{
		ID:   "test-formation",
		Name: "Test Formation",
		Port: 8080,
	}

	formation.UpdateFromProcess(proc)

	if formation.ProcessID != 12345 {
		t.Errorf("ProcessID = %d, want 12345", formation.ProcessID)
	}

	if formation.Status != "running" {
		t.Errorf("Status = %q, want %q", formation.Status, "running")
	}

	if formation.RestartCount != 3 {
		t.Errorf("RestartCount = %d, want 3", formation.RestartCount)
	}

	if formation.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
}

func TestFormation_ToProcessInfo(t *testing.T) {
	formation := &Formation{
		ID:           "test-formation",
		Name:         "Test Formation",
		Port:         8080,
		ProcessID:    12345,
		Status:       "running",
		StartedAt:    time.Now().Add(-5 * time.Minute),
		RestartCount: 2,
		Healthy:      true,
	}

	info := formation.ToProcessInfo()

	if info.ID != "test-formation" {
		t.Errorf("ID = %q, want %q", info.ID, "test-formation")
	}

	if info.Name != "Test Formation" {
		t.Errorf("Name = %q, want %q", info.Name, "Test Formation")
	}

	if info.Port != 8080 {
		t.Errorf("Port = %d, want 8080", info.Port)
	}

	if info.PID != 12345 {
		t.Errorf("PID = %d, want 12345", info.PID)
	}

	if info.Status != "running" {
		t.Errorf("Status = %q, want %q", info.Status, "running")
	}

	if info.RestartCount != 2 {
		t.Errorf("RestartCount = %d, want 2", info.RestartCount)
	}

	if info.Uptime == "" {
		t.Error("Uptime should not be empty")
	}

	if info.Uptime == "0s" {
		t.Error("Uptime should not be 0s for a process running for 5 minutes")
	}
}

func TestFormation_UpdateFromProcess(t *testing.T) {
	proc := &process.Process{
		ID:           "update-test",
		Name:         "Updated Name",
		PID:          54321,
		Status:       process.StatusRunning,
		StartedAt:    time.Now(),
		RestartCount: 5,
	}

	formation := &Formation{
		ID:   "update-test",
		Name: "Original Name",
		Port: 8080,
	}

	formation.UpdateFromProcess(proc)

	if formation.ProcessID != 54321 {
		t.Errorf("ProcessID = %d, want 54321", formation.ProcessID)
	}

	if formation.RestartCount != 5 {
		t.Errorf("RestartCount = %d, want 5", formation.RestartCount)
	}
}

func TestFormation_StatusMapping(t *testing.T) {
	tests := []struct {
		processStatus process.ProcessStatus
		wantStatus    string
	}{
		{process.StatusRunning, "running"},
		{process.StatusStarting, "starting"},
		{process.StatusStopped, "stopped"},
		{process.StatusCrashed, "crashed"},
		{process.StatusStopping, "stopping"},
		{process.StatusRestarting, "restarting"},
	}

	for _, tt := range tests {
		t.Run(string(tt.processStatus), func(t *testing.T) {
			proc := &process.Process{
				ID:     "test",
				Status: tt.processStatus,
			}

			formation := &Formation{ID: "test"}
			formation.UpdateFromProcess(proc)

			if formation.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q for process status %q",
					formation.Status, tt.wantStatus, tt.processStatus)
			}
		})
	}
}

func TestFormation_ToProcessInfo_StoppedFormation(t *testing.T) {
	formation := &Formation{
		ID:        "stopped-formation",
		Name:      "Stopped Formation",
		Port:      8080,
		Status:    "stopped",
		ProcessID: 0,
	}

	info := formation.ToProcessInfo()

	if info.Status != "stopped" {
		t.Errorf("Status = %q, want %q", info.Status, "stopped")
	}

	if info.PID != 0 {
		t.Errorf("PID = %d, want 0", info.PID)
	}

	if info.Uptime != "0s" {
		t.Errorf("Uptime = %q, want %q for stopped formation", info.Uptime, "0s")
	}
}

func TestFormation_ToProcessInfo_UptimeCalculation(t *testing.T) {
	tests := []struct {
		name      string
		startedAt time.Time
		status    string
		wantZero  bool
	}{
		{
			name:      "running for 1 hour",
			startedAt: time.Now().Add(-1 * time.Hour),
			status:    "running",
			wantZero:  false,
		},
		{
			name:      "stopped with zero time",
			startedAt: time.Time{},
			status:    "stopped",
			wantZero:  true,
		},
		{
			name:      "running but zero start time",
			startedAt: time.Time{},
			status:    "running",
			wantZero:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formation := &Formation{
				ID:        "test",
				Status:    tt.status,
				StartedAt: tt.startedAt,
			}

			info := formation.ToProcessInfo()

			if tt.wantZero {
				if info.Uptime != "0s" {
					t.Errorf("Uptime = %q, want %q", info.Uptime, "0s")
				}
			} else {
				if info.Uptime == "0s" {
					t.Error("Uptime should not be 0s for running formation with start time")
				}
			}
		})
	}
}

func TestFormation_Fields(t *testing.T) {
	formation := &Formation{
		ID:              "full-test",
		Name:            "Full Test",
		Port:            8080,
		Status:          "running",
		ProcessID:       12345,
		Command:         "python",
		Args:            []string{"app.py"},
		DeployedAt:      time.Now(),
		StartedAt:       time.Now(),
		Healthy:         true,
		LastHealthCheck: time.Now(),
		RestartCount:    3,
	}

	if formation.ID != "full-test" {
		t.Errorf("ID = %q, want %q", formation.ID, "full-test")
	}

	if formation.Port != 8080 {
		t.Errorf("Port = %d, want 8080", formation.Port)
	}

	if !formation.Healthy {
		t.Error("Healthy should be true")
	}

	if len(formation.Args) != 1 {
		t.Errorf("Args length = %d, want 1", len(formation.Args))
	}
}
