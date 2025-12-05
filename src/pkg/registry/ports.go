package registry

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// PortPool manages a pool of available ports for formations
type PortPool struct {
	start     int            // Starting port (e.g., 8000)
	end       int            // Ending port (e.g., 9000)
	allocated map[int]string // port -> formation ID
	mu        sync.RWMutex
}

// NewPortPool creates a new port pool
func NewPortPool(start, end int) (*PortPool, error) {
	if start >= end {
		return nil, fmt.Errorf("start port must be less than end port")
	}
	if start < 1024 {
		return nil, fmt.Errorf("start port must be >= 1024 (avoid privileged ports)")
	}
	if end > 65535 {
		return nil, fmt.Errorf("end port must be <= 65535")
	}

	return &PortPool{
		start:     start,
		end:       end,
		allocated: make(map[int]string),
	}, nil
}

// Allocate allocates a port for the given formation ID
// Returns an error if no ports are available
func (pp *PortPool) Allocate(formationID string) (int, error) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Check if formation already has a port
	for port, id := range pp.allocated {
		if id == formationID {
			return port, nil
		}
	}

	// Find next available port (check both internal allocation and OS-level availability)
	for port := pp.start; port < pp.end; port++ {
		if _, used := pp.allocated[port]; !used {
			// Also check if port is actually free at OS level
			if isPortAvailable(port) {
				pp.allocated[port] = formationID
				return port, nil
			}
			// Port in use at OS level but not in our registry - skip it
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", pp.start, pp.end)
}

// isPortAvailable checks if a port is available at the OS level
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	// Small delay to ensure port is fully released
	time.Sleep(10 * time.Millisecond)
	return true
}

// Release releases a port back to the pool
func (pp *PortPool) Release(port int) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	delete(pp.allocated, port)
}

// ReleaseByFormation releases the port allocated to a formation
func (pp *PortPool) ReleaseByFormation(formationID string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	for port, id := range pp.allocated {
		if id == formationID {
			delete(pp.allocated, port)
			return
		}
	}
}

// Get returns the formation ID that owns the given port
// Returns empty string if port is not allocated
func (pp *PortPool) Get(port int) string {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	return pp.allocated[port]
}

// GetPortForFormation returns the port allocated to a formation
// Returns 0 if formation has no port allocated
func (pp *PortPool) GetPortForFormation(formationID string) int {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	for port, id := range pp.allocated {
		if id == formationID {
			return port
		}
	}

	return 0
}

// Available returns the number of available ports
func (pp *PortPool) Available() int {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	total := pp.end - pp.start
	return total - len(pp.allocated)
}

// AllocatedCount returns the number of allocated ports
func (pp *PortPool) AllocatedCount() int {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	return len(pp.allocated)
}

// List returns all allocated ports and their formations
func (pp *PortPool) List() map[int]string {
	pp.mu.RLock()
	defer pp.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[int]string, len(pp.allocated))
	for port, id := range pp.allocated {
		result[port] = id
	}

	return result
}
