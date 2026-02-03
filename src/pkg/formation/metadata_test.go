package formation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectMetadata(t *testing.T) {
	t.Run("inject into valid formation", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a test formation.yaml
		formationPath := filepath.Join(tmpDir, "formation.yaml")
		content := `schema: muxi.org/formation/v1
id: test-formation
name: Test Formation
version: 1.0.0`

		if err := os.WriteFile(formationPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test formation.yaml: %v", err)
		}

		// Inject metadata
		serverID := "server-test-abc123"
		if err := InjectMetadata(tmpDir, serverID); err != nil {
			t.Fatalf("InjectMetadata() error = %v, want nil", err)
		}

		// Read back and verify
		data, err := os.ReadFile(formationPath)
		if err != nil {
			t.Fatalf("Failed to read formation.yaml: %v", err)
		}

		var result map[string]interface{}
		if err := yaml.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		// Verify metadata fields
		if result["_server_id"] != serverID {
			t.Errorf("_server_id = %v, want %v", result["_server_id"], serverID)
		}

		if result["_deployment_mode"] != "server" {
			t.Errorf("_deployment_mode = %v, want %v", result["_deployment_mode"], "server")
		}

		// Verify original fields are preserved
		if result["id"] != "test-formation" {
			t.Errorf("id = %v, want %v", result["id"], "test-formation")
		}

		if result["name"] != "Test Formation" {
			t.Errorf("name = %v, want %v", result["name"], "Test Formation")
		}
	})

	t.Run("inject preserves complex structure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a formation.yaml with nested structure
		formationPath := filepath.Join(tmpDir, "formation.yaml")
		content := `schema: muxi.org/formation/v1
id: complex-formation
name: Complex Formation
runtime:
  built_in_mcps: true
  custom_setting: value
nested:
  key1: value1
  key2:
    - item1
    - item2`

		if err := os.WriteFile(formationPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test formation.yaml: %v", err)
		}

		// Inject metadata
		serverID := "server-complex-xyz789"
		if err := InjectMetadata(tmpDir, serverID); err != nil {
			t.Fatalf("InjectMetadata() error = %v, want nil", err)
		}

		// Read back and verify
		data, err := os.ReadFile(formationPath)
		if err != nil {
			t.Fatalf("Failed to read formation.yaml: %v", err)
		}

		var result map[string]interface{}
		if err := yaml.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		// Verify metadata
		if result["_server_id"] != serverID {
			t.Errorf("_server_id = %v, want %v", result["_server_id"], serverID)
		}

		// Verify nested structure preserved
		runtime, ok := result["runtime"].(map[string]interface{})
		if !ok {
			t.Fatal("runtime field is not a map")
		}

		if runtime["built_in_mcps"] != true {
			t.Errorf("runtime.built_in_mcps = %v, want true", runtime["built_in_mcps"])
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		nonExistentDir := filepath.Join(t.TempDir(), "nonexistent")

		err := InjectMetadata(nonExistentDir, "server-test-123")
		if err == nil {
			t.Error("InjectMetadata() with non-existent dir should fail")
			return
		}

		if !contains(err.Error(), "failed to find formation config") {
			t.Errorf("InjectMetadata() error = %q, want error containing 'failed to find formation config'", err.Error())
		}
	})

	t.Run("invalid YAML file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid YAML file
		formationPath := filepath.Join(tmpDir, "formation.yaml")
		if err := os.WriteFile(formationPath, []byte("invalid: [yaml content"), 0644); err != nil {
			t.Fatalf("Failed to create invalid formation.yaml: %v", err)
		}

		err := InjectMetadata(tmpDir, "server-test-123")
		if err == nil {
			t.Error("InjectMetadata() with invalid YAML should fail")
			return
		}

		if !contains(err.Error(), "failed to parse formation config") {
			t.Errorf("InjectMetadata() error = %q, want error containing 'failed to parse formation config'", err.Error())
		}
	})

	t.Run("multiple injections overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a test formation.yaml
		formationPath := filepath.Join(tmpDir, "formation.yaml")
		content := `id: test-formation`

		if err := os.WriteFile(formationPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test formation.yaml: %v", err)
		}

		// First injection
		if err := InjectMetadata(tmpDir, "server-first-111"); err != nil {
			t.Fatalf("First InjectMetadata() error = %v", err)
		}

		// Second injection should overwrite
		if err := InjectMetadata(tmpDir, "server-second-222"); err != nil {
			t.Fatalf("Second InjectMetadata() error = %v", err)
		}

		// Verify second value is present
		data, err := os.ReadFile(formationPath)
		if err != nil {
			t.Fatalf("Failed to read formation.yaml: %v", err)
		}

		var result map[string]interface{}
		if err := yaml.Unmarshal(data, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if result["_server_id"] != "server-second-222" {
			t.Errorf("_server_id = %v, want %v", result["_server_id"], "server-second-222")
		}
	})
}

func TestGenerateServerID(t *testing.T) {
	t.Run("generates valid format", func(t *testing.T) {
		id, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v, want nil", err)
		}

		// Verify format: server-{hostname}-{8-hex-chars}
		if !strings.HasPrefix(id, "server-") {
			t.Errorf("GenerateServerID() = %q, want prefix 'server-'", id)
		}

		parts := strings.Split(id, "-")
		if len(parts) < 3 {
			t.Errorf("GenerateServerID() = %q, want at least 3 parts separated by '-'", id)
			return
		}

		// Last part should be 8 hex characters
		hashPart := parts[len(parts)-1]
		if len(hashPart) != 8 {
			t.Errorf("Hash part length = %d, want 8", len(hashPart))
		}

		// Verify all characters are hex
		for _, c := range hashPart {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Hash part %q contains non-hex character %c", hashPart, c)
			}
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)

		// Generate 100 IDs and ensure they're all unique
		for i := 0; i < 100; i++ {
			id, err := GenerateServerID()
			if err != nil {
				t.Fatalf("GenerateServerID() error = %v, want nil", err)
			}

			if ids[id] {
				t.Errorf("GenerateServerID() generated duplicate ID: %q", id)
			}
			ids[id] = true
		}

		if len(ids) != 100 {
			t.Errorf("Generated %d unique IDs, want 100", len(ids))
		}
	})

	t.Run("consistent format across calls", func(t *testing.T) {
		id1, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v", err)
		}

		id2, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v", err)
		}

		// IDs should be different (due to timestamp)
		if id1 == id2 {
			t.Error("GenerateServerID() should generate unique IDs on each call")
		}

		// But format should be consistent
		parts1 := strings.Split(id1, "-")
		parts2 := strings.Split(id2, "-")

		if len(parts1) != len(parts2) {
			t.Errorf("ID formats differ: %q vs %q", id1, id2)
		}

		// Hostname part should be the same
		if len(parts1) >= 2 && len(parts2) >= 2 {
			hostname1 := strings.Join(parts1[1:len(parts1)-1], "-")
			hostname2 := strings.Join(parts2[1:len(parts2)-1], "-")
			if hostname1 != hostname2 {
				t.Logf("Note: Hostname parts differ: %q vs %q (this may be normal if hostname changed)", hostname1, hostname2)
			}
		}
	})

	t.Run("hostname sanitization", func(t *testing.T) {
		id, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v", err)
		}

		// Extract hostname part (between "server-" and final hash)
		parts := strings.Split(id, "-")
		if len(parts) < 3 {
			t.Fatalf("Invalid ID format: %q", id)
		}

		hostnamePart := strings.Join(parts[1:len(parts)-1], "-")

		// Verify hostname contains only valid characters
		for _, c := range hostnamePart {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				t.Errorf("Hostname part %q contains invalid character %c", hostnamePart, c)
			}
		}

		// Verify hostname is not too long (max 16 chars before hash)
		if len(hostnamePart) > 16 {
			t.Errorf("Hostname part length = %d, want <= 16", len(hostnamePart))
		}
	})

	t.Run("handles special characters", func(t *testing.T) {
		// This test verifies that even if hostname has special chars,
		// they get sanitized properly
		id, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v", err)
		}

		// Should only contain alphanumeric, dash, and the hash
		validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-"
		for _, c := range id {
			valid := false
			for _, vc := range validChars {
				if c == vc {
					valid = true
					break
				}
			}
			if !valid {
				t.Errorf("ID %q contains invalid character %c", id, c)
			}
		}
	})

	t.Run("never returns empty", func(t *testing.T) {
		id, err := GenerateServerID()
		if err != nil {
			t.Fatalf("GenerateServerID() error = %v", err)
		}

		if id == "" {
			t.Error("GenerateServerID() returned empty string")
		}

		if len(id) < 10 {
			t.Errorf("GenerateServerID() = %q, seems too short (len=%d)", id, len(id))
		}
	})
}

func TestInjectMetadata_NoFormationYAML(t *testing.T) {
	tmpDir := t.TempDir()

	err := InjectMetadata(tmpDir, "test-server-123")

	if err == nil {
		t.Error("InjectMetadata should error when formation config doesn't exist")
	}
}

func TestInjectMetadata_AlreadyHasMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "formation.yaml")

	// Create formation.yaml with existing metadata
	content := `id: meta-test
name: Metadata Test
version: 1.0.0
metadata:
  _server_id: existing-server
  _deployment_mode: server
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	err := InjectMetadata(tmpDir, "new-server-123")
	if err != nil {
		t.Fatalf("InjectMetadata() error = %v", err)
	}

	// Read back and verify metadata was updated
	data, _ := os.ReadFile(yamlPath)
	content = string(data)

	if !contains(content, "new-server-123") {
		t.Error("Metadata should be updated with new server ID")
	}
}

func TestGenerateServerID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs
	for i := 0; i < 100; i++ {
		id, _ := GenerateServerID()

		if id == "" {
			t.Error("GenerateServerID() returned empty string")
		}

		if ids[id] {
			t.Errorf("GenerateServerID() generated duplicate: %s", id)
		}
		ids[id] = true

		// Should start with "server-"
		if !contains(id, "server-") {
			t.Errorf("ID = %q, should start with server-", id)
		}
	}

	// Should have generated 100 unique IDs
	if len(ids) != 100 {
		t.Errorf("Generated %d unique IDs, want 100", len(ids))
	}
}

func TestGenerateServerID_Format(t *testing.T) {
	id, _ := GenerateServerID()

	// Check format: server-XXXXXXXX (8 hex chars)
	if len(id) < 13 {
		t.Errorf("ID length = %d, should be at least 13 (server- + 8 chars)", len(id))
	}
}

func TestInjectMetadata_FileReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory named formation.yaml (not a file)
	yamlPath := filepath.Join(tmpDir, "formation.yaml")
	os.MkdirAll(yamlPath, 0755)

	err := InjectMetadata(tmpDir, "test-server")
	if err == nil {
		t.Error("InjectMetadata should error when formation config is a directory")
	}
}

func TestInjectMetadata_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "formation.yaml")

	// Create read-only formation.yaml
	content := "id: test\nname: Test\n"
	os.WriteFile(yamlPath, []byte(content), 0444)

	err := InjectMetadata(tmpDir, "test-server")

	// May fail to write back due to permissions
	t.Logf("InjectMetadata with read-only file: %v", err)
}

func TestInjectMetadata_EmptyServerID(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "formation.yaml")

	content := "id: test\nname: Test\n"
	os.WriteFile(yamlPath, []byte(content), 0644)

	err := InjectMetadata(tmpDir, "")
	if err != nil {
		t.Fatalf("InjectMetadata() error = %v", err)
	}

	// Should still inject empty server_id
	data, _ := os.ReadFile(yamlPath)
	if !contains(string(data), "_deployment_mode") {
		t.Error("Should inject metadata even with empty server ID")
	}
}

func TestInjectMetadata_ComplexYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "formation.yaml")

	// Complex YAML with nested structures
	content := `id: complex
name: Complex Formation
version: 1.0.0
runtime:
  command: python
  args:
    - main.py
    - --verbose
environment:
  DEBUG: "true"
  PORT: "8080"
`
	os.WriteFile(yamlPath, []byte(content), 0644)

	err := InjectMetadata(tmpDir, "server-complex-123")
	if err != nil {
		t.Fatalf("InjectMetadata() error = %v", err)
	}

	// Verify metadata injected
	data, _ := os.ReadFile(yamlPath)
	content = string(data)

	if !contains(content, "server-complex-123") {
		t.Error("Server ID not injected")
	}

	if !contains(content, "_deployment_mode") {
		t.Error("Deployment mode not injected")
	}

	// Verify existing content preserved
	if !contains(content, "DEBUG") {
		t.Error("Existing environment should be preserved")
	}
}
