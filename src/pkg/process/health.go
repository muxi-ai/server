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
		Endpoint:   "/v1/health",
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
		// Also check logs for failure markers (for containerized formations where container stays alive)
		if pid > 0 {
			processDead := !IsProcessRunning(pid)
			var logContent string
			var hasFailureMarker bool

			// Check stdout for failure markers (works for both native and containerized)
			if logFile != "" {
				if content, err := extractErrorSection(logFile); err == nil && content != "" {
					logContent = content
					hasFailureMarker = true
				}
				// Also check stderr for errors (Docker errors go here)
				if logContent == "" {
					errFile := logFile[:len(logFile)-8] + "-err.log" // Replace -out.log with -err.log
					if content, err := readLastLines(errFile, 20); err == nil && content != "" {
						logContent = content
						// If process is dead and stderr has content, treat as failure
						if processDead {
							hasFailureMarker = true
						}
					}
				}
			}

			if processDead || hasFailureMarker {
				errMsg := "Formation process crashed during startup"
				if logContent != "" {
					cleanedLog := sanitizeLogOutput(logContent)
					if cleanedLog != "" {
						errMsg = fmt.Sprintf("Formation crashed during startup:\n%s", cleanedLog)
					}
				}
				log.Error().
					Str("formation_id", formationID).
					Int("pid", pid).
					Int("attempt", attempt).
					Bool("process_dead", processDead).
					Bool("failure_marker", hasFailureMarker).
					Msg("Formation process crashed")
				return fmt.Errorf("%s", errMsg)
			}
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

// extractErrorSection reads a log file and extracts error content from the LATEST run
// Looks for the last "====" separator (start of new run), then finds "[ FAIL ]" or "❌" after it
func extractErrorSection(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	content := string(data)

	// Find the LAST "====" separator (indicates start of latest run)
	separator := "===================================================================="
	lastSepIdx := -1
	for i := len(content) - len(separator); i >= 0; i-- {
		if i+len(separator) <= len(content) && content[i:i+len(separator)] == separator {
			lastSepIdx = i
			break
		}
	}

	// If found, only look at content after the last separator
	searchContent := content
	if lastSepIdx >= 0 {
		searchContent = content[lastSepIdx:]
	}

	// Look for "[ FAIL ]" marker in the latest run
	marker := "[ FAIL ]"
	idx := -1
	for i := len(searchContent) - len(marker); i >= 0; i-- {
		if searchContent[i:i+len(marker)] == marker {
			idx = i
			break
		}
	}

	if idx >= 0 {
		// Return everything AFTER the marker, trimmed
		result := searchContent[idx+len(marker):]
		return trimWhitespace(result), nil
	}

	// Also check for "❌" marker (Unicode)
	errorMarker := "❌"
	for i := len(searchContent) - len(errorMarker); i >= 0; i-- {
		if i+len(errorMarker) <= len(searchContent) && searchContent[i:i+len(errorMarker)] == errorMarker {
			idx = i
			break
		}
	}

	if idx >= 0 {
		result := searchContent[idx:]
		return trimWhitespace(result), nil
	}

	// No marker found
	return "", nil
}

func trimWhitespace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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

// sanitizeLogOutput removes ASCII art banners and cleans up log output for error messages
func sanitizeLogOutput(content string) string {
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

	// Filter out ASCII art lines (contain box-drawing chars or mostly special chars)
	// and consecutive empty lines
	result := make([]string, 0)
	lastWasEmpty := false
	for _, line := range lines {
		trimmed := trimWhitespace(line)

		// Skip ASCII art lines (MUXI banner)
		if isASCIIArtLine(trimmed) {
			continue
		}

		// Skip "Documentation:" and "Support:" lines from banner
		if len(trimmed) > 2 && trimmed[0] == '*' && trimmed[1] == ' ' {
			continue
		}

		// Collapse consecutive empty lines
		if trimmed == "" {
			if lastWasEmpty {
				continue
			}
			lastWasEmpty = true
		} else {
			lastWasEmpty = false
		}

		result = append(result, line)
	}

	// Join and trim
	output := ""
	for i, line := range result {
		if i > 0 {
			output += "\n"
		}
		output += line
	}
	return trimWhitespace(output)
}

// isASCIIArtLine checks if a line is part of ASCII art (MUXI banner)
func isASCIIArtLine(line string) bool {
	if len(line) == 0 {
		return false
	}
	// Check for box-drawing characters or lines that are mostly special chars
	specialCount := 0
	for _, c := range line {
		if c == '|' || c == '\\' || c == '/' || c == '_' || c == '=' || c == '[' || c == ']' {
			specialCount++
		}
	}
	// If more than 30% special chars, likely ASCII art
	return specialCount > len(line)/3

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
