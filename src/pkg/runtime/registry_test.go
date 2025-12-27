package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	path := "/tmp/test-registry.json"
	reg := NewRegistry(path)

	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if reg.path != path {
		t.Errorf("path = %q, want %q", reg.path, path)
	}
	if reg.runtimes == nil {
		t.Error("runtimes map not initialized")
	}
}

func TestRegistry_LoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")
	reg := NewRegistry(path)

	err := reg.Load()
	if err != nil {
		t.Errorf("Load() on nonexistent file should succeed, got %v", err)
	}
}

func TestRegistry_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "registry.json")
	reg := NewRegistry(path)

	// Add a runtime
	info := &RuntimeInfo{
		Version:      "1.2.3",
		Hash:         "abc123",
		Path:         "/path/to/runtime.sif",
		Size:         1024,
		DownloadedAt: time.Now(),
		Formations:   []string{"formation1"},
	}
	reg.Add(info)

	// Save
	err := reg.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load in new registry
	reg2 := NewRegistry(path)
	err = reg2.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify
	loaded, ok := reg2.Get("1.2.3")
	if !ok || loaded == nil {
		t.Fatal("Get() returned nil after Load()")
	}
	if loaded.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", loaded.Version, "1.2.3")
	}
	if loaded.Hash != "abc123" {
		t.Errorf("Hash = %q, want %q", loaded.Hash, "abc123")
	}
}

func TestRegistry_Add(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	info := &RuntimeInfo{
		Version: "1.0.0",
		Hash:    "hash1",
	}
	reg.Add(info)

	if !reg.Exists("1.0.0") {
		t.Error("Exists() = false after Add()")
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Get non-existent
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent version")
	}

	// Add and get
	info := &RuntimeInfo{Version: "1.0.0"}
	reg.Add(info)

	got, ok := reg.Get("1.0.0")
	if !ok || got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.0.0")
	}
}

func TestRegistry_Exists(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	if reg.Exists("1.0.0") {
		t.Error("Exists() = true for empty registry")
	}

	reg.Add(&RuntimeInfo{Version: "1.0.0"})

	if !reg.Exists("1.0.0") {
		t.Error("Exists() = false after Add()")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Empty list
	list := reg.List()
	if len(list) != 0 {
		t.Errorf("List() = %d items, want 0", len(list))
	}

	// Add some runtimes
	reg.Add(&RuntimeInfo{Version: "1.0.0"})
	reg.Add(&RuntimeInfo{Version: "1.1.0"})

	list = reg.List()
	if len(list) != 2 {
		t.Errorf("List() = %d items, want 2", len(list))
	}
}

func TestRegistry_AddFormation(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Add formation to non-existent runtime
	err := reg.AddFormation("1.0.0", "formation1")
	if err == nil {
		t.Error("AddFormation() should error for non-existent runtime")
	}

	// Add runtime and formation
	reg.Add(&RuntimeInfo{Version: "1.0.0", Formations: []string{}})
	err = reg.AddFormation("1.0.0", "formation1")
	if err != nil {
		t.Errorf("AddFormation() error = %v", err)
	}

	info, _ := reg.Get("1.0.0")
	if len(info.Formations) != 1 || info.Formations[0] != "formation1" {
		t.Errorf("Formations = %v, want [formation1]", info.Formations)
	}

	// Add duplicate formation (should be idempotent)
	err = reg.AddFormation("1.0.0", "formation1")
	if err != nil {
		t.Errorf("AddFormation() duplicate error = %v", err)
	}
	info2, _ := reg.Get("1.0.0")
	if len(info2.Formations) != 1 {
		t.Error("AddFormation() should not duplicate formations")
	}
}

func TestRegistry_RemoveFormation(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Remove from non-existent runtime
	err := reg.RemoveFormation("1.0.0", "formation1")
	if err == nil {
		t.Error("RemoveFormation() should error for non-existent runtime")
	}

	// Add runtime with formations
	reg.Add(&RuntimeInfo{
		Version:    "1.0.0",
		Formations: []string{"formation1", "formation2"},
	})

	// Remove one
	err = reg.RemoveFormation("1.0.0", "formation1")
	if err != nil {
		t.Errorf("RemoveFormation() error = %v", err)
	}

	info, _ := reg.Get("1.0.0")
	if len(info.Formations) != 1 || info.Formations[0] != "formation2" {
		t.Errorf("Formations = %v, want [formation2]", info.Formations)
	}
}

func TestRegistry_GetUnused(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Add runtime with no formations
	reg.Add(&RuntimeInfo{Version: "1.0.0", Formations: []string{}})

	// Add runtime with formations
	reg.Add(&RuntimeInfo{Version: "1.1.0", Formations: []string{"formation1"}})

	unused := reg.GetUnused()
	if len(unused) != 1 || unused[0] != "1.0.0" {
		t.Errorf("GetUnused() = %v, want [1.0.0]", unused)
	}
}

func TestRegistry_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "registry.json")
	reg := NewRegistry(path)

	reg.Add(&RuntimeInfo{
		Version: "1.0.0",
		Path:    "/path/to/runtime.sif",
	})

	err := reg.Delete("1.0.0")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	if reg.Exists("1.0.0") {
		t.Error("Runtime should not exist after Delete()")
	}
}

func TestRegistry_DeleteNonexistent(t *testing.T) {
	reg := NewRegistry("/tmp/test.json")

	// Delete is idempotent - doesn't error for non-existent runtime
	err := reg.Delete("nonexistent")
	if err != nil {
		t.Errorf("Delete() error = %v, want nil (idempotent)", err)
	}
}

func TestRegistry_LoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "registry.json")
	os.WriteFile(path, []byte("invalid json"), 0644)

	reg := NewRegistry(path)
	err := reg.Load()
	if err == nil {
		t.Error("Load() should error for invalid JSON")
	}
}
