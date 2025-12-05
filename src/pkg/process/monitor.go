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

	// Initial health check with retries (formation may take time to start)
	// Docker + Singularity + Python startup can take 90+ seconds
	if m.process.HealthCheckURL != "" {
		maxRetries := 120 // 120 retries * 2 seconds = 240 seconds max startup time
		for i := 0; i < maxRetries; i++ {
			time.Sleep(2 * time.Second)
			
			// Check if stopped while waiting
			select {
			case <-m.stopChan:
				return
			default:
			}
			
			if err := m.healthCheck(); err != nil {
				// Only log every 10 attempts to reduce noise
				if (i+1)%10 == 1 {
					m.logger.Info().
						Str("id", m.process.ID).
						Int("attempt", i+1).
						Int("max_attempts", maxRetries).
						Msg("Waiting for formation to start...")
				}
			} else {
				m.process.SetStatus(StatusRunning)
				m.logger.Info().
					Str("id", m.process.ID).
					Int("attempts", i+1).
					Msg("✓ Health check passed")

				if m.onHealthy != nil {
					m.onHealthy(m.process)
				}
				break
			}
			
			// If we've exhausted retries, log error but keep monitoring
			if i == maxRetries-1 {
				m.logger.Error().
					Str("id", m.process.ID).
					Msg("Initial health check failed after max retries, will keep monitoring")
			}
		}
	} else {
		// No health check, assume running
		m.process.SetStatus(StatusRunning)
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
		if m.process.GetStopSignal() {
			m.logger.Debug().
				Str("id", m.process.ID).
				Msg("Process was intentionally stopped")
			m.process.SetStatus(StatusStopped)
			return
		}

		// Process crashed
		m.process.SetStatus(StatusCrashed)
		m.logger.Error().
			Str("id", m.process.ID).
			Int("restart_count", m.process.GetRestartCount()).
			Msg("Process crashed")

		if m.onCrash != nil {
			m.onCrash(m.process)
		}
		return
	}

	// Process is running, do periodic health check
	if m.process.HealthCheckURL != "" && m.process.GetStatus() == StatusRunning {
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
