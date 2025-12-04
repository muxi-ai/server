package formation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFormationYAML(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
		validate    func(*testing.T, *Formation)
	}{
		{
			name: "valid formation",
			content: `schema: muxi.org/formation/v1
id: test-formation
name: Test Formation
description: A test formation
version: 1.0.0
muxi_runtime: "1.2.3"`,
			wantErr: false,
			validate: func(t *testing.T, f *Formation) {
				if f.Schema != "muxi.org/formation/v1" {
					t.Errorf("Schema = %q, want %q", f.Schema, "muxi.org/formation/v1")
				}
				if f.ID != "test-formation" {
					t.Errorf("ID = %q, want %q", f.ID, "test-formation")
				}
				if f.Name != "Test Formation" {
					t.Errorf("Name = %q, want %q", f.Name, "Test Formation")
				}
				if f.Description != "A test formation" {
					t.Errorf("Description = %q, want %q", f.Description, "A test formation")
				}
				if f.Version != "1.0.0" {
					t.Errorf("Version = %q, want %q", f.Version, "1.0.0")
				}
				if f.MuxiRuntime != "1.2.3" {
					t.Errorf("MuxiRuntime = %q, want %q", f.MuxiRuntime, "1.2.3")
				}
			},
		},
		{
			name: "minimal valid formation",
			content: `id: minimal-formation
name: Minimal
description: A minimal test formation`,
			wantErr: false,
			validate: func(t *testing.T, f *Formation) {
				if f.ID != "minimal-formation" {
					t.Errorf("ID = %q, want %q", f.ID, "minimal-formation")
				}
				if f.Name != "Minimal" {
					t.Errorf("Name = %q, want %q", f.Name, "Minimal")
				}
				if f.MuxiRuntime != "latest" {
					t.Errorf("MuxiRuntime = %q, want %q (default)", f.MuxiRuntime, "latest")
				}
			},
		},
		{
			name:        "missing ID field",
			content:     `name: No ID Formation`,
			wantErr:     true,
			errContains: "must have an 'id' field",
		},
		{
			name:        "empty ID field",
			content:     `id: ""`,
			wantErr:     true,
			errContains: "must have an 'id' field",
		},
		{
			name:        "invalid YAML syntax",
			content:     `id: test\ninvalid yaml: [`,
			wantErr:     true,
			errContains: "failed to parse formation.yaml",
		},
		{
			name:        "non-existent file",
			content:     "",
			wantErr:     true,
			errContains: "failed to read formation.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string

			if tt.name == "non-existent file" {
				// Use a path that doesn't exist
				path = filepath.Join(t.TempDir(), "nonexistent", "formation.yaml")
			} else {
				// Create temp file with content
				tmpDir := t.TempDir()
				path = filepath.Join(tmpDir, "formation.yaml")
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			formation, err := ParseFormationYAML(path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFormationYAML() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseFormationYAML() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseFormationYAML() unexpected error: %v", err)
				return
			}

			if formation == nil {
				t.Error("ParseFormationYAML() returned nil formation")
				return
			}

			if tt.validate != nil {
				tt.validate(t, formation)
			}
		})
	}
}

func TestFormation_GetDefaultCommand(t *testing.T) {
	f := &Formation{ID: "test"}
	cmd := f.GetDefaultCommand()

	if cmd != "python" {
		t.Errorf("GetDefaultCommand() = %q, want %q", cmd, "python")
	}
}

func TestFormation_GetDefaultArgs(t *testing.T) {
	f := &Formation{ID: "test"}
	args := f.GetDefaultArgs()

	if len(args) != 1 {
		t.Errorf("GetDefaultArgs() len = %d, want 1", len(args))
		return
	}

	if args[0] != "app.py" {
		t.Errorf("GetDefaultArgs()[0] = %q, want %q", args[0], "app.py")
	}
}

func TestFormation_GetEnvironmentVars(t *testing.T) {
	f := &Formation{
		ID:   "test-formation-123",
		Name: "Test Formation",
	}

	port := 8080
	serverURL := "http://localhost:7890"
	bindHost := "127.0.0.1"

	env := f.GetEnvironmentVars(port, serverURL, bindHost)

	tests := []struct {
		key   string
		value string
	}{
		{"PORT", "8080"},
		{"HOST", "127.0.0.1"},
		{"FORMATION_ID", "test-formation-123"},
		{"MUXI_SERVER_URL", "http://localhost:7890"},
		{"MUXI_ENV", "production"},
		{"_bind_host", "127.0.0.1"},
		{"_port", "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, exists := env[tt.key]
			if !exists {
				t.Errorf("Environment variable %q not found", tt.key)
				return
			}
			if got != tt.value {
				t.Errorf("env[%q] = %q, want %q", tt.key, got, tt.value)
			}
		})
	}

	// Ensure we have exactly the expected number of variables
	if len(env) != 7 {
		t.Errorf("GetEnvironmentVars() returned %d variables, want 7", len(env))
		t.Logf("Variables: %+v", env)
	}
}

func TestFormation_GetEnvironmentVars_DifferentPorts(t *testing.T) {
	f := &Formation{ID: "test"}

	tests := []struct {
		port int
		want string
	}{
		{8000, "8000"},
		{8080, "8080"},
		{9000, "9000"},
		{3000, "3000"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			env := f.GetEnvironmentVars(tt.port, "http://localhost:7890", "127.0.0.1")
			if env["PORT"] != tt.want {
				t.Errorf("PORT = %q, want %q", env["PORT"], tt.want)
			}
		})
	}
}

func TestFormation_GetEnvironmentVars_DifferentServerURLs(t *testing.T) {
	f := &Formation{ID: "test"}

	tests := []struct {
		name      string
		serverURL string
	}{
		{"localhost", "http://localhost:3000"},
		{"custom domain", "https://muxi.example.com"},
		{"IP address", "http://192.168.1.100:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := f.GetEnvironmentVars(8080, tt.serverURL, "127.0.0.1")
			if env["MUXI_SERVER_URL"] != tt.serverURL {
				t.Errorf("MUXI_SERVER_URL = %q, want %q", env["MUXI_SERVER_URL"], tt.serverURL)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
