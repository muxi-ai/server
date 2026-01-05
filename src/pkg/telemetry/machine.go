package telemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetMachineID returns a deterministic machine ID.
// It's cached in ~/.muxi/config.yaml after first generation.
func GetMachineID() string {
	// Check cache first
	if id := getCachedMachineID(); id != "" {
		return id
	}

	// Generate from OS
	osID := getOSMachineID()
	if osID == "" {
		// Fallback: generate random UUID and cache it
		osID = generateRandomID()
	}

	machineID := hashMachineID(osID)
	cacheMachineID(machineID)
	return machineID
}

// getOSMachineID returns the platform-specific machine identifier
func getOSMachineID() string {
	switch runtime.GOOS {
	case "darwin":
		return getMacOSMachineID()
	case "linux":
		return getLinuxMachineID()
	case "windows":
		return getWindowsMachineID()
	default:
		return ""
	}
}

func getMacOSMachineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "IOPlatformUUID") {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				return parts[3]
			}
		}
	}
	return ""
}

func getLinuxMachineID() string {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func getWindowsMachineID() string {
	out, err := exec.Command("wmic", "csproduct", "get", "uuid").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1])
	}
	return ""
}

// hashMachineID creates a SHA256 hash with "muxi" salt, formatted as UUID
func hashMachineID(osID string) string {
	hash := sha256.Sum256([]byte(osID + "muxi"))
	hex := fmt.Sprintf("%x", hash)
	// Format as UUID: 8-4-4-4-12
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

func generateRandomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
