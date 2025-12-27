package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/muxi-ai/server/pkg/registry"
)

func TestRestoreFormations_NoFormations(t *testing.T) {
	server := createTestServer(t)

	// No formations registered - should not panic
	server.RestoreFormations()
}

func TestRestoreFormations_SkipsStoppedFormations(t *testing.T) {
	server := createTestServer(t)

	// Register a stopped formation
	server.registry.Register(&registry.Formation{
		ID:     "stopped-formation",
		Port:   8080,
		Status: "stopped",
	})

	// Should skip the stopped formation without error
	server.RestoreFormations()

	// Formation should still be stopped
	f, _ := server.registry.Get("stopped-formation")
	if f.Status != "stopped" {
		t.Errorf("Status = %q, want stopped", f.Status)
	}
}

func TestRestoreFormations_MissingDirectory(t *testing.T) {
	server := createTestServer(t)

	// Register a formation that doesn't exist on disk
	server.registry.Register(&registry.Formation{
		ID:     "missing-formation",
		Port:   8080,
		Status: "running",
	})

	// Should handle missing directory gracefully (logs error but doesn't panic)
	server.RestoreFormations()
}

func TestRestoreFormation_WithValidFormationFile(t *testing.T) {
	server := createTestServer(t)

	// Get muxi dir and create formation structure
	muxiDir, err := getMuxiDir()
	if err != nil {
		t.Fatalf("getMuxiDir() error: %v", err)
	}

	formationID := "test-restore-valid"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")

	// Create formation directory
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create formation dir: %v", err)
	}
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))

	// Create a valid formation.yaml
	formationYAML := `name: test-formation
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(formationYAML), 0644); err != nil {
		t.Fatalf("Failed to write formation.yaml: %v", err)
	}

	// Register formation
	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "running",
	})

	// RestoreFormations will try to spawn the process (which will fail)
	// but we get coverage of the file parsing code
	server.RestoreFormations()
}

func TestRestoreFormation_MissingFormationYAML(t *testing.T) {
	server := createTestServer(t)

	muxiDir, _ := getMuxiDir()
	formationID := "test-restore-no-yaml"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")

	// Create directory but NO formation.yaml
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create formation dir: %v", err)
	}
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))

	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "running",
	})

	// Should handle missing formation.yaml gracefully
	server.RestoreFormations()
}

func TestRestoreFormation_InvalidFormationYAML(t *testing.T) {
	server := createTestServer(t)

	muxiDir, _ := getMuxiDir()
	formationID := "test-restore-invalid-yaml"
	currentDir := filepath.Join(muxiDir, "formations", formationID, "current")

	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create formation dir: %v", err)
	}
	defer os.RemoveAll(filepath.Join(muxiDir, "formations", formationID))

	// Create invalid YAML
	invalidYAML := `name: [invalid yaml
  this is not valid`
	os.WriteFile(filepath.Join(currentDir, "formation.yaml"), []byte(invalidYAML), 0644)

	server.registry.Register(&registry.Formation{
		ID:     formationID,
		Port:   8080,
		Status: "running",
	})

	// Should handle invalid YAML gracefully
	server.RestoreFormations()
}
