package telemetry

import (
	"testing"
)

func TestHashMachineID(t *testing.T) {
	id := hashMachineID("test-machine-id")
	if id == "" {
		t.Error("expected non-empty hashed ID")
	}
	// Should be UUID format: 8-4-4-4-12
	parts := 0
	for _, c := range id {
		if c == '-' {
			parts++
		}
	}
	if parts != 4 {
		t.Errorf("expected UUID format (4 dashes), got %d dashes: %s", parts, id)
	}

	// Deterministic
	id2 := hashMachineID("test-machine-id")
	if id != id2 {
		t.Error("hashMachineID should be deterministic")
	}

	// Different input -> different output
	id3 := hashMachineID("different-machine")
	if id == id3 {
		t.Error("different inputs should produce different hashes")
	}
}

func TestGenerateRandomID(t *testing.T) {
	id1 := generateRandomID()
	id2 := generateRandomID()

	if id1 == "" || id2 == "" {
		t.Error("expected non-empty random IDs")
	}
	if id1 == id2 {
		t.Error("random IDs should be unique")
	}
}

func TestGetOSMachineID(t *testing.T) {
	// Should not panic on any platform
	id := getOSMachineID()
	t.Logf("OS machine ID: %q (may be empty on some platforms)", id)
}

func TestGetMachineID(t *testing.T) {
	id := GetMachineID()
	if id == "" {
		t.Error("expected non-empty machine ID")
	}
	// UUID format
	if len(id) < 32 {
		t.Errorf("machine ID too short: %s", id)
	}
}
