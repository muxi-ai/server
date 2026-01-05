package telemetry

import (
	"context"
	"sync"
	"time"
)

var (
	globalCollector *Collector
	globalSender    *Sender
	globalMu        sync.Mutex
	initialized     bool
)

// Init initializes the global telemetry system
func Init(version, runtime string) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if initialized {
		return
	}

	globalCollector = NewCollector(version, runtime)
	globalSender = NewSender(globalCollector)
	initialized = true
}

// Start starts the global telemetry sender
func Start(ctx context.Context) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if !initialized || globalSender == nil {
		return
	}

	globalSender.Start(ctx)
}

// Stop stops the global telemetry sender
func Stop() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if !initialized || globalSender == nil {
		return
	}

	globalSender.Stop()
}

// GetCollector returns the global collector instance
func GetCollector() *Collector {
	globalMu.Lock()
	defer globalMu.Unlock()
	return globalCollector
}

// IncrementServerStart increments the server start counter (convenience wrapper)
func IncrementServerStart() {
	if c := GetCollector(); c != nil {
		c.IncrementServerStart()
	}
}

// IncrementDeploy increments the deployment counter (convenience wrapper)
func IncrementDeploy(success bool) {
	if c := GetCollector(); c != nil {
		c.IncrementDeploy(success)
	}
}

// IncrementUpdate increments the update counter (convenience wrapper)
func IncrementUpdate(success bool) {
	if c := GetCollector(); c != nil {
		c.IncrementUpdate(success)
	}
}

// IncrementRollback increments the rollback counter (convenience wrapper)
func IncrementRollback() {
	if c := GetCollector(); c != nil {
		c.IncrementRollback()
	}
}

// IncrementDelete increments the delete counter (convenience wrapper)
func IncrementDelete() {
	if c := GetCollector(); c != nil {
		c.IncrementDelete()
	}
}

// IncrementAutoRestart increments the auto-restart counter (convenience wrapper)
func IncrementAutoRestart() {
	if c := GetCollector(); c != nil {
		c.IncrementAutoRestart()
	}
}

// IncrementCrash increments the crash counter (convenience wrapper)
func IncrementCrash() {
	if c := GetCollector(); c != nil {
		c.IncrementCrash()
	}
}

// IncrementHealthCheckFailure increments the health check failure counter
func IncrementHealthCheckFailure() {
	if c := GetCollector(); c != nil {
		c.IncrementHealthCheckFailure()
	}
}

// RecordAPICall records an API endpoint call (convenience wrapper)
func RecordAPICall(endpoint string) {
	if c := GetCollector(); c != nil {
		c.RecordAPICall(endpoint)
	}
}

// SetActiveFormations sets the active formation count (convenience wrapper)
func SetActiveFormations(count int) {
	if c := GetCollector(); c != nil {
		c.SetActiveFormations(count)
	}
}

// SetPortStats sets port allocation stats (convenience wrapper)
func SetPortStats(allocated, poolSize int) {
	if c := GetCollector(); c != nil {
		c.SetPortStats(allocated, poolSize)
	}
}

// RecordRequest records a proxied request (convenience wrapper)
func RecordRequest(statusCode int, latency time.Duration) {
	if c := GetCollector(); c != nil {
		c.RecordRequest(statusCode, latency)
	}
}
