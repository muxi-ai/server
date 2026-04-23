package api

import (
	"os"
	goruntime "runtime"
	"time"

	"github.com/muxi-ai/server/pkg/config"
)

// resolveHealthTimeout returns the configured total timeout for the
// staging-health-check loop used by every spawn-and-wait handler
// (deploy/update/restart/start/dev/rollback). Previously deploy and
// update honored Formations.Deployment.HealthCheck.Timeout while the
// other four handlers hardcoded 300s — meaning operators who shortened
// the default 30s timeout in config.yaml got their setting silently
// ignored on four of the six code paths, and tests that legitimately
// expected fast failure (crashed python stub, no app.py) hung for
// minutes in CI. One helper keeps all six in sync.
//
// Falls back to 300s only when the config field is zero — matches the
// "5 minutes" comment already present in deploy.go/update.go, preserving
// production behavior for operators who never touched the field.
func resolveHealthTimeout(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.Formations.Deployment.HealthCheck.Timeout > 0 {
		return time.Duration(cfg.Formations.Deployment.HealthCheck.Timeout) * time.Second
	}
	return 300 * time.Second
}

// isRunningInContainer checks if we're running inside a container
func isRunningInContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check for container environment variable (Podman, etc.)
	if os.Getenv("container") != "" {
		return true
	}
	return false
}

// getBindHost returns the appropriate bind host for formations.
// Uses 0.0.0.0 for macOS/Windows (Docker runtime) and containers (network namespace isolation).
// Uses configured bind host (127.0.0.1) for native Linux for security.
func getBindHost(configuredHost string) string {
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" || isRunningInContainer() {
		return "0.0.0.0"
	}
	return configuredHost
}
