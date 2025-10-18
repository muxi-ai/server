package formation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Version represents a deployed formation version
type Version struct {
	Version    int       `json:"version"`
	DeployedAt time.Time `json:"deployed_at"`
	BundleHash string    `json:"bundle_hash"`
	BackupPath string    `json:"backup_path"` // "current" or "previous"
}

// VersionHistory tracks formation versions
type VersionHistory struct {
	CurrentVersion  int      `json:"current_version"`
	PreviousVersion int      `json:"previous_version,omitempty"`
	Current         *Version `json:"current"`
	Previous        *Version `json:"previous,omitempty"`
}

// LoadVersionHistory loads version history from formation directory
func LoadVersionHistory(formationDir string) (*VersionHistory, error) {
	path := filepath.Join(formationDir, "version.json")

	// If file doesn't exist, this is first deployment
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &VersionHistory{
			CurrentVersion: 0,
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read version.json: %w", err)
	}

	var history VersionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse version.json: %w", err)
	}

	return &history, nil
}

// Save saves version history to formation directory
func (vh *VersionHistory) Save(formationDir string) error {
	path := filepath.Join(formationDir, "version.json")

	data, err := json.MarshalIndent(vh, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version history: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write version.json: %w", err)
	}

	return nil
}

// ComputeBundleHash computes SHA256 hash of bundle data
func ComputeBundleHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// HasPreviousVersion returns true if there's a previous version to rollback to
func (vh *VersionHistory) HasPreviousVersion() bool {
	return vh.Previous != nil && vh.PreviousVersion > 0
}
