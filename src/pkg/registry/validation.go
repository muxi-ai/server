package registry

import (
	"fmt"
	"regexp"
)

var (
	// Reserved formation IDs that cannot be used (conflict with server routes)
	reservedIDs = map[string]bool{
		"health":  true,
		"ping":    true,
		"rpc":     true,
		"server":  true,
		"admin":   true,
		"metrics": true,
		"api":     true,
	}

	// Formation ID pattern: lowercase letters, numbers, hyphens, 3-50 chars
	// Must start and end with alphanumeric
	formationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)
)

// ValidateFormationID checks if a formation ID is valid
func ValidateFormationID(id string) error {
	if id == "" {
		return fmt.Errorf("formation ID cannot be empty")
	}

	if len(id) < 3 || len(id) > 50 {
		return fmt.Errorf("formation ID must be 3-50 characters")
	}

	if reservedIDs[id] {
		return fmt.Errorf("formation ID %q is reserved", id)
	}

	if !formationIDPattern.MatchString(id) {
		return fmt.Errorf("formation ID must contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}
