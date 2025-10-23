package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RuntimeInfo contains metadata about an installed runtime
type RuntimeInfo struct {
	Version      string    `json:"version"`
	Hash         string    `json:"hash"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Formations   []string  `json:"formations"` // Formation IDs using this runtime
}

// Registry tracks installed runtime SIF files
type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]*RuntimeInfo
	path     string // Path to registry.json
}

// NewRegistry creates a new runtime registry
func NewRegistry(registryPath string) *Registry {
	return &Registry{
		runtimes: make(map[string]*RuntimeInfo),
		path:     registryPath,
	}
}

// Load loads the registry from disk
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(r.path); os.IsNotExist(err) {
		// File doesn't exist yet, that's OK
		return nil
	}

	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("failed to read registry: %w", err)
	}

	var runtimes map[string]*RuntimeInfo
	if err := json.Unmarshal(data, &runtimes); err != nil {
		return fmt.Errorf("failed to parse registry: %w", err)
	}

	r.runtimes = runtimes
	return nil
}

// Save saves the registry to disk
func (r *Registry) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	data, err := json.MarshalIndent(r.runtimes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry: %w", err)
	}

	return nil
}

// Add adds a runtime to the registry
func (r *Registry) Add(info *RuntimeInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.runtimes[info.Version] = info
	return nil
}

// Get retrieves runtime info by version
func (r *Registry) Get(version string) (*RuntimeInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.runtimes[version]
	return info, ok
}

// Exists checks if a runtime version is installed
func (r *Registry) Exists(version string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.runtimes[version]
	return ok
}

// List returns all installed runtime versions
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions := make([]string, 0, len(r.runtimes))
	for v := range r.runtimes {
		versions = append(versions, v)
	}
	return versions
}

// AddFormation adds a formation to a runtime's usage list
func (r *Registry) AddFormation(version, formationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.runtimes[version]
	if !ok {
		return fmt.Errorf("runtime %s not found", version)
	}

	// Check if formation already in list
	for _, id := range info.Formations {
		if id == formationID {
			return nil // Already present
		}
	}

	info.Formations = append(info.Formations, formationID)
	return nil
}

// RemoveFormation removes a formation from a runtime's usage list
func (r *Registry) RemoveFormation(version, formationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, ok := r.runtimes[version]
	if !ok {
		return fmt.Errorf("runtime %s not found", version)
	}

	// Remove formation from list
	filtered := make([]string, 0, len(info.Formations))
	for _, id := range info.Formations {
		if id != formationID {
			filtered = append(filtered, id)
		}
	}

	info.Formations = filtered
	return nil
}

// GetUnused returns runtime versions not used by any formation
func (r *Registry) GetUnused() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var unused []string
	for version, info := range r.runtimes {
		if len(info.Formations) == 0 {
			unused = append(unused, version)
		}
	}
	return unused
}

// Delete removes a runtime from the registry
func (r *Registry) Delete(version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.runtimes, version)
	return nil
}
