package telemetry

import (
	"sync"
	"time"
)

// Collector collects telemetry metrics throughout the server lifecycle
type Collector struct {
	mu sync.RWMutex

	// Server info (set once at startup)
	version   string
	runtime   string // "docker" or "singularity"
	startTime time.Time

	// Counters (accumulate until sent)
	serverStarts          int
	deploymentsSuccessful int
	deploymentsFailed     int
	deploymentsUpdates    int
	deploymentsRollbacks  int
	formationsDeployed    int
	formationsDeleted     int
	autoRestarts          int
	crashes               int
	healthCheckFailures   int
	usage                 map[string]int
	requestsTotal         int64
	requestsErrors4xx     int64
	requestsErrors5xx     int64
	latencySum            int64
	latencyCount          int64

	// Gauges (current state, not reset)
	activeFormations int
	portsAllocated   int
	portPoolSize     int
}

// NewCollector creates a new telemetry collector
func NewCollector(version, runtime string) *Collector {
	return &Collector{
		version:   version,
		runtime:   runtime,
		startTime: time.Now(),
		usage:     make(map[string]int),
	}
}

// IncrementServerStart increments the server start counter
func (c *Collector) IncrementServerStart() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverStarts++
}

// IncrementDeploy increments the deployment counter
func (c *Collector) IncrementDeploy(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if success {
		c.deploymentsSuccessful++
		c.formationsDeployed++
	} else {
		c.deploymentsFailed++
	}
}

// IncrementUpdate increments the update (blue-green deploy) counter
func (c *Collector) IncrementUpdate(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if success {
		c.deploymentsUpdates++
	} else {
		c.deploymentsFailed++
	}
}

// IncrementRollback increments the rollback counter
func (c *Collector) IncrementRollback() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deploymentsRollbacks++
}

// IncrementDelete increments the formation deletion counter
func (c *Collector) IncrementDelete() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.formationsDeleted++
}

// IncrementAutoRestart increments the auto-restart counter
func (c *Collector) IncrementAutoRestart() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoRestarts++
}

// IncrementCrash increments the crash counter
func (c *Collector) IncrementCrash() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.crashes++
}

// IncrementHealthCheckFailure increments the health check failure counter
func (c *Collector) IncrementHealthCheckFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthCheckFailures++
}

// RecordAPICall records an API endpoint call
func (c *Collector) RecordAPICall(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.usage[endpoint]++
}

// RecordRequest records a proxied request with status and latency
func (c *Collector) RecordRequest(statusCode int, latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestsTotal++
	c.latencySum += latency.Milliseconds()
	c.latencyCount++

	if statusCode >= 400 && statusCode < 500 {
		c.requestsErrors4xx++
	} else if statusCode >= 500 {
		c.requestsErrors5xx++
	}
}

// SetActiveFormations sets the current number of active formations
func (c *Collector) SetActiveFormations(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeFormations = count
}

// SetPortStats sets the port pool statistics
func (c *Collector) SetPortStats(allocated, poolSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.portsAllocated = allocated
	c.portPoolSize = poolSize
}

// Snapshot returns the current metrics and resets counters
func (c *Collector) Snapshot() Payload {
	c.mu.Lock()
	defer c.mu.Unlock()

	avgLatency := 0
	if c.latencyCount > 0 {
		avgLatency = int(c.latencySum / c.latencyCount)
	}

	payload := Payload{
		Server: ServerInfo{
			Version:     c.version,
			Runtime:     c.runtime,
			UptimeHours: int(time.Since(c.startTime).Hours()),
			Starts:      c.serverStarts,
		},
		System: GetSystemInfo(),
		Formations: FormationStats{
			Active:   c.activeFormations,
			Deployed: c.formationsDeployed,
			Deleted:  c.formationsDeleted,
		},
		Deployments: DeploymentStats{
			Successful: c.deploymentsSuccessful,
			Failed:     c.deploymentsFailed,
			Updates:    c.deploymentsUpdates,
			Rollbacks:  c.deploymentsRollbacks,
		},
		Health: HealthStats{
			AutoRestarts:        c.autoRestarts,
			Crashes:             c.crashes,
			HealthCheckFailures: c.healthCheckFailures,
		},
		Requests: RequestStats{
			Total:        c.requestsTotal,
			Errors4xx:    c.requestsErrors4xx,
			Errors5xx:    c.requestsErrors5xx,
			AvgLatencyMs: avgLatency,
		},
		Usage: c.usage,
		Resources: ResourceStats{
			PortPoolSize:   c.portPoolSize,
			PortsAllocated: c.portsAllocated,
		},
	}

	return payload
}

// Reset resets all counters (called after successful send)
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.serverStarts = 0
	c.deploymentsSuccessful = 0
	c.deploymentsFailed = 0
	c.deploymentsUpdates = 0
	c.deploymentsRollbacks = 0
	c.formationsDeployed = 0
	c.formationsDeleted = 0
	c.autoRestarts = 0
	c.crashes = 0
	c.healthCheckFailures = 0
	c.usage = make(map[string]int)
	c.requestsTotal = 0
	c.requestsErrors4xx = 0
	c.requestsErrors5xx = 0
	c.latencySum = 0
	c.latencyCount = 0
}
