package registry

import (
	"fmt"
	"sync"
	"time"

	"github.com/muxi-ai/server/pkg/process"
)

// Registry manages the mapping between formations and their runtime information
// Thread-safe for concurrent access
type Registry struct {
	formations map[string]*Formation // formation ID -> Formation
	portPool   *PortPool
	mu         sync.RWMutex
	onChange   func() // Callback when registry changes (for persistence)
}

// NewRegistry creates a new formation registry
func NewRegistry(portStart, portEnd int) (*Registry, error) {
	portPool, err := NewPortPool(portStart, portEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to create port pool: %w", err)
	}

	return &Registry{
		formations: make(map[string]*Formation),
		portPool:   portPool,
	}, nil
}

// OnChange sets a callback to be called when the registry changes
// Useful for triggering persistence
func (r *Registry) OnChange(fn func()) {
	r.onChange = fn
}

// Register adds a new formation to the registry
func (r *Registry) Register(formation *Formation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if formation already exists
	if _, exists := r.formations[formation.ID]; exists {
		return fmt.Errorf("formation %s already registered", formation.ID)
	}

	// Allocate port if not set
	if formation.Port == 0 {
		port, err := r.portPool.Allocate(formation.ID)
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}
		formation.Port = port
	} else {
		// Port was specified, mark it as allocated
		// (This handles restoration from persistence)
		r.portPool.allocated[formation.Port] = formation.ID
	}

	r.formations[formation.ID] = formation

	r.triggerChange()

	return nil
}

// Unregister removes a formation from the registry
func (r *Registry) Unregister(formationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	formation, exists := r.formations[formationID]
	if !exists {
		return fmt.Errorf("formation %s not found", formationID)
	}

	// Release the port
	r.portPool.Release(formation.Port)

	// Remove from registry
	delete(r.formations, formationID)

	r.triggerChange()

	return nil
}

// Get retrieves a formation by ID
func (r *Registry) Get(formationID string) (*Formation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formation, exists := r.formations[formationID]
	if !exists {
		return nil, fmt.Errorf("formation %s not found", formationID)
	}

	return formation, nil
}

// Update updates a formation's information
func (r *Registry) Update(formationID string, updateFn func(*Formation)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	formation, exists := r.formations[formationID]
	if !exists {
		return fmt.Errorf("formation %s not found", formationID)
	}

	updateFn(formation)

	r.triggerChange()

	return nil
}

// UpdateFromProcess updates a formation with information from a process
func (r *Registry) UpdateFromProcess(formationID string, proc *process.Process) error {
	return r.Update(formationID, func(f *Formation) {
		f.UpdateFromProcess(proc)
	})
}

// List returns all formations
func (r *Registry) List() []*Formation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formations := make([]*Formation, 0, len(r.formations))
	for _, formation := range r.formations {
		formations = append(formations, formation)
	}

	return formations
}

// Count returns the number of registered formations
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.formations)
}

// GetByPort returns the formation using the given port
func (r *Registry) GetByPort(port int) (*Formation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	formationID := r.portPool.Get(port)
	if formationID == "" {
		return nil, fmt.Errorf("no formation using port %d", port)
	}

	formation, exists := r.formations[formationID]
	if !exists {
		return nil, fmt.Errorf("formation %s not found (inconsistent registry)", formationID)
	}

	return formation, nil
}

// AllocatePort allocates a port for a formation
func (r *Registry) AllocatePort(formationID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.portPool.Allocate(formationID)
}

// ReleasePort releases a port
func (r *Registry) ReleasePort(port int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.portPool.Release(port)
}

// PortPoolStatus returns information about the port pool
func (r *Registry) PortPoolStatus() (available, allocated, total int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allocated = r.portPool.AllocatedCount()
	available = r.portPool.Available()
	total = available + allocated

	return
}

// UpdateHealthCheck updates the health check status for a formation
func (r *Registry) UpdateHealthCheck(formationID string, healthy bool) error {
	return r.Update(formationID, func(f *Formation) {
		f.Healthy = healthy
		f.LastHealthCheck = time.Now()
	})
}

// triggerChange calls the onChange callback if set
func (r *Registry) triggerChange() {
	if r.onChange != nil {
		r.onChange()
	}
}
