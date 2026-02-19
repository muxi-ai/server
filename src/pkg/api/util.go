package api

import (
	"os"
	goruntime "runtime"
)

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
