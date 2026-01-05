package telemetry

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetSystemInfo returns OS and hardware information
func GetSystemInfo() SystemInfo {
	return SystemInfo{
		OS:       runtime.GOOS,
		Arch:     normalizeArch(runtime.GOARCH),
		CPUCores: runtime.NumCPU(),
		RAMGB:    getRAMGB(),
	}
}

// normalizeArch converts Go arch names to common names
func normalizeArch(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "386":
		return "i386"
	default:
		return arch
	}
}

// getRAMGB returns total system RAM in GB
func getRAMGB() int {
	switch runtime.GOOS {
	case "linux":
		return getLinuxRAM()
	case "darwin":
		return getDarwinRAM()
	default:
		return 0
	}
}

func getLinuxRAM() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return 0
				}
				return int(kb / 1024 / 1024) // Convert KB to GB
			}
		}
	}
	return 0
}

func getDarwinRAM() int {
	// Use sysctl to get hw.memsize (returns bytes)
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}

	bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}

	return int(bytes / 1024 / 1024 / 1024) // Convert bytes to GB
}
