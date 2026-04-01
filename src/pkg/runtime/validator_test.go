package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSingularityPathPrefersApptainer(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	apptainerPath := filepath.Join(tmpDir, "apptainer")
	singularityPath := filepath.Join(tmpDir, "singularity")

	for _, path := range []string{apptainerPath, singularityPath} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", path, err)
		}
	}

	got, err := getSingularityPath()
	if err != nil {
		t.Fatalf("getSingularityPath() error = %v", err)
	}
	if got != apptainerPath {
		t.Fatalf("getSingularityPath() = %q, want %q", got, apptainerPath)
	}
}

func TestGetSingularityPathFallsBackToSingularity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	singularityPath := filepath.Join(tmpDir, "singularity")
	if err := os.WriteFile(singularityPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create singularity: %v", err)
	}

	got, err := getSingularityPath()
	if err != nil {
		t.Fatalf("getSingularityPath() error = %v", err)
	}
	if got != singularityPath {
		t.Fatalf("getSingularityPath() = %q, want %q", got, singularityPath)
	}
}
