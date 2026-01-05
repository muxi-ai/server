package telemetry

import "time"

// Event represents the telemetry payload sent to the capture server
type Event struct {
	Module        string  `json:"module"`
	MachineID     string  `json:"machine_id"`
	Timestamp     string  `json:"ts"`
	Country       string  `json:"country"`
	SchemaVersion int     `json:"schema_version"`
	Payload       Payload `json:"payload"`
}

// Payload contains the server-specific telemetry data
type Payload struct {
	Server      ServerInfo      `json:"server"`
	System      SystemInfo      `json:"system"`
	Formations  FormationStats  `json:"formations"`
	Deployments DeploymentStats `json:"deployments"`
	Health      HealthStats     `json:"health"`
	Requests    RequestStats    `json:"requests"`
	Usage       map[string]int  `json:"usage"`
	Resources   ResourceStats   `json:"resources"`
}

// ServerInfo contains server version and runtime info
type ServerInfo struct {
	Version     string `json:"version"`
	Runtime     string `json:"runtime"` // "docker" or "singularity"
	UptimeHours int    `json:"uptime_hours"`
	Starts      int    `json:"starts"`
}

// SystemInfo contains OS and hardware info
type SystemInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUCores int    `json:"cpu_cores"`
	RAMGB    int    `json:"ram_gb"`
}

// FormationStats contains formation counts
type FormationStats struct {
	Active   int `json:"active"`
	Deployed int `json:"deployed"`
	Deleted  int `json:"deleted"`
}

// DeploymentStats contains deployment operation counts
type DeploymentStats struct {
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
	Updates    int `json:"updates"`
	Rollbacks  int `json:"rollbacks"`
}

// HealthStats contains process health metrics
type HealthStats struct {
	AutoRestarts        int `json:"auto_restarts"`
	Crashes             int `json:"crashes"`
	HealthCheckFailures int `json:"health_check_failures"`
}

// RequestStats contains proxy request metrics
type RequestStats struct {
	Total        int64 `json:"total"`
	Errors4xx    int64 `json:"errors_4xx"`
	Errors5xx    int64 `json:"errors_5xx"`
	AvgLatencyMs int   `json:"avg_latency_ms"`
}

// ResourceStats contains resource allocation info
type ResourceStats struct {
	PortPoolSize   int `json:"port_pool_size"`
	PortsAllocated int `json:"ports_allocated"`
}

// LocalState represents the persistent state stored in telemetry.json
type LocalState struct {
	LastFlush   time.Time       `json:"last_flush"`
	LastSent    time.Time       `json:"last_sent"`
	ServerStart time.Time       `json:"server_start"`
	Starts      int             `json:"starts"`
	Formations  FormationStats  `json:"formations"`
	Deployments DeploymentStats `json:"deployments"`
	Health      HealthStats     `json:"health"`
	Requests    RequestStats    `json:"requests"`
	Usage       map[string]int  `json:"usage"`
	LatencySum  int64           `json:"latency_sum"`
	LatencyN    int64           `json:"latency_n"`
}
