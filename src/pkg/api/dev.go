package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/muxi-ai/server/pkg/runtime"
	"github.com/rs/zerolog/log"
)

// DevRunRequest represents the request body for POST /rpc/dev/run
type DevRunRequest struct {
	Path        string `json:"path"`         // Local filesystem path (muxi up)
	FormationID string `json:"formation_id"` // Run from draft/ dir (Console)
}

// DevRunResponse represents the response for POST /rpc/dev/run
type DevRunResponse struct {
	Success     bool   `json:"success"`
	FormationID string `json:"formation_id"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// DevStopRequest represents the request body for POST /rpc/dev/stop
type DevStopRequest struct {
	FormationID string `json:"formation_id"`
}

// DevStopResponse represents the response for POST /rpc/dev/stop
type DevStopResponse struct {
	Success     bool   `json:"success"`
	FormationID string `json:"formation_id"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// HandleDevRun handles POST /rpc/dev/run
// Starts a formation in development/draft mode from either:
//   - Local path (muxi up): {"path": "/absolute/path/to/formation"}
//   - Draft directory (Console): {"formation_id": "my-app"} (uses draft/ dir)
func (s *Server) HandleDevRun(w http.ResponseWriter, r *http.Request) {
	var req DevRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondDevError(w, http.StatusBadRequest, "", "Invalid request body")
		return
	}

	// Determine working directory and formation ID
	var workDir string
	var formationID string

	if req.Path != "" {
		// Local path mode (muxi up)
		workDir = req.Path

		// Validate path is absolute
		if !filepath.IsAbs(workDir) {
			respondDevError(w, http.StatusBadRequest, "", "Path must be absolute")
			return
		}

		// Validate directory exists
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			respondDevError(w, http.StatusBadRequest, "", fmt.Sprintf("Directory not found: %s", workDir))
			return
		}
	} else if req.FormationID != "" {
		// Draft directory mode (Console)
		muxiDir, err := getMuxiDir()
		if err != nil {
			respondDevError(w, http.StatusInternalServerError, req.FormationID, err.Error())
			return
		}
		workDir = filepath.Join(muxiDir, "formations", req.FormationID, "draft")

		// Validate draft directory exists
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			respondDevError(w, http.StatusNotFound, req.FormationID, fmt.Sprintf("Draft directory not found for formation %s", req.FormationID))
			return
		}
		formationID = req.FormationID
	} else {
		respondDevError(w, http.StatusBadRequest, "", "Must provide either 'path' or 'formation_id'")
		return
	}

	// Find and parse formation config
	formationConfigPath, err := formation.FindFormationFile(workDir)
	if err != nil {
		respondDevError(w, http.StatusBadRequest, formationID, fmt.Sprintf("formation.afs not found in %s", workDir))
		return
	}

	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		respondDevError(w, http.StatusBadRequest, formationID, fmt.Sprintf("Failed to parse formation config: %v", err))
		return
	}

	// Get formation ID from config if not already set
	if formationID == "" {
		formationID = formationConfig.ID
	}

	if formationID == "" {
		respondDevError(w, http.StatusBadRequest, "", "Formation ID not found in formation.afs")
		return
	}

	log.Info().
		Str("formation_id", formationID).
		Str("work_dir", workDir).
		Msg("Starting draft formation")

	// Check if draft already running for this ID
	if existing, _ := s.registry.GetDraft(formationID); existing != nil {
		respondDevError(w, http.StatusConflict, formationID,
			fmt.Sprintf("Draft formation %s is already running on port %d. Run 'muxi down' first.", formationID, existing.Port))
		return
	}

	// Create draft formation entry to allocate port
	draftFormation := &registry.Formation{
		ID:         formationID,
		Name:       formationConfig.Name,
		Version:    formationConfig.Version,
		Status:     "starting",
		DeployedAt: time.Now(),
	}

	if err := s.registry.RegisterDraft(draftFormation); err != nil {
		respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Failed to allocate port: %v", err))
		return
	}

	port := draftFormation.Port

	// Compute environment variables
	bindHost := getBindHost(s.config.Formations.BindHost)
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)

	// Build spawn config
	// Use -draft suffix (not :draft) because container names don't allow colons
	envVars := formationConfig.GetEnvironmentVars(port, serverURL, bindHost)

	// Inject RCE connection info if available
	if s.rceManager != nil {
		s.rceManager.InjectEnvVars(envVars)
	}

	spawnConfig := process.SpawnConfig{
		ID:           formationID + "-draft",
		WorkDir:      workDir,
		Port:         port,
		Env:          envVars,
		AutoRestart:  false, // No auto-restart for dev formations
		TruncateLogs: true,  // Fresh logs each dev run
		RuntimeType:  "native",
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		muxiDir, err := getMuxiDir()
		if err != nil {
			s.registry.UnregisterDraft(formationID)
			respondDevError(w, http.StatusInternalServerError, formationID, err.Error())
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			s.registry.UnregisterDraft(formationID)
			respondDevError(w, http.StatusInternalServerError, formationID, err.Error())
			return
		}

		runtimeRegistryPath := filepath.Join(runtimesDir, "registry.json")
		runtimeRegistry := runtime.NewRegistry(runtimeRegistryPath)
		if err := runtimeRegistry.Load(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to load runtimes registry")
		}

		availableVersions := runtimeRegistry.List()
		resolver := runtime.NewResolver(availableVersions, runtimesDir)

		resolvedVersion, err := resolver.Resolve(formationConfig.MuxiRuntime)
		if err != nil {
			s.registry.UnregisterDraft(formationID)
			respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Failed to resolve runtime: %v", err))
			return
		}

		// Create downloader for auto-download
		downloader := runtime.NewDownloader(
			s.config.Runtime.SIFBaseURL,
			s.config.Runtime.RuntimeRunnerImage,
			runtimesDir,
			s.logger,
		)

		// Ensure SIF exists (download if missing)
		sifPath, _, _, err := downloader.EnsureSIF(resolvedVersion)
		if err != nil {
			s.registry.UnregisterDraft(formationID)
			respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Failed to download runtime: %v", err))
			return
		}

		_, err = downloader.EnsureRuntimeRunner()
		if err != nil {
			s.registry.UnregisterDraft(formationID)
			respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Failed to pull runtime-runner: %v", err))
			return
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.runtime.utils.run_formation",
			"/formation",
			"--port", fmt.Sprintf("%d", port),
			"--host", bindHost,
		}
	}

	// Spawn the process
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.registry.UnregisterDraft(formationID)
		respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Failed to start formation: %v", err))
		return
	}

	// Update draft formation with process info
	s.registry.UpdateDraft(formationID, func(f *registry.Formation) {
		f.ProcessID = proc.PID
		f.Command = spawnConfig.Command
		f.Args = spawnConfig.Args
		f.StartedAt = time.Now()
	})

	// Wait for health check
	time.Sleep(2 * time.Second)

	healthTimeout := 300 * time.Second
	healthInterval := 1 * time.Second
	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthChecker.Endpoint = "/v1/health"

	healthErr := healthChecker.WaitForHealthyWithPID(port, formationID, proc.PID, proc.LogFile, nil)
	if healthErr != nil {
		log.Error().Err(healthErr).Str("id", formationID).Msg("Draft formation failed health check")
		s.processManager.Stop(spawnConfig.ID)
		s.registry.UnregisterDraft(formationID)
		respondDevError(w, http.StatusInternalServerError, formationID, fmt.Sprintf("Health check failed: %v", healthErr))
		return
	}

	// Update status to running
	proc.SetStatus(process.StatusRunning)
	s.registry.UpdateDraft(formationID, func(f *registry.Formation) {
		f.Status = "running"
		f.Healthy = true
		f.LastHealthCheck = time.Now()
	})

	log.Info().
		Str("formation_id", formationID).
		Int("port", port).
		Msg("Draft formation started successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DevRunResponse{
		Success:     true,
		FormationID: formationID,
		Port:        port,
		Status:      "running",
	})
}

// HandleDevStop handles POST /rpc/dev/stop
// Stops a draft formation
func (s *Server) HandleDevStop(w http.ResponseWriter, r *http.Request) {
	var req DevStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondDevError(w, http.StatusBadRequest, "", "Invalid request body")
		return
	}

	if req.FormationID == "" {
		respondDevError(w, http.StatusBadRequest, "", "Missing formation_id")
		return
	}

	formationID := req.FormationID

	log.Info().
		Str("formation_id", formationID).
		Msg("Stopping draft formation")

	// Get draft formation
	draftFormation, err := s.registry.GetDraft(formationID)
	if err != nil {
		respondDevError(w, http.StatusNotFound, formationID, fmt.Sprintf("Draft formation %s not found", formationID))
		return
	}

	// Stop the process
	processID := formationID + "-draft"
	if err := s.processManager.Stop(processID); err != nil {
		log.Warn().Err(err).Str("id", formationID).Msg("Failed to stop draft process (may already be stopped)")
	}

	// Unregister draft formation
	if err := s.registry.UnregisterDraft(formationID); err != nil {
		log.Warn().Err(err).Str("id", formationID).Msg("Failed to unregister draft formation")
	}

	log.Info().
		Str("formation_id", formationID).
		Int("port", draftFormation.Port).
		Msg("Draft formation stopped")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DevStopResponse{
		Success:     true,
		FormationID: formationID,
		Status:      "stopped",
	})
}

// respondDevError sends a JSON error response for dev endpoints
func respondDevError(w http.ResponseWriter, status int, formationID, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(DevRunResponse{
		Success:     false,
		FormationID: formationID,
		Error:       message,
	})
}
