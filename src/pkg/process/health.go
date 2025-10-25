package process

import (
	"context"
	"fmt"
	"net/http"
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

// WaitForHealthy polls the formation's health endpoint until healthy or timeout
// Returns nil if the formation becomes healthy, error otherwise
func (hc *HealthChecker) WaitForHealthy(port int, formationID string) error {
	deadline := time.Now().Add(hc.Timeout)
	attempt := 0

	log.Info().
		Str("formation_id", formationID).
		Int("port", port).
		Str("endpoint", hc.Endpoint).
		Dur("timeout", hc.Timeout).
		Msg("Starting health checks for staging formation")

	for time.Now().Before(deadline) && attempt < hc.MaxRetries {
		attempt++

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
