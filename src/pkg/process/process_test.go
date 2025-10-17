package process

import (
	"testing"
	"time"
)

func TestProcess_IsRunning(t *testing.T) {
	tests := []struct {
		name   string
		status ProcessStatus
		want   bool
	}{
		{"Running", StatusRunning, true},
		{"Starting", StatusStarting, true},
		{"Stopping", StatusStopping, false},
		{"Stopped", StatusStopped, false},
		{"Crashed", StatusCrashed, false},
		{"Restarting", StatusRestarting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Process{Status: tt.status}
			if got := p.IsRunning(); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v (status: %s)", got, tt.want, tt.status)
			}
		})
	}
}

func TestProcess_IsStopped(t *testing.T) {
	tests := []struct {
		name   string
		status ProcessStatus
		want   bool
	}{
		{"Stopped", StatusStopped, true},
		{"Crashed", StatusCrashed, true},
		{"Running", StatusRunning, false},
		{"Starting", StatusStarting, false},
		{"Stopping", StatusStopping, false},
		{"Restarting", StatusRestarting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Process{Status: tt.status}
			if got := p.IsStopped(); got != tt.want {
				t.Errorf("IsStopped() = %v, want %v (status: %s)", got, tt.want, tt.status)
			}
		})
	}
}

func TestProcess_ShouldRestart(t *testing.T) {
	tests := []struct {
		name         string
		autoRestart  bool
		restartCount int
		maxRestarts  int
		stopSignal   bool
		status       ProcessStatus
		want         bool
	}{
		{
			name:         "Should restart - all conditions met",
			autoRestart:  true,
			restartCount: 2,
			maxRestarts:  10,
			stopSignal:   false,
			status:       StatusCrashed,
			want:         true,
		},
		{
			name:         "Should not restart - auto restart disabled",
			autoRestart:  false,
			restartCount: 2,
			maxRestarts:  10,
			stopSignal:   false,
			status:       StatusCrashed,
			want:         false,
		},
		{
			name:         "Should not restart - max restarts reached",
			autoRestart:  true,
			restartCount: 10,
			maxRestarts:  10,
			stopSignal:   false,
			status:       StatusCrashed,
			want:         false,
		},
		{
			name:         "Should not restart - stop signal set",
			autoRestart:  true,
			restartCount: 2,
			maxRestarts:  10,
			stopSignal:   true,
			status:       StatusCrashed,
			want:         false,
		},
		{
			name:         "Should not restart - not crashed",
			autoRestart:  true,
			restartCount: 2,
			maxRestarts:  10,
			stopSignal:   false,
			status:       StatusStopped,
			want:         false,
		},
		{
			name:         "Should not restart - max restarts exceeded",
			autoRestart:  true,
			restartCount: 11,
			maxRestarts:  10,
			stopSignal:   false,
			status:       StatusCrashed,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Process{
				AutoRestart:  tt.autoRestart,
				RestartCount: tt.restartCount,
				MaxRestarts:  tt.maxRestarts,
				StopSignal:   tt.stopSignal,
				Status:       tt.status,
			}

			if got := p.ShouldRestart(); got != tt.want {
				t.Errorf("ShouldRestart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcess_Uptime(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		p := &Process{}
		if uptime := p.Uptime(); uptime != 0 {
			t.Errorf("Uptime() = %v, want 0", uptime)
		}
	})

	t.Run("with start time", func(t *testing.T) {
		startTime := time.Now().Add(-5 * time.Minute)
		p := &Process{
			StartedAt: startTime,
		}

		uptime := p.Uptime()
		if uptime < 4*time.Minute || uptime > 6*time.Minute {
			t.Errorf("Uptime() = %v, want approximately 5 minutes", uptime)
		}
	})

	t.Run("just started", func(t *testing.T) {
		p := &Process{
			StartedAt: time.Now(),
		}

		uptime := p.Uptime()
		if uptime > 1*time.Second {
			t.Errorf("Uptime() = %v, want less than 1 second", uptime)
		}
	})
}

func TestProcess_ToInfo(t *testing.T) {
	t.Run("running process", func(t *testing.T) {
		startTime := time.Now().Add(-10 * time.Minute)
		p := &Process{
			ID:           "test-proc",
			Name:         "Test Process",
			PID:          12345,
			Status:       StatusRunning,
			StartedAt:    startTime,
			RestartCount: 3,
		}

		info := p.ToInfo()

		if info.ID != "test-proc" {
			t.Errorf("ID = %q, want %q", info.ID, "test-proc")
		}

		if info.Name != "Test Process" {
			t.Errorf("Name = %q, want %q", info.Name, "Test Process")
		}

		if info.PID != 12345 {
			t.Errorf("PID = %d, want %d", info.PID, 12345)
		}

		if info.Status != StatusRunning {
			t.Errorf("Status = %s, want %s", info.Status, StatusRunning)
		}

		if info.RestartCount != 3 {
			t.Errorf("RestartCount = %d, want %d", info.RestartCount, 3)
		}

		if info.Uptime == "" {
			t.Error("Uptime should not be empty for running process")
		}

		if info.Uptime == "0s" {
			t.Error("Uptime should not be 0s for process running 10 minutes")
		}
	})

	t.Run("stopped process", func(t *testing.T) {
		p := &Process{
			ID:     "stopped-proc",
			Name:   "Stopped Process",
			Status: StatusStopped,
		}

		info := p.ToInfo()

		if info.Uptime != "0s" {
			t.Errorf("Uptime = %q, want %q for stopped process", info.Uptime, "0s")
		}
	})

	t.Run("crashed process", func(t *testing.T) {
		p := &Process{
			ID:           "crashed-proc",
			Name:         "Crashed Process",
			PID:          0,
			Status:       StatusCrashed,
			RestartCount: 5,
		}

		info := p.ToInfo()

		if info.Status != StatusCrashed {
			t.Errorf("Status = %s, want %s", info.Status, StatusCrashed)
		}

		if info.RestartCount != 5 {
			t.Errorf("RestartCount = %d, want %d", info.RestartCount, 5)
		}

		if info.Uptime != "0s" {
			t.Errorf("Uptime = %q, want %q for crashed process", info.Uptime, "0s")
		}
	})
}

func TestProcessStatus_String(t *testing.T) {
	tests := []struct {
		status ProcessStatus
		want   string
	}{
		{StatusStopped, "stopped"},
		{StatusStarting, "starting"},
		{StatusRunning, "running"},
		{StatusStopping, "stopping"},
		{StatusCrashed, "crashed"},
		{StatusRestarting, "restarting"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("Status = %q, want %q", string(tt.status), tt.want)
			}
		})
	}
}

func TestProcess_RestartLogic(t *testing.T) {
	t.Run("restart count increments", func(t *testing.T) {
		p := &Process{
			AutoRestart:  true,
			RestartCount: 0,
			MaxRestarts:  10,
			Status:       StatusCrashed,
		}

		// Should restart initially
		if !p.ShouldRestart() {
			t.Error("Should restart with 0 restart count")
		}

		// Simulate multiple restarts
		for i := 1; i <= 10; i++ {
			p.RestartCount = i
			shouldRestart := p.ShouldRestart()
			
			if i < 10 && !shouldRestart {
				t.Errorf("Should restart at count %d (max: 10)", i)
			}
			if i >= 10 && shouldRestart {
				t.Errorf("Should not restart at count %d (max: 10)", i)
			}
		}
	})

	t.Run("different max restart values", func(t *testing.T) {
		tests := []struct {
			maxRestarts  int
			restartCount int
			shouldAllow  bool
		}{
			{5, 0, true},
			{5, 4, true},
			{5, 5, false},
			{5, 6, false},
			{1, 0, true},
			{1, 1, false},
			{100, 99, true},
			{100, 100, false},
		}

		for _, tt := range tests {
			p := &Process{
				AutoRestart:  true,
				RestartCount: tt.restartCount,
				MaxRestarts:  tt.maxRestarts,
				Status:       StatusCrashed,
			}

			got := p.ShouldRestart()
			if got != tt.shouldAllow {
				t.Errorf("MaxRestarts=%d, RestartCount=%d: ShouldRestart()=%v, want %v",
					tt.maxRestarts, tt.restartCount, got, tt.shouldAllow)
			}
		}
	})
}

func TestProcess_UptimeFormatting(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"1 minute", 1 * time.Minute},
		{"5 minutes", 5 * time.Minute},
		{"1 hour", 1 * time.Hour},
		{"24 hours", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Process{
				Status:    StatusRunning,
				StartedAt: time.Now().Add(-tt.duration),
			}

			info := p.ToInfo()
			if info.Uptime == "" {
				t.Error("Uptime should not be empty")
			}
			if info.Uptime == "0s" {
				t.Errorf("Uptime should not be 0s for duration %v", tt.duration)
			}

			t.Logf("Duration: %v → Uptime: %s", tt.duration, info.Uptime)
		})
	}
}
