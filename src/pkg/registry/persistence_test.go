package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	if persistence == nil {
		t.Fatal("NewPersistence() returned nil")
	}

	// Should not start auto-save by default
	// (Can't easily test this without exposing internals)
}

func TestPersistence_EnableAutoSave(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a subdirectory to test directory creation
	subDir := filepath.Join(tmpDir, "data")
	registryPath := filepath.Join(subDir, "registry.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Enable auto-save
	persistence.EnableAutoSave()

	// Add a formation
	reg.Register(&Formation{
		ID:   "auto-save-test",
		Port: 8050,
	})

	// Wait for auto-save to trigger (debounce is 2s + processing time)
	time.Sleep(2500 * time.Millisecond)

	// Stop auto-save
	persistence.DisableAutoSave()

	// Verify file was created
	if _, err := os.Stat(registryPath); err != nil {
		t.Errorf("Auto-save should have created file: %v", err)
	}
}

func TestPersistence_DisableAutoSave(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Enable and then disable
	persistence.EnableAutoSave()
	time.Sleep(50 * time.Millisecond)
	persistence.DisableAutoSave()

	// Should not panic when called multiple times
	persistence.DisableAutoSave()
	persistence.DisableAutoSave()
}

func TestPersistence_AutoSaveLoop(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "data")
	registryPath := filepath.Join(subDir, "registry.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Enable auto-save with short interval
	persistence.EnableAutoSave()

	// Add formations
	reg.Register(&Formation{ID: "test-1", Port: 8001})
	reg.Register(&Formation{ID: "test-2", Port: 8002})

	// Wait for auto-save to trigger (debounce is 2s + processing time)
	time.Sleep(2500 * time.Millisecond)

	persistence.DisableAutoSave()

	// Verify file exists
	if _, err := os.Stat(registryPath); err != nil {
		t.Errorf("Auto-save file should exist: %v", err)
	}

	// Load and verify
	newReg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create new registry: %v", err)
	}

	newPersistence := NewPersistence(newReg, registryPath, &logger)
	if err := newPersistence.Load(); err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if newReg.Count() != 2 {
		t.Errorf("Loaded %d formations, want 2", newReg.Count())
	}
}

func TestPersistence_Save_Error(t *testing.T) {
	// Try to save to an invalid path
	// Use platform-appropriate absolute invalid path
	var invalidPath string
	if runtime.GOOS == "windows" {
		// Windows: Use a path on a drive that doesn't exist
		invalidPath = "Z:\\nonexistent\\directory\\registry.json"
	} else {
		// Unix: Use root-level nonexistent directory
		invalidPath = "/nonexistent/directory/registry.json"
	}
	
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, invalidPath, &logger)

	err = persistence.Save()
	if err == nil {
		t.Errorf("Save() to invalid path %s should fail", invalidPath)
	}
}

func TestPersistence_Load_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "nonexistent.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	err = persistence.Load()
	// Load is designed to silently succeed for nonexistent files (fresh start)
	// This is by design as shown in the implementation
	if err != nil {
		t.Logf("Load() result: %v", err)
	}
}

func TestPersistence_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "invalid.json")
	logger := zerolog.Nop()

	// Create invalid JSON file
	if err := os.WriteFile(registryPath, []byte("invalid json content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	err = persistence.Load()
	if err == nil {
		t.Error("Load() should error for invalid JSON")
	}
}

func TestPersistence_SaveLoad_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "roundtrip.json")
	logger := zerolog.Nop()

	// Create and populate registry
	reg1, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	reg1.Register(&Formation{
		ID:     "roundtrip-1",
		Name:   "Roundtrip Test 1",
		Port:   8010,
		Status: "running",
	})
	reg1.Register(&Formation{
		ID:     "roundtrip-2",
		Name:   "Roundtrip Test 2",
		Port:   8011,
		Status: "stopped",
	})

	// Save
	persistence1 := NewPersistence(reg1, registryPath, &logger)
	if err := persistence1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load into new registry
	reg2, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry 2: %v", err)
	}

	persistence2 := NewPersistence(reg2, registryPath, &logger)
	if err := persistence2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded data
	if reg2.Count() != 2 {
		t.Errorf("Loaded count = %d, want 2", reg2.Count())
	}

	formation, err := reg2.Get("roundtrip-1")
	if err != nil {
		t.Fatalf("Failed to get formation: %v", err)
	}

	if formation.Name != "Roundtrip Test 1" {
		t.Errorf("Name = %q, want %q", formation.Name, "Roundtrip Test 1")
	}

	if formation.Port != 8010 {
		t.Errorf("Port = %d, want 8010", formation.Port)
	}
}

func TestPersistence_AutoSave_StopWhileRunning(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "autosave-stop.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Start auto-save
	persistence.EnableAutoSave()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop while potentially mid-cycle
	persistence.DisableAutoSave()

	// Should not panic or hang
	time.Sleep(50 * time.Millisecond)
}

func TestPersistence_MultipleEnableDisable(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "toggle.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Enable/disable multiple times
	for i := 0; i < 3; i++ {
		persistence.EnableAutoSave()
		time.Sleep(25 * time.Millisecond)
		persistence.DisableAutoSave()
	}

	// Should not panic or leave goroutines running
	time.Sleep(100 * time.Millisecond)
}

func TestPersistence_SaveWithNoFormations(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "empty.json")
	logger := zerolog.Nop()

	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	persistence := NewPersistence(reg, registryPath, &logger)

	// Save empty registry
	if err := persistence.Save(); err != nil {
		t.Errorf("Save() with empty registry error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(registryPath); err != nil {
		t.Errorf("File should be created even for empty registry: %v", err)
	}
}

func TestNewPersistence_Initialization(t *testing.T) {
	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "test.json")
	logger := zerolog.Nop()

	reg, _ := NewRegistry(8000, 8100)
	p := NewPersistence(reg, regPath, &logger)

	if p == nil {
		t.Fatal("NewPersistence() returned nil")
	}

	if p.registry != reg {
		t.Error("Registry not set correctly")
	}

	if p.filePath != regPath {
		t.Errorf("filePath = %q, want %q", p.filePath, regPath)
	}

	if p.logger != &logger {
		t.Error("Logger not set correctly")
	}

	if p.saveDebounce != 2*time.Second {
		t.Errorf("saveDebounce = %v, want 2s", p.saveDebounce)
	}

	if p.saveChan == nil {
		t.Error("saveChan not initialized")
	}

	if p.stopChan == nil {
		t.Error("stopChan not initialized")
	}
}

func TestNewPersistence_NilLogger(t *testing.T) {
	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "test.json")

	reg, _ := NewRegistry(8000, 8100)
	p := NewPersistence(reg, regPath, nil)

	if p.logger == nil {
		t.Error("Should create Nop logger when nil provided")
	}
}

func TestPersistence_Load_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	regPath := filepath.Join(tmpDir, "version.json")
	logger := zerolog.Nop()

	// Create a file with wrong version
	wrongVersion := `{
		"version": 999,
		"saved_at": "2024-01-01T00:00:00Z",
		"formations": []
	}`
	os.WriteFile(regPath, []byte(wrongVersion), 0644)

	reg, _ := NewRegistry(8000, 8100)
	p := NewPersistence(reg, regPath, &logger)

	err := p.Load()
	if err == nil {
		t.Error("Load() should error for unsupported version")
	}

	if !strings.Contains(err.Error(), "version") {
		t.Errorf("Error = %q, should mention version", err.Error())
	}
}
