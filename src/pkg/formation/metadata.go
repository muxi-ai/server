package formation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// InjectMetadata adds server metadata to formation config file
// Adds:
//
//	_server_id: "server-abc-123"
//	_deployment_mode: "server"
func InjectMetadata(formationDir string, serverID string) error {
	formationPath, err := FindFormationFile(formationDir)
	if err != nil {
		return fmt.Errorf("failed to find formation config: %w", err)
	}

	// Read existing file
	data, err := os.ReadFile(formationPath)
	if err != nil {
		return fmt.Errorf("failed to read formation config: %w", err)
	}

	// Parse as generic map to preserve structure
	var formationMap map[string]interface{}
	if err := yaml.Unmarshal(data, &formationMap); err != nil {
		return fmt.Errorf("failed to parse formation config: %w", err)
	}

	// Inject metadata
	formationMap["_server_id"] = serverID
	formationMap["_deployment_mode"] = "server"

	// Write back
	newData, err := yaml.Marshal(formationMap)
	if err != nil {
		return fmt.Errorf("failed to marshal formation config: %w", err)
	}

	if err := os.WriteFile(formationPath, newData, 0644); err != nil {
		return fmt.Errorf("failed to write formation config: %w", err)
	}

	return nil
}

// GenerateServerID generates a unique server ID based on hostname + timestamp + hash
// Format: server-{hostname-prefix}-{8-hex-chars}
// Example: server-macbook-a3b4c5d6
func GenerateServerID() (string, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create unique string: hostname + timestamp
	uniqueStr := fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())

	// Hash it
	hash := sha256.Sum256([]byte(uniqueStr))
	hashStr := hex.EncodeToString(hash[:])

	// Take first 8 chars of hash
	shortHash := hashStr[:8]

	// Sanitize hostname (keep only alphanumeric and dash, max 16 chars)
	sanitized := ""
	for _, c := range hostname {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			sanitized += string(c)
		}
		if len(sanitized) >= 16 {
			break
		}
	}
	if sanitized == "" {
		sanitized = "unknown"
	}

	return fmt.Sprintf("server-%s-%s", sanitized, shortHash), nil
}
