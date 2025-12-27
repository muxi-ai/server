package formation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadVersionHistory(t *testing.T) {
	t.Run("load nonexistent version history", func(t *testing.T) {
		formationDir := filepath.Join(t.TempDir(), "formation")
		os.MkdirAll(formationDir, 0755)

		history, err := LoadVersionHistory(formationDir)
		if err != nil {
			t.Fatalf("LoadVersionHistory() error = %v, want nil", err)
		}

		if history == nil {
			t.Fatal("LoadVersionHistory() returned nil")
		}

		// Should return initialized history with version 0
		if history.CurrentVersion != 0 {
			t.Errorf("CurrentVersion = %d, want 0 for new formation", history.CurrentVersion)
		}

		if history.Current != nil {
			t.Error("Current should be nil for new formation")
		}

		if history.Previous != nil {
			t.Error("Previous should be nil for new formation")
		}
	})

	t.Run("load existing version history", func(t *testing.T) {
		formationDir := filepath.Join(t.TempDir(), "formation")
		os.MkdirAll(formationDir, 0755)

		// Create a version history file
		testHistory := &VersionHistory{
			CurrentVersion: 2,
			Current: &Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}

		// Save it
		if err := testHistory.Save(formationDir); err != nil {
			t.Fatalf("Failed to save test history: %v", err)
		}

		// Load it back
		history, err := LoadVersionHistory(formationDir)
		if err != nil {
			t.Fatalf("LoadVersionHistory() error = %v", err)
		}

		if history.CurrentVersion != 2 {
			t.Errorf("CurrentVersion = %d, want 2", history.CurrentVersion)
		}

		if history.PreviousVersion != 1 {
			t.Errorf("PreviousVersion = %d, want 1", history.PreviousVersion)
		}

		if history.Current == nil {
			t.Fatal("Current version should not be nil")
		}

		if history.Current.Version != 2 {
			t.Errorf("Current.Version = %d, want 2", history.Current.Version)
		}

		if history.Current.BundleHash != "abc123" {
			t.Errorf("Current.BundleHash = %q, want %q", history.Current.BundleHash, "abc123")
		}

		if history.Previous == nil {
			t.Fatal("Previous version should not be nil")
		}

		if history.Previous.Version != 1 {
			t.Errorf("Previous.Version = %d, want 1", history.Previous.Version)
		}
	})

	t.Run("load invalid version history", func(t *testing.T) {
		formationDir := filepath.Join(t.TempDir(), "formation")
		os.MkdirAll(formationDir, 0755)

		// Write invalid JSON
		versionPath := filepath.Join(formationDir, "version.json")
		os.WriteFile(versionPath, []byte("invalid json"), 0644)

		_, err := LoadVersionHistory(formationDir)
		if err == nil {
			t.Error("LoadVersionHistory() should fail with invalid JSON")
		}
	})
}

func TestVersionHistorySave(t *testing.T) {
	t.Run("save version history", func(t *testing.T) {
		formationDir := filepath.Join(t.TempDir(), "formation")
		os.MkdirAll(formationDir, 0755)

		history := &VersionHistory{
			CurrentVersion: 1,
			Current: &Version{
				Version:    1,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
		}

		err := history.Save(formationDir)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Check file was created
		versionPath := filepath.Join(formationDir, "version.json")
		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			t.Error("version.json file was not created")
		}

		// Read back and verify
		data, err := os.ReadFile(versionPath)
		if err != nil {
			t.Fatalf("Failed to read version.json: %v", err)
		}

		var loaded VersionHistory
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("Failed to unmarshal version.json: %v", err)
		}

		if loaded.CurrentVersion != 1 {
			t.Errorf("Loaded CurrentVersion = %d, want 1", loaded.CurrentVersion)
		}

		if loaded.Current.BundleHash != "abc123" {
			t.Errorf("Loaded BundleHash = %q, want %q", loaded.Current.BundleHash, "abc123")
		}
	})

	t.Run("save to nonexistent directory", func(t *testing.T) {
		// Create a file that blocks directory creation
		tmpDir := t.TempDir()
		blocker := filepath.Join(tmpDir, "blocker")
		if err := os.WriteFile(blocker, []byte("block"), 0644); err != nil {
			t.Fatalf("Failed to create blocker file: %v", err)
		}
		// Use a path that requires blocker to be a directory
		formationDir := filepath.Join(blocker, "subdir")

		history := &VersionHistory{
			CurrentVersion: 1,
		}

		err := history.Save(formationDir)
		if err == nil {
			t.Error("Save() should fail for nonexistent directory")
		}
	})

	t.Run("save with both current and previous", func(t *testing.T) {
		formationDir := filepath.Join(t.TempDir(), "formation")
		os.MkdirAll(formationDir, 0755)

		now := time.Now()
		history := &VersionHistory{
			CurrentVersion: 2,
			Current: &Version{
				Version:    2,
				DeployedAt: now,
				BundleHash: "current-hash",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &Version{
				Version:    1,
				DeployedAt: now.Add(-1 * time.Hour),
				BundleHash: "previous-hash",
				BackupPath: "previous",
			},
		}

		err := history.Save(formationDir)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load and verify
		loaded, err := LoadVersionHistory(formationDir)
		if err != nil {
			t.Fatalf("Failed to load saved history: %v", err)
		}

		if loaded.CurrentVersion != 2 {
			t.Errorf("CurrentVersion = %d, want 2", loaded.CurrentVersion)
		}

		if loaded.PreviousVersion != 1 {
			t.Errorf("PreviousVersion = %d, want 1", loaded.PreviousVersion)
		}

		if loaded.Current.BundleHash != "current-hash" {
			t.Errorf("Current.BundleHash = %q, want %q", loaded.Current.BundleHash, "current-hash")
		}

		if loaded.Previous.BundleHash != "previous-hash" {
			t.Errorf("Previous.BundleHash = %q, want %q", loaded.Previous.BundleHash, "previous-hash")
		}
	})
}

func TestComputeBundleHash(t *testing.T) {
	t.Run("compute hash of data", func(t *testing.T) {
		data := []byte("test data")
		hash := ComputeBundleHash(data)

		if hash == "" {
			t.Error("ComputeBundleHash() returned empty string")
		}

		// SHA256 hash should be 64 characters (hex encoded)
		if len(hash) != 64 {
			t.Errorf("Hash length = %d, want 64", len(hash))
		}
	})

	t.Run("same data produces same hash", func(t *testing.T) {
		data := []byte("test data")
		hash1 := ComputeBundleHash(data)
		hash2 := ComputeBundleHash(data)

		if hash1 != hash2 {
			t.Errorf("Same data should produce same hash: %q != %q", hash1, hash2)
		}
	})

	t.Run("different data produces different hash", func(t *testing.T) {
		data1 := []byte("test data 1")
		data2 := []byte("test data 2")

		hash1 := ComputeBundleHash(data1)
		hash2 := ComputeBundleHash(data2)

		if hash1 == hash2 {
			t.Error("Different data should produce different hashes")
		}
	})

	t.Run("empty data produces hash", func(t *testing.T) {
		data := []byte("")
		hash := ComputeBundleHash(data)

		if hash == "" {
			t.Error("ComputeBundleHash() should work with empty data")
		}

		if len(hash) != 64 {
			t.Errorf("Hash length = %d, want 64", len(hash))
		}
	})

	t.Run("hash is lowercase hex", func(t *testing.T) {
		data := []byte("test data")
		hash := ComputeBundleHash(data)

		// Should be all lowercase hex characters
		for _, c := range hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Hash contains non-hex character: %c", c)
			}
		}
	})
}

func TestHasPreviousVersion(t *testing.T) {
	t.Run("no previous version", func(t *testing.T) {
		history := &VersionHistory{
			CurrentVersion: 1,
			Current: &Version{
				Version: 1,
			},
		}

		if history.HasPreviousVersion() {
			t.Error("HasPreviousVersion() = true, want false when no previous version")
		}
	})

	t.Run("with previous version", func(t *testing.T) {
		history := &VersionHistory{
			CurrentVersion: 2,
			Current: &Version{
				Version: 2,
			},
			PreviousVersion: 1,
			Previous: &Version{
				Version: 1,
			},
		}

		if !history.HasPreviousVersion() {
			t.Error("HasPreviousVersion() = false, want true when previous version exists")
		}
	})

	t.Run("previous version is nil", func(t *testing.T) {
		history := &VersionHistory{
			CurrentVersion:  2,
			PreviousVersion: 1,
			Current: &Version{
				Version: 2,
			},
			Previous: nil,
		}

		if history.HasPreviousVersion() {
			t.Error("HasPreviousVersion() = true, want false when Previous is nil")
		}
	})

	t.Run("previous version number is 0", func(t *testing.T) {
		history := &VersionHistory{
			CurrentVersion:  1,
			PreviousVersion: 0,
			Current: &Version{
				Version: 1,
			},
			Previous: &Version{
				Version: 0,
			},
		}

		if history.HasPreviousVersion() {
			t.Error("HasPreviousVersion() = true, want false when PreviousVersion is 0")
		}
	})
}

func TestVersionStructure(t *testing.T) {
	t.Run("version marshals to JSON", func(t *testing.T) {
		now := time.Now()
		version := &Version{
			Version:    1,
			DeployedAt: now,
			BundleHash: "abc123",
			BackupPath: "current",
		}

		data, err := json.Marshal(version)
		if err != nil {
			t.Fatalf("Failed to marshal Version: %v", err)
		}

		// Unmarshal back
		var loaded Version
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("Failed to unmarshal Version: %v", err)
		}

		if loaded.Version != 1 {
			t.Errorf("Version = %d, want 1", loaded.Version)
		}

		if loaded.BundleHash != "abc123" {
			t.Errorf("BundleHash = %q, want %q", loaded.BundleHash, "abc123")
		}

		if loaded.BackupPath != "current" {
			t.Errorf("BackupPath = %q, want %q", loaded.BackupPath, "current")
		}
	})

	t.Run("version history marshals to JSON", func(t *testing.T) {
		history := &VersionHistory{
			CurrentVersion: 2,
			Current: &Version{
				Version:    2,
				DeployedAt: time.Now(),
				BundleHash: "abc123",
				BackupPath: "current",
			},
			PreviousVersion: 1,
			Previous: &Version{
				Version:    1,
				DeployedAt: time.Now().Add(-1 * time.Hour),
				BundleHash: "def456",
				BackupPath: "previous",
			},
		}

		data, err := json.Marshal(history)
		if err != nil {
			t.Fatalf("Failed to marshal VersionHistory: %v", err)
		}

		// Unmarshal back
		var loaded VersionHistory
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Fatalf("Failed to unmarshal VersionHistory: %v", err)
		}

		if loaded.CurrentVersion != 2 {
			t.Errorf("CurrentVersion = %d, want 2", loaded.CurrentVersion)
		}

		if loaded.PreviousVersion != 1 {
			t.Errorf("PreviousVersion = %d, want 1", loaded.PreviousVersion)
		}
	})
}
