package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DeployStage represents the current stage of deployment/update
type DeployStage string

const (
	// Common stages (deploy and update)
	StageExtracting       DeployStage = "extracting"
	StageValidating       DeployStage = "validating"
	StageResolvingRuntime DeployStage = "resolving_runtime"
	StageDownloadingSIF   DeployStage = "downloading_sif"
	StagePullingRunner    DeployStage = "pulling_runner"
	StageSpawning         DeployStage = "spawning"
	StageHealthCheck      DeployStage = "health_check"

	// Update-specific stages (blue-green deployment)
	StageSpawningStaging DeployStage = "spawning_staging"
	StageSwapping        DeployStage = "swapping"
	StageStoppingOld     DeployStage = "stopping_old"
)

// ProgressEvent represents a progress update during deployment
type ProgressEvent struct {
	Stage       DeployStage `json:"stage"`
	Message     string      `json:"message"`
	Progress    *int        `json:"progress,omitempty"`     // For downloading_sif (0-100)
	URL         string      `json:"url,omitempty"`          // For downloading_sif
	Version     string      `json:"version,omitempty"`      // For resolving_runtime
	Attempt     *int        `json:"attempt,omitempty"`      // For health_check
	MaxAttempts *int        `json:"max_attempts,omitempty"` // For health_check
	StagingPort *int        `json:"staging_port,omitempty"` // For spawning_staging (update)
}

// CompleteEvent represents successful deployment/update completion
type CompleteEvent struct {
	FormationID     string `json:"formation_id"`
	Port            int    `json:"port"`
	Status          string `json:"status"`
	URL             string `json:"url"`
	HealthURL       string `json:"health_url,omitempty"`
	PID             int    `json:"pid,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"` // For updates
	NewVersion      string `json:"new_version,omitempty"`      // For updates
}

// ErrorEvent represents a deployment failure
type ErrorEvent struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Stage   DeployStage            `json:"stage"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SSEWriter wraps an http.ResponseWriter for Server-Sent Events
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer if streaming is supported
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	return &SSEWriter{w: w, flusher: flusher}, true
}

// Init sets up SSE headers and sends initial response
func (s *SSEWriter) Init() {
	s.w.Header().Set("Content-Type", "text/event-stream")
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("Connection", "keep-alive")
	s.w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	s.w.WriteHeader(http.StatusOK)
	s.flusher.Flush()
}

// SendProgress sends a progress event
func (s *SSEWriter) SendProgress(event ProgressEvent) error {
	return s.sendEvent("progress", event)
}

// SendComplete sends a completion event
func (s *SSEWriter) SendComplete(event CompleteEvent) error {
	return s.sendEvent("complete", event)
}

// SendError sends an error event
func (s *SSEWriter) SendError(event ErrorEvent) error {
	return s.sendEvent("error", event)
}

// sendEvent sends a generic SSE event
func (s *SSEWriter) sendEvent(eventType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// SSE format: event: <type>\ndata: <json>\n\n
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	s.flusher.Flush()
	return nil
}

// ProgressEmitter provides a way to emit progress events during deployment
type ProgressEmitter struct {
	sse     *SSEWriter
	enabled bool
}

// NewProgressEmitter creates a new progress emitter
func NewProgressEmitter(sse *SSEWriter) *ProgressEmitter {
	return &ProgressEmitter{
		sse:     sse,
		enabled: sse != nil,
	}
}

// Emit sends a progress event if streaming is enabled
func (p *ProgressEmitter) Emit(event ProgressEvent) {
	if p.enabled && p.sse != nil {
		p.sse.SendProgress(event)
	}
}

// Complete sends a completion event if streaming is enabled
func (p *ProgressEmitter) Complete(event CompleteEvent) {
	if p.enabled && p.sse != nil {
		p.sse.SendComplete(event)
	}
}

// Error sends an error event if streaming is enabled
func (p *ProgressEmitter) Error(event ErrorEvent) {
	if p.enabled && p.sse != nil {
		p.sse.SendError(event)
	}
}

// IsEnabled returns true if progress streaming is enabled
func (p *ProgressEmitter) IsEnabled() bool {
	return p.enabled
}
