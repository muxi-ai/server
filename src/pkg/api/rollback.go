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
)

// Rollback stages
const (
	StageRollbackValidating DeployStage = "validating"
	StageRollbackStopping   DeployStage = "stopping"
	StageRollbackSwapping   DeployStage = "swapping"
)

// HandleRollback handles POST /rpc/formations/{id}/rollback
// Rolls back a formation to its previous version
func (s *Server) HandleRollback(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	s.logger.Info().Str("id", formationID).Msg("Rolling back formation")

	// Check if client wants SSE
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

	_ = sse // Silence unused variable warning

	// Stage: Validating
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackValidating,
		Message: "Validating rollback request...",
	})

	// Get existing formation
	existingFormation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation not found")
		respondErr(http.StatusNotFound, StageRollbackValidating, "NotFound", "Formation not found")
		return
	}

	// Get formation base directory
	muxiDir := s.config.Formations.FormationsDir
	if muxiDir == "" {
		muxiDir = "formations"
	}
	formationBaseDir := filepath.Join(muxiDir, formationID)

	// Load version history
	history, err := formation.LoadVersionHistory(formationBaseDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load version history")
		respondErr(http.StatusInternalServerError, StageRollbackValidating, "HistoryError", "Failed to load version history")
		return
	}

	// Check if previous version exists
	if !history.HasPreviousVersion() {
		s.logger.Warn().Str("id", formationID).Msg("No previous version to rollback to")
		respondErr(http.StatusBadRequest, StageRollbackValidating, "NoPreviousVersion", "No previous version available for rollback")
		return
	}

	currentDir := filepath.Join(formationBaseDir, "current")
	previousDir := filepath.Join(formationBaseDir, "previous")
	tempDir := filepath.Join(formationBaseDir, "temp")

	// Ensure both directories exist
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		s.logger.Error().Msg("Current directory does not exist")
		respondErr(http.StatusInternalServerError, StageRollbackValidating, "DirectoryError", "Current version directory not found")
		return
	}
	if _, err := os.Stat(previousDir); os.IsNotExist(err) {
		s.logger.Error().Msg("Previous directory does not exist")
		respondErr(http.StatusBadRequest, StageRollbackValidating, "DirectoryError", "Previous version directory not found")
		return
	}

	// Stage: Stopping current formation
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackStopping,
		Message: "Stopping current formation...",
	})

	if err := s.processManager.Stop(formationID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to stop formation (continuing anyway)")
	}

	// Stage: Swapping versions
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackSwapping,
		Message: "Swapping to previous version...",
	})

	// Three-way swap
	if err := os.Rename(currentDir, tempDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move current to temp")
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "SwapError", "Failed to perform rollback (step 1)")
		return
	}

	if err := os.Rename(previousDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move previous to current")
		os.Rename(tempDir, currentDir) // Try to restore
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "SwapError", "Failed to perform rollback (step 2)")
		return
	}

	if err := os.Rename(tempDir, previousDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move temp to previous")
		// Don't fail - the important swap is done
	}

	// Swap version history
	tempVersion := history.Current
	tempVersionNum := history.CurrentVersion

	history.Current = history.Previous
	history.CurrentVersion = history.PreviousVersion

	history.Previous = tempVersion
	history.PreviousVersion = tempVersionNum

	if history.Current != nil {
		history.Current.BackupPath = "current"
	}
	if history.Previous != nil {
		history.Previous.BackupPath = "previous"
	}

	if err := history.Save(formationBaseDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save version history")
	}

	// Parse formation.yaml from rolled-back version
	formationYAMLPath := filepath.Join(currentDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation.yaml")
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "ParseError", fmt.Sprintf("Failed to parse formation.yaml: %v", err))
		return
	}

	// Get environment variables
	port := existingFormation.Port
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(port, serverURL, bindHost)

	// Build spawn config
	spawnConfig := process.SpawnConfig{
		ID:                     formationID,
		Name:                   formationConfig.Name,
		Command:                formationConfig.GetDefaultCommand(),
		Args:                   formationConfig.GetDefaultArgs(),
		Port:                   port,
		WorkDir:                currentDir,
		Env:                    envVars,
		AutoRestart:            s.config.Formations.AutoRestart,
		RuntimeType:            "native",
		SkipInitialHealthCheck: true,
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: "Resolving runtime version...",
		})

		muxiDir, err := getMuxiDir()
		if err != nil {
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "DirectoryError", err.Error())
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "DirectoryError", err.Error())
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
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "ResolveError", fmt.Sprintf("Failed to resolve runtime: %v", err))
			return
		}

		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: fmt.Sprintf("Resolved runtime version: %s", resolvedVersion),
			Version: resolvedVersion,
		})

		downloader := runtime.NewDownloader(
			s.config.Runtime.SIFBaseURL,
			s.config.Runtime.RuntimeRunnerImage,
			runtimesDir,
			s.logger,
		)

		var sifPath string
		if s.config.Runtime.AutoDownload {
			var sifDownloaded bool
			sifPath, sifDownloaded, err = downloader.EnsureSIF(resolvedVersion)
			if err != nil {
				respondErr(http.StatusInternalServerError, StageDownloadingSIF, "DownloadError", fmt.Sprintf("Failed to download runtime: %v", err))
				return
			}

			if sifDownloaded {
				progress.Emit(ProgressEvent{
					Stage:   StageDownloadingSIF,
					Message: "Downloaded runtime image",
				})
			}

			runnerPulled, err := downloader.EnsureRuntimeRunner()
			if err != nil {
				respondErr(http.StatusInternalServerError, StagePullingRunner, "PullError", fmt.Sprintf("Failed to pull runtime-runner: %v", err))
				return
			}

			if goruntime.GOOS != "linux" && runnerPulled {
				progress.Emit(ProgressEvent{
					Stage:   StagePullingRunner,
					Message: "Pulled runtime runner",
				})
			}
		} else {
			sifPath = resolver.GetSIFPath(resolvedVersion)
			if _, err := os.Stat(sifPath); os.IsNotExist(err) {
				respondErr(http.StatusNotFound, StageDownloadingSIF, "RuntimeNotFound", fmt.Sprintf("Runtime %s not found", resolvedVersion))
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

	// Stage: Spawning
	progress.Emit(ProgressEvent{
		Stage:   StageSpawning,
		Message: "Starting formation...",
	})

	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to restart formation")
		respondErr(http.StatusInternalServerError, StageSpawning, "SpawnError", fmt.Sprintf("Failed to restart formation: %v", err))
		return
	}

	// Update registry
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.ProcessID = proc.PID
		f.Status = "starting"
		f.Version = formationConfig.Version
	})

	// Stage: Health check
	progress.Emit(ProgressEvent{
		Stage:   StageHealthCheck,
		Message: "Waiting for formation health check...",
	})

	time.Sleep(2 * time.Second)

	healthTimeout := 300 * time.Second
	healthInterval := 1 * time.Second
	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthChecker.Endpoint = "/v1/health"

	healthErr := healthChecker.WaitForHealthyWithPID(port, formationID, proc.PID, proc.LogFile, func(attempt, maxAttempts int) {
		progress.Emit(ProgressEvent{
			Stage:       StageHealthCheck,
			Message:     fmt.Sprintf("Health check attempt %d/%d...", attempt, maxAttempts),
			Attempt:     &attempt,
			MaxAttempts: &maxAttempts,
		})
	})

	if healthErr != nil {
		s.logger.Error().Err(healthErr).Str("id", formationID).Msg("Formation failed health check after rollback")
		respondErr(http.StatusInternalServerError, StageHealthCheck, "HealthCheckFailed", healthErr.Error())
		return
	}

	// Update registry to running
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.Status = "running"
		f.Healthy = true
	})

	s.logger.Info().
		Str("id", formationID).
		Int("version", history.CurrentVersion).
		Int("pid", proc.PID).
		Msg("Formation rolled back successfully")

	if wantsSSE && sseInitialized {
		progress.Complete(CompleteEvent{
			FormationID:     formationID,
			Port:            port,
			Status:          "running",
			URL:             fmt.Sprintf("http://localhost:%d/api/%s", s.config.Server.Port, formationID),
			PreviousVersion: fmt.Sprintf("%d", history.PreviousVersion),
			NewVersion:      fmt.Sprintf("%d", history.CurrentVersion),
		})
	} else {
		response := map[string]interface{}{
			"id":               formationID,
			"status":           "running",
			"version":          history.CurrentVersion,
			"previous_version": history.PreviousVersion,
			"pid":              proc.PID,
			"message":          "Rolled back to previous version",
		}
		RespondSuccess(w, response)
	}
}
