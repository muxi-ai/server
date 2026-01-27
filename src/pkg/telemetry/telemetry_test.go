package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollector_IncrementDeploy(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.IncrementDeploy(true)
	c.IncrementDeploy(true)
	c.IncrementDeploy(false)

	snapshot := c.Snapshot()

	if snapshot.Deployments.Successful != 2 {
		t.Errorf("expected 2 successful deployments, got %d", snapshot.Deployments.Successful)
	}
	if snapshot.Deployments.Failed != 1 {
		t.Errorf("expected 1 failed deployment, got %d", snapshot.Deployments.Failed)
	}
	if snapshot.Formations.Deployed != 2 {
		t.Errorf("expected 2 formations deployed, got %d", snapshot.Formations.Deployed)
	}
}

func TestCollector_IncrementUpdate(t *testing.T) {
	c := NewCollector("1.0.0", "docker")

	c.IncrementUpdate(true)
	c.IncrementUpdate(false)

	snapshot := c.Snapshot()

	if snapshot.Deployments.Updates != 1 {
		t.Errorf("expected 1 update, got %d", snapshot.Deployments.Updates)
	}
	if snapshot.Deployments.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", snapshot.Deployments.Failed)
	}
}

func TestCollector_IncrementRollback(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.IncrementRollback()
	c.IncrementRollback()

	snapshot := c.Snapshot()

	if snapshot.Deployments.Rollbacks != 2 {
		t.Errorf("expected 2 rollbacks, got %d", snapshot.Deployments.Rollbacks)
	}
}

func TestCollector_HealthMetrics(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.IncrementCrash()
	c.IncrementCrash()
	c.IncrementAutoRestart()
	c.IncrementHealthCheckFailure()

	snapshot := c.Snapshot()

	if snapshot.Health.Crashes != 2 {
		t.Errorf("expected 2 crashes, got %d", snapshot.Health.Crashes)
	}
	if snapshot.Health.AutoRestarts != 1 {
		t.Errorf("expected 1 auto-restart, got %d", snapshot.Health.AutoRestarts)
	}
	if snapshot.Health.HealthCheckFailures != 1 {
		t.Errorf("expected 1 health check failure, got %d", snapshot.Health.HealthCheckFailures)
	}
}

func TestCollector_RecordRequest(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.RecordRequest(200, 50*time.Millisecond)
	c.RecordRequest(200, 100*time.Millisecond)
	c.RecordRequest(404, 30*time.Millisecond)
	c.RecordRequest(500, 20*time.Millisecond)

	snapshot := c.Snapshot()

	if snapshot.Requests.Total != 4 {
		t.Errorf("expected 4 total requests, got %d", snapshot.Requests.Total)
	}
	if snapshot.Requests.Errors4xx != 1 {
		t.Errorf("expected 1 4xx error, got %d", snapshot.Requests.Errors4xx)
	}
	if snapshot.Requests.Errors5xx != 1 {
		t.Errorf("expected 1 5xx error, got %d", snapshot.Requests.Errors5xx)
	}
	if snapshot.Requests.AvgLatencyMs != 50 {
		t.Errorf("expected 50ms avg latency, got %d", snapshot.Requests.AvgLatencyMs)
	}
}

func TestCollector_RecordAPICall(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.RecordAPICall("deploy")
	c.RecordAPICall("deploy")
	c.RecordAPICall("list")
	c.RecordAPICall("get")

	snapshot := c.Snapshot()

	if snapshot.Usage["deploy"] != 2 {
		t.Errorf("expected 2 deploy calls, got %d", snapshot.Usage["deploy"])
	}
	if snapshot.Usage["list"] != 1 {
		t.Errorf("expected 1 list call, got %d", snapshot.Usage["list"])
	}
}

func TestCollector_Reset(t *testing.T) {
	c := NewCollector("1.0.0", "singularity")

	c.IncrementDeploy(true)
	c.IncrementCrash()
	c.RecordRequest(200, 50*time.Millisecond)

	// First snapshot should have data
	snapshot1 := c.Snapshot()
	if snapshot1.Deployments.Successful != 1 {
		t.Errorf("expected 1 successful deployment, got %d", snapshot1.Deployments.Successful)
	}

	// Reset
	c.Reset()

	// Second snapshot should be empty (except server info)
	snapshot2 := c.Snapshot()
	if snapshot2.Deployments.Successful != 0 {
		t.Errorf("expected 0 successful deployments after reset, got %d", snapshot2.Deployments.Successful)
	}
	if snapshot2.Health.Crashes != 0 {
		t.Errorf("expected 0 crashes after reset, got %d", snapshot2.Health.Crashes)
	}
}

func TestCollector_ServerInfo(t *testing.T) {
	c := NewCollector("1.2.3", "docker")

	snapshot := c.Snapshot()

	if snapshot.Server.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", snapshot.Server.Version)
	}
	if snapshot.Server.Runtime != "docker" {
		t.Errorf("expected runtime docker, got %s", snapshot.Server.Runtime)
	}
}

func TestGetSystemInfo(t *testing.T) {
	info := GetSystemInfo()

	if info.OS == "" {
		t.Error("expected non-empty OS")
	}
	if info.Arch == "" {
		t.Error("expected non-empty Arch")
	}
	if info.CPUCores < 1 {
		t.Errorf("expected at least 1 CPU core, got %d", info.CPUCores)
	}
	// RAM might be 0 on some systems
}

func TestIsEnabled(t *testing.T) {
	// Save original values
	origTelemetry := os.Getenv("MUXI_TELEMETRY")
	origHome := os.Getenv("HOME")

	// Create temp dir for config
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	defer func() {
		if origTelemetry != "" {
			os.Setenv("MUXI_TELEMETRY", origTelemetry)
		} else {
			os.Unsetenv("MUXI_TELEMETRY")
		}
		os.Setenv("HOME", origHome)
	}()

	// Clear env var
	os.Unsetenv("MUXI_TELEMETRY")

	// Default should be enabled
	if !IsEnabled() {
		t.Error("telemetry should be enabled by default")
	}

	// Env var should disable
	os.Setenv("MUXI_TELEMETRY", "0")
	if IsEnabled() {
		t.Error("telemetry should be disabled when MUXI_TELEMETRY=0")
	}

	// Clear env var
	os.Unsetenv("MUXI_TELEMETRY")

	// Config file should disable
	configDir := filepath.Join(tmpDir, ".muxi")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte("telemetry: false\n"), 0644)

	if IsEnabled() {
		t.Error("telemetry should be disabled when config says false")
	}
}

func TestGlobalTelemetry_StartStop(t *testing.T) {
	// Reset global state
	globalMu.Lock()
	initialized = false
	globalCollector = nil
	globalSender = nil
	globalMu.Unlock()

	Init("1.0.0-startstop", "docker")

	ctx, cancel := context.WithCancel(context.Background())
	Start(ctx)

	// Double Start should be safe
	Start(ctx)

	cancel()
	Stop()

	// Double Stop should be safe
	Stop()

	// Reset for other tests
	globalMu.Lock()
	initialized = false
	globalCollector = nil
	globalSender = nil
	globalMu.Unlock()
}

func TestGlobalTelemetry_StartStopBeforeInit(t *testing.T) {
	globalMu.Lock()
	initialized = false
	globalCollector = nil
	globalSender = nil
	globalMu.Unlock()

	// Start/Stop before Init should be safe
	ctx := context.Background()
	Start(ctx)
	Stop()
}

func TestSaveGlobalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	config := map[string]interface{}{
		"telemetry":  true,
		"machine_id": "test-id-123",
	}

	err := saveGlobalConfig(config)
	if err != nil {
		t.Fatalf("saveGlobalConfig() error = %v", err)
	}

	// Verify it was saved
	loaded, err := loadGlobalConfig()
	if err != nil {
		t.Fatalf("loadGlobalConfig() error = %v", err)
	}
	if loaded["machine_id"] != "test-id-123" {
		t.Errorf("machine_id = %v, want test-id-123", loaded["machine_id"])
	}
}

func TestCacheMachineID(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cacheMachineID("cached-machine-id")

	got := getCachedMachineID()
	if got != "cached-machine-id" {
		t.Errorf("getCachedMachineID() = %q, want cached-machine-id", got)
	}
}

func TestCacheCountry(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cacheCountry("US")

	got := getCachedCountry()
	if got != "US" {
		t.Errorf("getCachedCountry() = %q, want US", got)
	}
}

func TestGlobalTelemetry(t *testing.T) {
	// Initialize
	Init("1.0.0-test", "singularity")

	// Get collector should not be nil
	c := GetCollector()
	if c == nil {
		t.Error("expected non-nil collector after Init")
	}

	// Convenience wrappers should not panic
	IncrementServerStart()
	IncrementDeploy(true)
	IncrementUpdate(true)
	IncrementRollback()
	IncrementDelete()
	IncrementAutoRestart()
	IncrementCrash()
	IncrementHealthCheckFailure()
	RecordAPICall("test")
	RecordRequest(200, 10*time.Millisecond)
	SetActiveFormations(5)
	SetPortStats(3, 1000)

	// Verify some values
	snapshot := c.Snapshot()
	if snapshot.Server.Starts != 1 {
		t.Errorf("expected 1 server start, got %d", snapshot.Server.Starts)
	}
}
