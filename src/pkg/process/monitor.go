package process

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Monitor watches a process and handles health checks and restarts
type Monitor struct {
	process   *Process
	logger    *zerolog.Logger
	onCrash   func(*Process) // Callback when process crashes
	onHealthy func(*Process) // Callback when health check passes
	stopChan  chan struct{}
}

// NewMonitor creates a new process monitor
func NewMonitor(proc *Process, logger *zerolog.Logger) *Monitor {
	if logger == nil {
		l := zerolog.Nop()
		logger = &l
	}

	return &Monitor{
		process:  proc,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// OnCrash sets the callback for when a process crashes
func (m *Monitor) OnCrash(fn func(*Process)) {
	m.onCrash = fn
}

// OnHealthy sets the callback for when health check passes
func (m *Monitor) OnHealthy(fn func(*Process)) {
	m.onHealthy = fn
}

// Start begins monitoring the process
// Runs in a goroutine and returns immediately
func (m *Monitor) Start() {
	go m.run()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	close(m.stopChan)
}

func (m *Monitor) run() {
	m.logger.Debug().
		Str("id", m.process.ID).
		Msg("Monitor started")

	// Initial health check (after startup delay)
	if m.process.HealthCheckURL != "" {
		time.Sleep(2 * time.Second)
		if err := m.healthCheck(); err != nil {
			m.logger.Warn().
				Err(err).
				Str("id", m.process.ID).
				Msg("Initial health check failed")
		} else {
			m.process.Status = StatusRunning
			m.logger.Info().
				Str("id", m.process.ID).
				Msg("✓ Health check passed")

			if m.onHealthy != nil {
				m.onHealthy(m.process)
			}
		}
	} else {
		// No health check, assume running
		m.process.Status = StatusRunning
	}

	// Monitor loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.check()
		case <-m.stopChan:
			m.logger.Debug().
				Str("id", m.process.ID).
				Msg("Monitor stopped")
			return
		}
	}
}

func (m *Monitor) check() {
	// Check if process is still running
	if !IsProcessRunning(m.process.PID) {
		m.logger.Warn().
			Str("id", m.process.ID).
			Int("pid", m.process.PID).
			Msg("Process not running")

		// Check if it was intentionally stopped
		if m.process.StopSignal {
			m.logger.Debug().
				Str("id", m.process.ID).
				Msg("Process was intentionally stopped")
			m.process.Status = StatusStopped
			return
		}

		// Process crashed
		m.process.Status = StatusCrashed
		m.logger.Error().
			Str("id", m.process.ID).
			Int("restart_count", m.process.RestartCount).
			Msg("Process crashed")

		if m.onCrash != nil {
			m.onCrash(m.process)
		}
		return
	}

	// Process is running, do periodic health check
	if m.process.HealthCheckURL != "" && m.process.Status == StatusRunning {
		if err := m.healthCheck(); err != nil {
			m.logger.Warn().
				Err(err).
				Str("id", m.process.ID).
				Msg("Health check failed (process may be unhealthy)")
			// Don't mark as crashed yet, just log warning
		}
	}
}

func (m *Monitor) healthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", m.process.HealthCheckURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// WaitForExit waits for the process to exit
// Returns error if process exits unexpectedly (crash)
func WaitForExit(proc *Process) error {
	if proc.cmd == nil {
		return fmt.Errorf("process not running")
	}

	return proc.cmd.Wait()
}
