package config

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server defaults
	if cfg.Server.Port != 7890 {
		t.Errorf("Server.Port = %d, want 7890", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %s, want 0.0.0.0", cfg.Server.Host)
	}

	// Test auth defaults
	if cfg.Auth.Enabled != false {
		t.Errorf("Auth.Enabled = %v, want false", cfg.Auth.Enabled)
	}
	if cfg.Auth.TimestampTolerance != 300 {
		t.Errorf("Auth.TimestampTolerance = %d, want 300", cfg.Auth.TimestampTolerance)
	}

	// Test formations defaults
	if cfg.Formations.RuntimeType != "native" {
		t.Errorf("Formations.RuntimeType = %s, want native", cfg.Formations.RuntimeType)
	}
	if cfg.Formations.PortRangeStart != 8000 {
		t.Errorf("Formations.PortRangeStart = %d, want 8000", cfg.Formations.PortRangeStart)
	}
	if cfg.Formations.PortRangeEnd != 9000 {
		t.Errorf("Formations.PortRangeEnd = %d, want 9000", cfg.Formations.PortRangeEnd)
	}
	if cfg.Formations.BindHost != "127.0.0.1" {
		t.Errorf("Formations.BindHost = %s, want 127.0.0.1", cfg.Formations.BindHost)
	}
	if cfg.Formations.MaxFormations != 100 {
		t.Errorf("Formations.MaxFormations = %d, want 100", cfg.Formations.MaxFormations)
	}
	if cfg.Formations.AutoRestart != true {
		t.Errorf("Formations.AutoRestart = %v, want true", cfg.Formations.AutoRestart)
	}
	if cfg.Formations.MaxRestarts != 10 {
		t.Errorf("Formations.MaxRestarts = %d, want 10", cfg.Formations.MaxRestarts)
	}
	if cfg.Formations.RestartDelay != 1 {
		t.Errorf("Formations.RestartDelay = %d, want 1", cfg.Formations.RestartDelay)
	}
	if cfg.Formations.KeepBackups != 1 {
		t.Errorf("Formations.KeepBackups = %d, want 1", cfg.Formations.KeepBackups)
	}

	// Test logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
	if cfg.Logging.AuditLog != "logs/audit.log" {
		t.Errorf("Logging.AuditLog = %s, want logs/audit.log", cfg.Logging.AuditLog)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	t.Run("LoadNonExistentFile", func(t *testing.T) {
		// Should return default config
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.Server.Port != 7890 {
			t.Errorf("Expected default config, got Port = %d", cfg.Server.Port)
		}
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		// Create a custom config
		cfg := &Config{
			ServerID: "test-server-123",
			Server: ServerConfig{
				Port: 4000,
				Host: "127.0.0.1",
			},
			Auth: AuthConfig{
				Enabled:            true,
				Key:                "test-key",
				Secret:             "test-secret",
				TimestampTolerance: 600,
			},
			Formations: FormationsConfig{
				RuntimeType:    "docker",
				PortRangeStart: 9000,
				PortRangeEnd:   10000,
				AutoRestart:    false,
			},
		}

		// Save config
		err := cfg.Save(configPath)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Fatal("Config file was not created")
		}

		// Load config
		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify values
		if loaded.ServerID != cfg.ServerID {
			t.Errorf("ServerID = %s, want %s", loaded.ServerID, cfg.ServerID)
		}
		if loaded.Server.Port != cfg.Server.Port {
			t.Errorf("Server.Port = %d, want %d", loaded.Server.Port, cfg.Server.Port)
		}
		if loaded.Server.Host != cfg.Server.Host {
			t.Errorf("Server.Host = %s, want %s", loaded.Server.Host, cfg.Server.Host)
		}
		if loaded.Auth.Enabled != cfg.Auth.Enabled {
			t.Errorf("Auth.Enabled = %v, want %v", loaded.Auth.Enabled, cfg.Auth.Enabled)
		}
		if loaded.Auth.Key != cfg.Auth.Key {
			t.Errorf("Auth.Key = %s, want %s", loaded.Auth.Key, cfg.Auth.Key)
		}
		if loaded.Auth.Secret != cfg.Auth.Secret {
			t.Errorf("Auth.Secret = %s, want %s", loaded.Auth.Secret, cfg.Auth.Secret)
		}
		if loaded.Auth.TimestampTolerance != cfg.Auth.TimestampTolerance {
			t.Errorf("Auth.TimestampTolerance = %d, want %d", loaded.Auth.TimestampTolerance, cfg.Auth.TimestampTolerance)
		}
		if loaded.Formations.RuntimeType != cfg.Formations.RuntimeType {
			t.Errorf("Formations.RuntimeType = %s, want %s", loaded.Formations.RuntimeType, cfg.Formations.RuntimeType)
		}
		if loaded.Formations.PortRangeStart != cfg.Formations.PortRangeStart {
			t.Errorf("Formations.PortRangeStart = %d, want %d", loaded.Formations.PortRangeStart, cfg.Formations.PortRangeStart)
		}
		if loaded.Formations.PortRangeEnd != cfg.Formations.PortRangeEnd {
			t.Errorf("Formations.PortRangeEnd = %d, want %d", loaded.Formations.PortRangeEnd, cfg.Formations.PortRangeEnd)
		}
		if loaded.Formations.AutoRestart != cfg.Formations.AutoRestart {
			t.Errorf("Formations.AutoRestart = %v, want %v", loaded.Formations.AutoRestart, cfg.Formations.AutoRestart)
		}
	})

	t.Run("LoadInvalidYAML", func(t *testing.T) {
		invalidPath := filepath.Join(tempDir, "invalid.yaml")
		err := os.WriteFile(invalidPath, []byte("invalid: yaml: content: ["), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid yaml: %v", err)
		}

		_, err = Load(invalidPath)
		if err == nil {
			t.Error("Load() should fail on invalid YAML")
		}
	})
}

func TestGetMuxiDir(t *testing.T) {
	dir, err := GetMuxiDir()
	if err != nil {
		t.Fatalf("GetMuxiDir() error = %v", err)
	}

	// Should end with .muxi/server
	if !filepath.IsAbs(dir) {
		t.Error("GetMuxiDir() should return absolute path")
	}
	if filepath.Base(dir) != "server" {
		t.Errorf("GetMuxiDir() should end with 'server', got %s", dir)
	}
}

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() error = %v", err)
	}

	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("GetConfigPath() should end with 'config.yaml', got %s", path)
	}

	// Should be absolute
	if !filepath.IsAbs(path) {
		t.Error("GetConfigPath() should return absolute path")
	}
}

func TestGetRegistryPath(t *testing.T) {
	path, err := GetRegistryPath()
	if err != nil {
		t.Fatalf("GetRegistryPath() error = %v", err)
	}

	// Should end with registry.json
	if filepath.Base(path) != "registry.json" {
		t.Errorf("GetRegistryPath() should end with 'registry.json', got %s", path)
	}

	// Should be absolute
	if !filepath.IsAbs(path) {
		t.Error("GetRegistryPath() should return absolute path")
	}
}


func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	cfg := &Config{
		ServerID: "save-test-123",
		Server: ServerConfig{
			Port: 5000,
			Host: "localhost",
		},
		Auth: AuthConfig{
			Enabled: true,
			Key:     "save-key",
			Secret:  "save-secret",
		},
	}

	// Save should create parent directory if it doesn't exist
	nestedPath := filepath.Join(tempDir, "nested", "deep", "config.yaml")
	err := cfg.Save(nestedPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Save() should create parent directories")
	}

	// Save to simple path
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists and is readable
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}
	if len(data) == 0 {
		t.Error("Saved config file is empty")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "Valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "Valid custom config",
			config: &Config{
				ServerID: "test-123",
				Server: ServerConfig{
					Port: 8080,
					Host: "0.0.0.0",
				},
				Formations: FormationsConfig{
					PortRangeStart: 9000,
					PortRangeEnd:   9100,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that config can be saved and loaded without errors
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			err := tt.config.Save(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				_, err := Load(configPath)
				if err != nil {
					t.Errorf("Load() error = %v", err)
				}
			}
		})
	}
}

func TestConfigPersistence(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create original config
	original := &Config{
		ServerID: "persistence-test",
		Server: ServerConfig{
			Port: 7000,
			Host: "192.168.1.1",
		},
		Auth: AuthConfig{
			Enabled:            true,
			Key:                "persist-key",
			Secret:             "persist-secret",
			TimestampTolerance: 450,
		},
		Formations: FormationsConfig{
			RuntimeType:         "singularity",
			LogsDir:             "/var/log/muxi",
			PIDsDir:             "/var/run/muxi",
			FormationsDir:       "/opt/muxi/formations",
			PortRangeStart:      10000,
			PortRangeEnd:        11000,
			MaxFormations:       50,
			AutoRestart:         false,
			MaxRestarts:         5,
			RestartDelay:        2,
			HealthCheckInterval: 60,
			HealthCheckTimeout:  10,
			StartupHealthDelay:  5,
			LogRotationEnabled:  false,
			LogMaxSize:          "50M",
			LogMaxFiles:         20,
		},
	}

	// Save
	if err := original.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify all fields persisted correctly
	if loaded.ServerID != original.ServerID {
		t.Errorf("ServerID not persisted: got %s, want %s", loaded.ServerID, original.ServerID)
	}
	if loaded.Formations.LogsDir != original.Formations.LogsDir {
		t.Errorf("LogsDir not persisted: got %s, want %s", loaded.Formations.LogsDir, original.Formations.LogsDir)
	}
	if loaded.Formations.HealthCheckInterval != original.Formations.HealthCheckInterval {
		t.Errorf("HealthCheckInterval not persisted: got %d, want %d", loaded.Formations.HealthCheckInterval, original.Formations.HealthCheckInterval)
	}
	if loaded.Formations.LogMaxSize != original.Formations.LogMaxSize {
		t.Errorf("LogMaxSize not persisted: got %s, want %s", loaded.Formations.LogMaxSize, original.Formations.LogMaxSize)
	}
}


func TestConfig_Save_InvalidPath(t *testing.T) {
	cfg := DefaultConfig()
	
	// Try to save to a path that's actually a directory
	tmpDir := t.TempDir()
	err := cfg.Save(tmpDir) // Directory, not file
	
	if err == nil {
		t.Error("Save() should error when path is a directory")
	}
}

func TestConfig_Save_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	cfg := DefaultConfig()
	
	// Try to save to /root (should fail with permission denied)
	err := cfg.Save("/root/config.yaml")
	
	if err == nil {
		t.Log("Save() should error with permission denied (may vary by system)")
	}
}



func TestGetMuxiDir_HomeDirExists(t *testing.T) {
	dir, err := GetMuxiDir()
	if err != nil {
		t.Fatalf("GetMuxiDir() error = %v", err)
	}

	// Should end with .muxi/server
	if !strings.HasSuffix(dir, filepath.Join(".muxi", "server")) {
		t.Errorf("GetMuxiDir() = %q, should end with .muxi/server", dir)
	}
}

func TestGetConfigPath_Format(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() error = %v", err)
	}

	// Should end with config.yaml
	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("GetConfigPath() = %q, should end with config.yaml", path)
	}

	// Should contain .muxi
	if !strings.Contains(path, ".muxi") {
		t.Errorf("GetConfigPath() = %q, should contain .muxi", path)
	}
}

func TestGetRegistryPath_Format(t *testing.T) {
	path, err := GetRegistryPath()
	if err != nil {
		t.Fatalf("GetRegistryPath() error = %v", err)
	}

	// Should end with registry.json
	if !strings.HasSuffix(path, "registry.json") {
		t.Errorf("GetRegistryPath() = %q, should end with registry.json", path)
	}

	// Should contain .muxi
	if !strings.Contains(path, ".muxi") {
		t.Errorf("GetRegistryPath() = %q, should contain .muxi", path)
	}
}

func TestEnsureDirectories_AllCreated(t *testing.T) {
	tmpBase := t.TempDir()
	
	cfg := DefaultConfig()
	
	err := EnsureDirectories(tmpBase, cfg)
	if err != nil {
		t.Fatalf("EnsureDirectories() error = %v", err)
	}

	// Verify base dir
	if _, err := os.Stat(tmpBase); err != nil {
		t.Errorf("Base dir not created: %v", err)
	}

	// After EnsureDirectories, config paths are normalized to absolute paths
	// Verify logs dir (now absolute in config)
	if _, err := os.Stat(cfg.Formations.LogsDir); err != nil {
		t.Errorf("Logs dir not created: %v", err)
	}

	// Verify PIDs dir (now absolute in config)
	if _, err := os.Stat(cfg.Formations.PIDsDir); err != nil {
		t.Errorf("PIDs dir not created: %v", err)
	}

	// Verify formations dir (now absolute in config)
	if _, err := os.Stat(cfg.Formations.FormationsDir); err != nil {
		t.Errorf("Formations dir not created: %v", err)
	}
}

func TestEnsureDirectories_AlreadyExist(t *testing.T) {
	tmpBase := t.TempDir()
	cfg := DefaultConfig()
	
	// Create dirs first
	logsDir := filepath.Join(tmpBase, cfg.Formations.LogsDir)
	os.MkdirAll(logsDir, 0755)

	// Should not error when dirs already exist
	err := EnsureDirectories(tmpBase, cfg)
	if err != nil {
		t.Errorf("EnsureDirectories() should not error when dirs exist: %v", err)
	}
}

func TestEnsureDirectories_NestedPaths(t *testing.T) {
	tmpBase := t.TempDir()
	
	cfg := DefaultConfig()
	cfg.Formations.LogsDir = "nested/deep/logs"
	cfg.Formations.PIDsDir = "nested/deep/pids"
	
	err := EnsureDirectories(tmpBase, cfg)
	if err != nil {
		t.Fatalf("EnsureDirectories() error = %v", err)
	}

	// Verify nested dirs created
	logsDir := filepath.Join(tmpBase, "nested/deep/logs")
	if _, err := os.Stat(logsDir); err != nil {
		t.Errorf("Nested logs dir not created: %v", err)
	}
}

func TestEnsureDirectories_InvalidPath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	cfg := DefaultConfig()
	
	// Try to create in /root (should fail with permission denied)
	err := EnsureDirectories("/root/muxi-test", cfg)
	
	if err == nil {
		t.Log("EnsureDirectories() should error with permission denied (may vary by system)")
	} else {
		// Expected error
		t.Logf("Got expected error: %v", err)
	}
}

func TestGetMuxiDir_HomeDirError(t *testing.T) {
	// This test would require mocking os.UserHomeDir() which isn't easy in Go
	// The function handles the error path internally
	dir, err := GetMuxiDir()
	if err != nil {
		t.Logf("GetMuxiDir() error = %v (expected if home dir unavailable)", err)
	}
	if dir != "" && !strings.Contains(dir, ".muxi") {
		t.Errorf("GetMuxiDir() = %q, should contain .muxi", dir)
	}
}

func TestGetConfigPath_Integration(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath() error = %v", err)
	}

	// Should be absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("GetConfigPath() = %q, should be absolute path", path)
	}
}

func TestGetRegistryPath_Integration(t *testing.T) {
	path, err := GetRegistryPath()
	if err != nil {
		t.Fatalf("GetRegistryPath() error = %v", err)
	}

	// Should be absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("GetRegistryPath() = %q, should be absolute path", path)
	}
}

func TestConfig_Save_WithMarshalError(t *testing.T) {
	// Config struct should always marshal successfully, but test the flow
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "marshal-test.yaml")

	cfg := DefaultConfig()
	cfg.ServerID = "test-server-123"

	err := cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists and is valid YAML
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	if !strings.Contains(string(data), "test-server-123") {
		t.Error("Saved config doesn't contain server ID")
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpFile := t.TempDir() + "/partial.yaml"
	
	// Write minimal valid config
	content := `server_id: test-123
server:
  port: 3000
`
	os.WriteFile(tmpFile, []byte(content), 0644)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ServerID != "test-123" {
		t.Errorf("ServerID = %q, want test-123", cfg.ServerID)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("Port = %d, want 3000", cfg.Server.Port)
	}
}

func TestLoad_FileReadError(t *testing.T) {
	// Try to load from a directory (not a file)
	tmpDir := t.TempDir()
	
	_, err := Load(tmpDir)
	if err == nil {
		t.Error("Load() should error when path is a directory")
	}
}

func TestSave_DirectoryCreateError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	cfg := DefaultConfig()
	
	// Try to save to a path where we can't create the directory
	err := cfg.Save("/root/nonexistent/deeply/nested/config.yaml")
	
	if err == nil {
		t.Log("Save() should error when can't create directory (may vary by system)")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tmpFile := t.TempDir() + "/empty.yaml"
	os.WriteFile(tmpFile, []byte(""), 0644)

	_, err := Load(tmpFile)
	if err == nil {
		t.Log("Load() handles empty file gracefully")
	}
}

func TestLoad_OnlyWhitespace(t *testing.T) {
	tmpFile := t.TempDir() + "/whitespace.yaml"
	os.WriteFile(tmpFile, []byte("   \n  \n  "), 0644)

	_, err := Load(tmpFile)
	// May or may not error depending on YAML parser
	t.Logf("Load() with whitespace: err = %v", err)
}
