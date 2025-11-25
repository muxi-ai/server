package runtime

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Resolver resolves runtime version constraints to exact versions
type Resolver struct {
	available   []string // List of available runtime versions
	runtimesDir string   // Directory where SIF files are stored
}

// NewResolver creates a new runtime version resolver
func NewResolver(available []string, runtimesDir string) *Resolver {
	return &Resolver{
		available:   available,
		runtimesDir: runtimesDir,
	}
}

// GetSIFPath returns the full path to the SIF file for a given version
// Format: ~/.muxi/server/runtimes/muxi-runtime-{version}-{platform}.sif
// Example: ~/.muxi/server/runtimes/muxi-runtime-0.2025.0-darwin-arm64.sif
func (r *Resolver) GetSIFPath(version string) string {
	platform := getPlatform()
	filename := fmt.Sprintf("muxi-runtime-%s-%s.sif", version, platform)
	return filepath.Join(r.runtimesDir, filename)
}

// Resolve resolves a version constraint to an exact version
// Examples:
//   - "1.2.3" → "1.2.3" (exact)
//   - "1.2" → latest "1.2.x" (e.g., "1.2.5")
//   - "1" → latest "1.x.x" (e.g., "1.9.3")
//   - "latest" or "" → absolute latest version
func (r *Resolver) Resolve(constraint string) (string, error) {
	if constraint == "" || constraint == "latest" {
		return r.latest(), nil
	}

	parts := strings.Split(constraint, ".")

	switch len(parts) {
	case 3:
		// Exact version: "1.2.3"
		if r.exists(constraint) {
			return constraint, nil
		}
		return "", fmt.Errorf("runtime version %s not available", constraint)

	case 2:
		// Minor constraint: "1.2" → latest "1.2.x"
		latest := r.latestMatching(constraint + ".")
		if latest == "" {
			return "", fmt.Errorf("no runtime versions found matching %s.x", constraint)
		}
		return latest, nil

	case 1:
		// Major constraint: "1" → latest "1.x.x"
		latest := r.latestMatching(constraint + ".")
		if latest == "" {
			return "", fmt.Errorf("no runtime versions found matching %s.x.x", constraint)
		}
		return latest, nil

	default:
		return "", fmt.Errorf("invalid runtime version format: %s", constraint)
	}
}

// exists checks if an exact version exists
func (r *Resolver) exists(version string) bool {
	for _, v := range r.available {
		if v == version {
			return true
		}
	}
	return false
}

// latest returns the absolute latest version
func (r *Resolver) latest() string {
	if len(r.available) == 0 {
		return ""
	}

	// Find the highest version
	var latest string
	var latestMajor, latestMinor, latestPatch int

	for _, v := range r.available {
		major, minor, patch, err := parseVersion(v)
		if err != nil {
			continue
		}

		if latest == "" {
			latest = v
			latestMajor, latestMinor, latestPatch = major, minor, patch
			continue
		}

		if isNewer(major, minor, patch, latestMajor, latestMinor, latestPatch) {
			latest = v
			latestMajor, latestMinor, latestPatch = major, minor, patch
		}
	}

	return latest
}

// latestMatching returns the latest version matching a prefix
func (r *Resolver) latestMatching(prefix string) string {
	var matches []string

	for _, v := range r.available {
		if strings.HasPrefix(v, prefix) {
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Find the highest version among matches
	latest := matches[0]
	latestMajor, latestMinor, latestPatch, _ := parseVersion(latest)

	for _, v := range matches[1:] {
		major, minor, patch, err := parseVersion(v)
		if err != nil {
			continue
		}

		if isNewer(major, minor, patch, latestMajor, latestMinor, latestPatch) {
			latest = v
			latestMajor, latestMinor, latestPatch = major, minor, patch
		}
	}

	return latest
}

// parseVersion parses a semantic version string into major, minor, patch
func parseVersion(version string) (major, minor, patch int, err error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return major, minor, patch, nil
}

// isNewer returns true if version a is newer than version b
func isNewer(aMajor, aMinor, aPatch, bMajor, bMinor, bPatch int) bool {
	if aMajor > bMajor {
		return true
	}
	if aMajor < bMajor {
		return false
	}

	if aMinor > bMinor {
		return true
	}
	if aMinor < bMinor {
		return false
	}

	return aPatch > bPatch
}
