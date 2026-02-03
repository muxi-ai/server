package registry

import (
	"sync"
	"testing"
	"time"

	"github.com/muxi-ai/server/pkg/process"
)

func TestPortPool_EdgeCases(t *testing.T) {
	t.Run("invalid start >= end", func(t *testing.T) {
		_, err := NewPortPool(9000, 8000)
		if err == nil {
			t.Error("NewPortPool with start >= end should fail")
		}
	})

	t.Run("invalid equal start and end", func(t *testing.T) {
		_, err := NewPortPool(8000, 8000)
		if err == nil {
			t.Error("NewPortPool with equal start and end should fail")
		}
	})

	t.Run("privileged ports", func(t *testing.T) {
		_, err := NewPortPool(80, 100)
		if err == nil {
			t.Error("NewPortPool with privileged ports should fail")
		}
	})

	t.Run("port > 65535", func(t *testing.T) {
		_, err := NewPortPool(8000, 70000)
		if err == nil {
			t.Error("NewPortPool with port > 65535 should fail")
		}
	})

	t.Run("single port pool", func(t *testing.T) {
		// Use high port to avoid conflicts with api tests
		pool, err := NewPortPool(19500, 19501)
		if err != nil {
			t.Fatalf("Failed to create single port pool: %v", err)
		}

		port, err := pool.Allocate("test")
		if err != nil {
			t.Fatalf("Failed to allocate from single port pool: %v", err)
		}

		if port != 19500 {
			t.Errorf("Port = %d, want 19500", port)
		}

		// Second allocation should fail
		_, err = pool.Allocate("test2")
		if err == nil {
			t.Error("Second allocation from single port pool should fail")
		}
	})
}

func TestPortPool_Concurrent(t *testing.T) {
	pool, err := NewPortPool(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create port pool: %v", err)
	}

	var wg sync.WaitGroup
	ports := make(chan int, 100)
	errors := make(chan error, 100)

	// Allocate 50 ports concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			port, err := pool.Allocate(formatID(id))
			if err != nil {
				errors <- err
				return
			}
			ports <- port
		}(i)
	}

	wg.Wait()
	close(ports)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Allocation error: %v", err)
	}

	// Verify unique ports
	seen := make(map[int]bool)
	count := 0
	for port := range ports {
		if seen[port] {
			t.Errorf("Port %d allocated twice!", port)
		}
		seen[port] = true
		count++
	}

	if count != 50 {
		t.Errorf("Got %d ports, want 50", count)
	}
}

func TestPortPool_GetPortForFormation(t *testing.T) {
	pool, err := NewPortPool(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create port pool: %v", err)
	}

	// Allocate a port
	port, err := pool.Allocate("test-formation")
	if err != nil {
		t.Fatalf("Failed to allocate port: %v", err)
	}

	// Get the port for formation
	foundPort := pool.GetPortForFormation("test-formation")
	if foundPort != port {
		t.Errorf("GetPortForFormation() = %d, want %d", foundPort, port)
	}

	// Non-existent formation should return 0
	foundPort = pool.GetPortForFormation("nonexistent")
	if foundPort != 0 {
		t.Errorf("GetPortForFormation(nonexistent) = %d, want 0", foundPort)
	}
}

func TestPortPool_List(t *testing.T) {
	pool, err := NewPortPool(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create port pool: %v", err)
	}

	// Allocate some ports
	pool.Allocate("form1")
	pool.Allocate("form2")
	pool.Allocate("form3")

	list := pool.List()
	if len(list) != 3 {
		t.Errorf("List() len = %d, want 3", len(list))
	}

	// Verify we got a copy (modifying it shouldn't affect pool)
	for port := range list {
		list[port] = "modified"
	}

	list2 := pool.List()
	for _, id := range list2 {
		if id == "modified" {
			t.Error("List() should return a copy, not the internal map")
		}
	}
}

func TestRegistry_UpdateFromProcess(t *testing.T) {
	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Register a formation
	formation := &Formation{
		ID:     "test",
		Name:   "Test",
		Status: "starting",
	}
	reg.Register(formation)

	// Create a process with updated info
	proc := &process.Process{
		ID:           "test",
		Name:         "Test Updated",
		PID:          12345,
		Status:       process.StatusRunning,
		RestartCount: 3,
		StartedAt:    time.Now(),
	}

	// Update from process
	err = reg.UpdateFromProcess("test", proc)
	if err != nil {
		t.Fatalf("UpdateFromProcess() error = %v", err)
	}

	// Verify update
	updated, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if updated.ProcessID != 12345 {
		t.Errorf("ProcessID = %d, want 12345", updated.ProcessID)
	}

	if updated.RestartCount != 3 {
		t.Errorf("RestartCount = %d, want 3", updated.RestartCount)
	}
}

func TestRegistry_UpdateHealthCheck(t *testing.T) {
	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Register a formation
	formation := &Formation{
		ID:      "test",
		Healthy: false,
	}
	reg.Register(formation)

	// Update health check
	err = reg.UpdateHealthCheck("test", true)
	if err != nil {
		t.Fatalf("UpdateHealthCheck() error = %v", err)
	}

	// Verify update
	updated, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !updated.Healthy {
		t.Error("Healthy should be true after UpdateHealthCheck")
	}

	if updated.LastHealthCheck.IsZero() {
		t.Error("LastHealthCheck should be set")
	}

	// Update to unhealthy
	err = reg.UpdateHealthCheck("test", false)
	if err != nil {
		t.Fatalf("UpdateHealthCheck() error = %v", err)
	}

	updated, _ = reg.Get("test")
	if updated.Healthy {
		t.Error("Healthy should be false after UpdateHealthCheck")
	}
}

func TestRegistry_GetByPort(t *testing.T) {
	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Register a formation
	formation := &Formation{
		ID:   "test",
		Name: "Test",
	}
	err = reg.Register(formation)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	port := formation.Port

	// Get by port
	found, err := reg.GetByPort(port)
	if err != nil {
		t.Fatalf("GetByPort() error = %v", err)
	}

	if found.ID != "test" {
		t.Errorf("GetByPort() ID = %q, want %q", found.ID, "test")
	}

	// Non-existent port
	_, err = reg.GetByPort(9999)
	if err == nil {
		t.Error("GetByPort(9999) should return error")
	}
}

func TestRegistry_OnChange(t *testing.T) {
	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	changeCount := 0
	reg.OnChange(func() {
		changeCount++
	})

	// Register should trigger change
	formation := &Formation{ID: "test1"}
	reg.Register(formation)

	if changeCount != 1 {
		t.Errorf("Change count after Register = %d, want 1", changeCount)
	}

	// Update should trigger change
	reg.Update("test1", func(f *Formation) {
		f.Status = "running"
	})

	if changeCount != 2 {
		t.Errorf("Change count after Update = %d, want 2", changeCount)
	}

	// Unregister should trigger change
	reg.Unregister("test1")

	if changeCount != 3 {
		t.Errorf("Change count after Unregister = %d, want 3", changeCount)
	}
}

func TestRegistry_AllocatePort(t *testing.T) {
	// Use high port range to avoid conflicts with api tests
	reg, err := NewRegistry(19100, 19110)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Allocate a port directly
	port, err := reg.AllocatePort("test-formation")
	if err != nil {
		t.Fatalf("AllocatePort() error = %v", err)
	}

	if port < 19100 || port >= 19110 {
		t.Errorf("Port %d out of range", port)
	}

	// Same formation should get same port
	port2, err := reg.AllocatePort("test-formation")
	if err != nil {
		t.Fatalf("AllocatePort() second call error = %v", err)
	}

	if port2 != port {
		t.Errorf("Same formation got different port: %d vs %d", port, port2)
	}
}

func TestRegistry_ReleasePort(t *testing.T) {
	// Use high port range to avoid conflicts with api tests
	reg, err := NewRegistry(19200, 19210)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	port, _ := reg.AllocatePort("test")

	// Release the port
	reg.ReleasePort(port)

	// Should be able to allocate it again
	port2, err := reg.AllocatePort("test2")
	if err != nil {
		t.Fatalf("AllocatePort() after release error = %v", err)
	}

	if port2 != port {
		t.Logf("Note: Got different port %d (original was %d)", port2, port)
	}
}

func TestRegistry_PortPoolStatus(t *testing.T) {
	// Use high port range to avoid conflicts with api tests
	reg, err := NewRegistry(19300, 19310)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	available, allocated, total := reg.PortPoolStatus()

	if total != 10 {
		t.Errorf("Total = %d, want 10", total)
	}

	if allocated != 0 {
		t.Errorf("Initial allocated = %d, want 0", allocated)
	}

	if available != 10 {
		t.Errorf("Initial available = %d, want 10", available)
	}

	// Allocate some ports
	reg.AllocatePort("test1")
	reg.AllocatePort("test2")

	available, allocated, total = reg.PortPoolStatus()

	if allocated != 2 {
		t.Errorf("Allocated after 2 allocs = %d, want 2", allocated)
	}

	if available != 8 {
		t.Errorf("Available after 2 allocs = %d, want 8", available)
	}

	if total != 10 {
		t.Errorf("Total = %d, want 10", total)
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	reg, err := NewRegistry(8000, 8100)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			formation := &Formation{
				ID:   formatID(id),
				Name: formatID(id),
			}
			reg.Register(formation)
		}(i)
	}

	wg.Wait()

	// Concurrent reads
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reg.Get(formatID(id))
			reg.List()
		}(i)
	}

	wg.Wait()

	count := reg.Count()
	if count != 20 {
		t.Errorf("Count = %d, want 20", count)
	}
}

// Helper function to format IDs
func formatID(id int) string {
	// Create unique ID for each number
	if id < 10 {
		return "formation-0" + string(rune('0'+id))
	}
	return "formation-" + string(rune('0'+id/10)) + string(rune('0'+id%10))
}

func TestRegistry_SetDeploying(t *testing.T) {
	reg, _ := NewRegistry(8000, 8100)

	reg.Register(&Formation{ID: "test", Port: 8080})

	// Set deploying
	err := reg.SetDeploying("test", true)
	if err != nil {
		t.Errorf("SetDeploying(true) error = %v", err)
	}

	// Try to set deploying again - should fail
	err = reg.SetDeploying("test", true)
	if err == nil {
		t.Error("SetDeploying(true) should fail when already deploying")
	}

	// Clear deploying
	err = reg.SetDeploying("test", false)
	if err != nil {
		t.Errorf("SetDeploying(false) error = %v", err)
	}

	// Non-existent formation
	err = reg.SetDeploying("nonexistent", true)
	if err == nil {
		t.Error("SetDeploying on nonexistent should fail")
	}
}

func TestRegistry_StagingPort(t *testing.T) {
	reg, _ := NewRegistry(8000, 8100)

	reg.Register(&Formation{ID: "test", Port: 8080})

	// Initially no staging port
	if port := reg.GetStagingPort("test"); port != 0 {
		t.Errorf("GetStagingPort = %d, want 0", port)
	}

	// Set staging port
	err := reg.SetStagingPort("test", 8081)
	if err != nil {
		t.Errorf("SetStagingPort error = %v", err)
	}

	if port := reg.GetStagingPort("test"); port != 8081 {
		t.Errorf("GetStagingPort = %d, want 8081", port)
	}

	// Switch to staging port
	oldPort, err := reg.SwitchToStagingPort("test")
	if err != nil {
		t.Errorf("SwitchToStagingPort error = %v", err)
	}
	if oldPort != 8080 {
		t.Errorf("Old port = %d, want 8080", oldPort)
	}

	f, _ := reg.Get("test")
	if f.Port != 8081 {
		t.Errorf("Port after switch = %d, want 8081", f.Port)
	}
	if f.StagingPort != 0 {
		t.Errorf("StagingPort after switch = %d, want 0", f.StagingPort)
	}
}

func TestRegistry_ClearStagingPort(t *testing.T) {
	reg, _ := NewRegistry(8000, 8100)

	reg.Register(&Formation{ID: "test", Port: 8080, StagingPort: 8081})

	err := reg.ClearStagingPort("test")
	if err != nil {
		t.Errorf("ClearStagingPort error = %v", err)
	}

	if port := reg.GetStagingPort("test"); port != 0 {
		t.Errorf("StagingPort after clear = %d, want 0", port)
	}
}

func TestRegistry_SwitchToStagingPort_Errors(t *testing.T) {
	reg, _ := NewRegistry(8000, 8100)

	// Non-existent formation
	_, err := reg.SwitchToStagingPort("nonexistent")
	if err == nil {
		t.Error("SwitchToStagingPort on nonexistent should fail")
	}

	// Formation without staging port
	reg.Register(&Formation{ID: "test", Port: 8080})
	_, err = reg.SwitchToStagingPort("test")
	if err == nil {
		t.Error("SwitchToStagingPort without staging port should fail")
	}
}

func TestRegistry_GetStagingPort_Nonexistent(t *testing.T) {
	reg, _ := NewRegistry(8000, 8100)

	// Non-existent formation returns 0
	if port := reg.GetStagingPort("nonexistent"); port != 0 {
		t.Errorf("GetStagingPort(nonexistent) = %d, want 0", port)
	}
}
