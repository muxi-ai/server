package api

import (
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
