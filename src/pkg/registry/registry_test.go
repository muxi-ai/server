package registry_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/rs/zerolog"
)

// TestPortPool tests port allocation and release
func TestPortPool(t *testing.T) {
	pool, err := registry.NewPortPool(8000, 8010)
	if err != nil {
		t.Fatalf("Failed to create port pool: %v", err)
	}

	t.Run("AllocatePort", func(t *testing.T) {
		port, err := pool.Allocate("formation-1")
		if err != nil {
			t.Fatalf("Failed to allocate port: %v", err)
		}

		if port < 8000 || port >= 8010 {
			t.Errorf("Port %d out of range 8000-8010", port)
		}

		t.Logf("✓ Allocated port %d", port)
	})

	t.Run("AllocateMultiple", func(t *testing.T) {
		ports := make(map[int]bool)
		
		for i := 1; i < 10; i++ {
			port, err := pool.Allocate(fmt.Sprintf("formation-%d", i+1))
			if err != nil {
				t.Fatalf("Failed to allocate port %d: %v", i+1, err)
			}

			if ports[port] {
				t.Errorf("Port %d allocated twice!", port)
			}
			ports[port] = true
		}

		t.Logf("✓ Allocated %d unique ports", len(ports))
	})

	t.Run("Exhaust", func(t *testing.T) {
		// Pool should be full now (10 ports, all allocated)
		_, err := pool.Allocate("formation-extra")
		if err == nil {
			t.Error("Expected error when pool exhausted, got nil")
		}

		t.Log("✓ Port pool exhaustion handled correctly")
	})

	t.Run("Release", func(t *testing.T) {
		// Release a port
		pool.ReleaseByFormation("formation-5")

		// Should be able to allocate again
		port, err := pool.Allocate("formation-new")
		if err != nil {
			t.Fatalf("Failed to allocate after release: %v", err)
		}

		t.Logf("✓ Released and re-allocated port %d", port)
	})
}

// TestRegistry tests basic registry operations
func TestRegistry(t *testing.T) {
	reg, err := registry.NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	t.Run("Register", func(t *testing.T) {
		formation := &registry.Formation{
			ID:         "test-formation",
			Name:       "Test Formation",
			Status:     "running",
			DeployedAt: time.Now(),
		}

		err := reg.Register(formation)
		if err != nil {
			t.Fatalf("Failed to register formation: %v", err)
		}

		// Port should have been allocated
		if formation.Port == 0 {
			t.Error("Port was not allocated")
		}

		t.Logf("✓ Registered formation with port %d", formation.Port)
	})

	t.Run("Get", func(t *testing.T) {
		formation, err := reg.Get("test-formation")
		if err != nil {
			t.Fatalf("Failed to get formation: %v", err)
		}

		if formation.ID != "test-formation" {
			t.Errorf("Expected ID 'test-formation', got '%s'", formation.ID)
		}

		t.Log("✓ Retrieved formation successfully")
	})

	t.Run("List", func(t *testing.T) {
		formations := reg.List()
		if len(formations) != 1 {
			t.Errorf("Expected 1 formation, got %d", len(formations))
		}

		t.Logf("✓ Listed %d formation(s)", len(formations))
	})

	t.Run("Update", func(t *testing.T) {
		err := reg.Update("test-formation", func(f *registry.Formation) {
			f.Status = "stopped"
			f.RestartCount = 5
		})

		if err != nil {
			t.Fatalf("Failed to update formation: %v", err)
		}

		formation, _ := reg.Get("test-formation")
		if formation.Status != "stopped" {
			t.Errorf("Expected status 'stopped', got '%s'", formation.Status)
		}
		if formation.RestartCount != 5 {
			t.Errorf("Expected restart count 5, got %d", formation.RestartCount)
		}

		t.Log("✓ Updated formation successfully")
	})

	t.Run("Unregister", func(t *testing.T) {
		err := reg.Unregister("test-formation")
		if err != nil {
			t.Fatalf("Failed to unregister formation: %v", err)
		}

		// Should not be found
		_, err = reg.Get("test-formation")
		if err == nil {
			t.Error("Expected error when getting unregistered formation")
		}

		t.Log("✓ Unregistered formation successfully")
	})
}

// TestPersistence tests saving and loading registry
func TestPersistence(t *testing.T) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	tmpDir := t.TempDir()
	registryFile := filepath.Join(tmpDir, "registry.json")

	// Create registry with some formations
	reg, _ := registry.NewRegistry(8000, 8100)
	
	formations := []*registry.Formation{
		{
			ID:         "formation-1",
			Name:       "Formation 1",
			Port:       8001,
			Status:     "running",
			ProcessID:  12345,
			DeployedAt: time.Now(),
		},
		{
			ID:         "formation-2",
			Name:       "Formation 2",
			Port:       8002,
			Status:     "stopped",
			DeployedAt: time.Now(),
		},
	}

	for _, f := range formations {
		if err := reg.Register(f); err != nil {
			t.Fatalf("Failed to register formation: %v", err)
		}
	}

	t.Run("Save", func(t *testing.T) {
		persist := registry.NewPersistence(reg, registryFile, &logger)
		
		err := persist.Save()
		if err != nil {
			t.Fatalf("Failed to save registry: %v", err)
		}

		// Check file exists
		if _, err := os.Stat(registryFile); os.IsNotExist(err) {
			t.Error("Registry file was not created")
		}

		t.Log("✓ Saved registry to file")
	})

	t.Run("Load", func(t *testing.T) {
		// Create new registry
		newReg, _ := registry.NewRegistry(8000, 8100)
		persist := registry.NewPersistence(newReg, registryFile, &logger)

		err := persist.Load()
		if err != nil {
			t.Fatalf("Failed to load registry: %v", err)
		}

		// Check formations were loaded
		loaded := newReg.List()
		if len(loaded) != 2 {
			t.Errorf("Expected 2 formations, got %d", len(loaded))
		}

		// Verify data
		formation, err := newReg.Get("formation-1")
		if err != nil {
			t.Fatalf("Failed to get formation-1: %v", err)
		}

		if formation.Port != 8001 {
			t.Errorf("Expected port 8001, got %d", formation.Port)
		}

		t.Log("✓ Loaded registry from file")
	})
}

// TestConfig tests configuration loading
func TestConfig(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		cfg := config.DefaultConfig()

		if cfg.Server.Port != 3000 {
			t.Errorf("Expected default port 3000, got %d", cfg.Server.Port)
		}

		if cfg.Formations.PortRangeStart != 8000 {
			t.Errorf("Expected port range start 8000, got %d", cfg.Formations.PortRangeStart)
		}

		t.Log("✓ Default config values correct")
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		// Create and save config
		cfg := config.DefaultConfig()
		cfg.Server.Port = 4000
		cfg.Formations.PortRangeStart = 9000

		err := cfg.Save(configPath)
		if err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Load config
		loaded, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if loaded.Server.Port != 4000 {
			t.Errorf("Expected port 4000, got %d", loaded.Server.Port)
		}

		if loaded.Formations.PortRangeStart != 9000 {
			t.Errorf("Expected port range start 9000, got %d", loaded.Formations.PortRangeStart)
		}

		t.Log("✓ Config save/load works correctly")
	})
}
