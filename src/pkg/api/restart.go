package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/muxi-ai/server/pkg/runtime"
	"github.com/rs/zerolog/log"
)

// Restart stages for SSE progress
const (
	StageStopping        = "stopping"
	StageRestartSpawning = "spawning"
)

// HandleRestart handles POST /formations/{id}/restart
// Restarts a formation by stopping current process and starting fresh
// Re-reads formation config to ensure runtime type is correct
// Supports SSE streaming with Accept: text/event-stream header
func (s *Server) HandleRestart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	formationID := vars["id"]

	// Check if client wants SSE streaming
	acceptHeader := r.Header.Get("Accept")
	wantsSSE := strings.Contains(acceptHeader, "text/event-stream")

	var sse *SSEWriter
	sseInitialized := false
	progress := NewProgressEmitter(nil)

	// Initialize SSE
	if wantsSSE {
		var ok bool
		sse, ok = NewSSEWriter(w)
		if ok {
			sse.Init()
			sseInitialized = true
			progress = NewProgressEmitter(sse)
		}
	}

	// Helper to respond with error
	respondErr := func(status int, stage DeployStage, errType, message string) {
		if wantsSSE && sseInitialized {
			progress.Error(ErrorEvent{
				Error:   errType,
				Message: message,
				Stage:   stage,
			})
		} else {
			RespondError(w, status, message)
		}
	}

	log.Info().
		Str("formation_id", formationID).
		Bool("sse", wantsSSE).
		Msg("Restarting formation")

	// Check if formation exists
	formationReg, err := s.registry.Get(formationID)
	if err != nil {
		log.Warn().
			Err(err).
			Str("formation_id", formationID).
			Msg("Formation not found")
		respondErr(http.StatusNotFound, StageStopping, "NotFound", "Formation not found")
		return
	}

	port := formationReg.Port

	// Get old restart count before stopping
	oldRestartCount := formationReg.RestartCount
	proc, _ := s.processManager.Get(formationID)
	if proc != nil {
		oldRestartCount = proc.RestartCount
	}

	// Stage 1: Stop the current process
	progress.Emit(ProgressEvent{
		Stage:   StageStopping,
		Message: "Stopping current process...",
	})

	if err := s.processManager.Stop(formationID); err != nil {
		log.Warn().Err(err).Str("id", formationID).Msg("Failed to stop process (may already be stopped)")
	}

	// Get MUXI directory
	muxiDir, err := getMuxiDir()
	if err != nil {
		respondErr(http.StatusInternalServerError, StageStopping, "ConfigError", err.Error())
		return
	}

	// Check formation directory exists
	formationDir := filepath.Join(muxiDir, "formations", formationID)
	if _, err := os.Stat(formationDir); os.IsNotExist(err) {
		respondErr(http.StatusInternalServerError, StageStopping, "NotFound", "Formation directory not found")
		return
	}

	// Parse formation.yaml
	progress.Emit(ProgressEvent{
		Stage:   StageValidating,
		Message: "Loading formation configuration...",
	})

	formationYAMLPath := filepath.Join(formationDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		respondErr(http.StatusInternalServerError, StageValidating, "ParseError", err.Error())
		return
	}

	// Compute environment variables
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)

	// Build spawn config
	spawnConfig := process.SpawnConfig{
		ID:          formationID,
		WorkDir:     formationDir,
		Port:        port,
		Env:         formationConfig.GetEnvironmentVars(port, serverURL, bindHost),
		AutoRestart: s.config.Formations.AutoRestart,
		RuntimeType: "native",
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: fmt.Sprintf("Resolving runtime version: %s", formationConfig.MuxiRuntime),
		})

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "RuntimeError", err.Error())
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
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "RuntimeError", err.Error())
			return
		}

		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: fmt.Sprintf("Resolved runtime version: %s", resolvedVersion),
			Version: resolvedVersion,
		})

		// Create downloader for auto-download
		downloader := runtime.NewDownloader(
			s.config.Runtime.SIFBaseURL,
			s.config.Runtime.RuntimeRunnerImage,
			runtimesDir,
			s.logger,
		)

		// Ensure SIF exists (download if missing and auto_download enabled)
		var sifPath string
		if s.config.Runtime.AutoDownload {
			progress.Emit(ProgressEvent{
				Stage:   StageDownloadingSIF,
				Message: "Checking/downloading SIF runtime...",
			})

			sifPath, _, err = downloader.EnsureSIF(resolvedVersion)
			if err != nil {
				respondErr(http.StatusInternalServerError, StageDownloadingSIF, "DownloadError", fmt.Sprintf("Failed to download runtime: %v", err))
				return
			}

			// Ensure runtime-runner is available (macOS/Windows)
			if goruntime.GOOS != "linux" {
				progress.Emit(ProgressEvent{
					Stage:   StagePullingRunner,
					Message: "Checking/pulling runtime-runner Docker image...",
				})
			}

			if _, err := downloader.EnsureRuntimeRunner(); err != nil {
				respondErr(http.StatusInternalServerError, StagePullingRunner, "PullError", fmt.Sprintf("Failed to pull runtime-runner: %v", err))
				return
			}
		} else {
			// Auto-download disabled, just get path and check existence
			sifPath = resolver.GetSIFPath(resolvedVersion)
			if _, err := os.Stat(sifPath); os.IsNotExist(err) {
				respondErr(http.StatusNotFound, StageDownloadingSIF, "RuntimeNotFound",
					fmt.Sprintf("Runtime %s not found at %s. Enable auto_download or manually install.", resolvedVersion, sifPath))
				return
			}
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.utils.run_formation",
			"/formation/formation.yaml",
			"--port", fmt.Sprintf("%d", port),
			"--host", bindHost,
		}
	}

	// Stage: Spawn the process
	progress.Emit(ProgressEvent{
		Stage:   StageRestartSpawning,
		Message: "Starting formation...",
	})

	newProc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		respondErr(http.StatusInternalServerError, StageRestartSpawning, "SpawnError", err.Error())
		return
	}

	// Update registry with new process info
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.ProcessID = newProc.PID
		f.Status = "starting"
	})

	// Stage: Health check
	progress.Emit(ProgressEvent{
		Stage:   StageHealthCheck,
		Message: "Waiting for formation health check...",
	})

	// Wait for formation to become healthy
	time.Sleep(2 * time.Second)

	healthTimeout := 300 * time.Second
	healthInterval := 1 * time.Second
	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthChecker.Endpoint = "/v1/health"

	healthErr := healthChecker.WaitForHealthyWithPID(port, formationID, newProc.PID, newProc.LogFile, func(attempt, maxAttempts int) {
		progress.Emit(ProgressEvent{
			Stage:       StageHealthCheck,
			Message:     fmt.Sprintf("Health check attempt %d/%d...", attempt, maxAttempts),
			Attempt:     &attempt,
			MaxAttempts: &maxAttempts,
		})
	})

	if healthErr != nil {
		log.Error().Err(healthErr).Str("id", formationID).Msg("Formation failed health check after restart")
		respondErr(http.StatusInternalServerError, StageHealthCheck, "HealthCheckFailed", healthErr.Error())
		return
	}

	// Update status and restart count
	newRestartCount := oldRestartCount + 1
	newProc.RestartCount = newRestartCount
	newProc.SetStatus(process.StatusRunning)

	s.registry.UpdateHealthCheck(formationID, true)
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.Status = "running"
		f.RestartCount = newRestartCount
	})

	log.Info().
		Str("formation_id", formationID).
		Int("restart_count", newRestartCount).
		Msg("Formation restarted successfully")

	if wantsSSE && sseInitialized {
		progress.Complete(CompleteEvent{
			FormationID:  formationID,
			Port:         port,
			Status:       "running",
			RestartCount: newRestartCount,
		})
	} else {
		RespondSuccess(w, map[string]interface{}{
			"id":            formationID,
			"status":        "running",
			"message":       "Formation restarted",
			"restart_count": newRestartCount,
			"port":          port,
		})
	}
}
