package process

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// HealthChecker polls a formation's health endpoint until it becomes healthy or times out
type HealthChecker struct {
	Endpoint   string        // Health endpoint path (e.g., "/health")
	Timeout    time.Duration // Total timeout for health checks
	Interval   time.Duration // Poll interval between checks
	MaxRetries int           // Maximum number of health check attempts
}

// NewHealthChecker creates a new health checker with default settings
func NewHealthChecker(timeout, interval time.Duration) *HealthChecker {
	maxRetries := int(timeout / interval)
	if maxRetries <= 0 {
		maxRetries = 30
	}

	return &HealthChecker{
		Endpoint:   "/health",
		Timeout:    timeout,
		Interval:   interval,
		MaxRetries: maxRetries,
	}
}

// HealthCheckProgress is called during health check attempts
type HealthCheckProgress func(attempt, maxAttempts int)

// WaitForHealthy polls the formation's health endpoint until healthy or timeout
// Returns nil if the formation becomes healthy, error otherwise
func (hc *HealthChecker) WaitForHealthy(port int, formationID string) error {
	return hc.WaitForHealthyWithProgress(port, formationID, nil)
}

// WaitForHealthyWithProgress polls with a progress callback
func (hc *HealthChecker) WaitForHealthyWithProgress(port int, formationID string, onProgress HealthCheckProgress) error {
	return hc.WaitForHealthyWithPID(port, formationID, 0, "", onProgress)
}

// WaitForHealthyWithPID polls with process crash detection
// If pid > 0, checks if process is still running and fails fast if it crashed
// If logFile is provided, reads it on crash for error details (checks both stdout and stderr)
func (hc *HealthChecker) WaitForHealthyWithPID(port int, formationID string, pid int, logFile string, onProgress HealthCheckProgress) error {
	deadline := time.Now().Add(hc.Timeout)
	attempt := 0

	log.Info().
		Str("formation_id", formationID).
		Int("port", port).
		Int("pid", pid).
		Str("endpoint", hc.Endpoint).
		Dur("timeout", hc.Timeout).
		Dur("interval", hc.Interval).
		Int("max_retries", hc.MaxRetries).
		Msg("Starting health checks for staging formation")

	for time.Now().Before(deadline) && attempt < hc.MaxRetries {
		attempt++

		// Check if process crashed (if PID provided)
		if pid > 0 && !IsProcessRunning(pid) {
			errMsg := "Formation process crashed during startup"
			// Try to read log files for details (check both stdout and stderr)
			if logFile != "" {
				var logContent string
				// Try stdout log first (most errors go here) - just last 15 lines
				if content, err := readLastLines(logFile, 15); err == nil && content != "" {
					logContent = content
				}
				// Also check stderr log if stdout was empty
				if logContent == "" {
					errFile := logFile[:len(logFile)-8] + "-err.log" // Replace -out.log with -err.log
					if content, err := readLastLines(errFile, 15); err == nil && content != "" {
						logContent = content
					}
				}
				if logContent != "" {
					errMsg = fmt.Sprintf("Formation crashed during startup:\n%s", logContent)
				}
			}
			log.Error().
				Str("formation_id", formationID).
				Int("pid", pid).
				Int("attempt", attempt).
				Msg("Formation process crashed")
			return fmt.Errorf(errMsg)
		}

		// Notify progress callback
		if onProgress != nil {
			onProgress(attempt, hc.MaxRetries)
		}

		// Try health check
		if err := hc.checkHealth(port); err == nil {
			log.Info().
				Str("formation_id", formationID).
				Int("port", port).
				Int("attempt", attempt).
				Msg("Formation is healthy")
			return nil // Success!
		}

		// Log progress every 5 attempts
		if attempt%5 == 0 {
			log.Debug().
				Str("formation_id", formationID).
				Int("port", port).
				Int("attempt", attempt).
				Int("max_retries", hc.MaxRetries).
				Msg("Health check still pending")
		}

		// Wait before next attempt
		time.Sleep(hc.Interval)
	}

	// Timeout or max retries reached
	err := fmt.Errorf("health check failed after %d attempts (timeout: %v)", attempt, hc.Timeout)
	log.Error().
		Err(err).
		Str("formation_id", formationID).
		Int("port", port).
		Msg("Formation failed to become healthy")

	return err
}

// readLastLines reads the last n lines from a file
func readLastLines(filePath string, n int) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	content := string(data)
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result, nil
}

// checkHealth performs a single health check against the formation
func (hc *HealthChecker) checkHealth(port int) error {
	// Use a short timeout for individual requests (5 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, hc.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept any 2xx status code as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
}
