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
	"github.com/muxi-ai/server/pkg/telemetry"
)

// Rollback stages
const (
	StageRollbackValidating    DeployStage = "validating"
	StageRollbackSpawnPrevious DeployStage = "spawning_previous"
	StageRollbackStopping      DeployStage = "stopping_current"
	StageRollbackSwapping      DeployStage = "swapping"
)

// HandleRollback handles POST /rpc/formations/{id}/rollback
// Uses blue-green deployment: starts previous version on staging port,
// health checks it, then switches over (keeping current running until previous is healthy)
func (s *Server) HandleRollback(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	s.logger.Info().Str("id", formationID).Msg("Rolling back formation (blue-green)")

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

	// Get formation base directory from config
	formationBaseDir := filepath.Join(s.config.Formations.FormationsDir, formationID)

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

	// Allocate staging port for previous version (blue-green)
	stagingPort, err := s.registry.AllocatePort(formationID + "-rollback")
	if err != nil {
		s.logger.Error().Err(err).Msg("No available ports for rollback staging")
		respondErr(http.StatusInsufficientStorage, StageRollbackValidating, "PortAllocationError", "No available ports for rollback")
		return
	}
	defer func() {
		// Clean up staging port if rollback fails
		if stagingPort != 0 {
			s.registry.ReleasePort(stagingPort)
		}
	}()

	s.logger.Info().
		Str("id", formationID).
		Int("current_port", existingFormation.Port).
		Int("staging_port", stagingPort).
		Msg("Allocated staging port for blue-green rollback")

	// Find and parse formation config from PREVIOUS version (we're rolling back to it)
	formationConfigPath, err := formation.FindFormationFile(previousDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to find formation config from previous version")
		respondErr(http.StatusInternalServerError, StageRollbackValidating, "ParseError", fmt.Sprintf("Failed to find formation config: %v", err))
		return
	}
	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation config from previous version")
		respondErr(http.StatusInternalServerError, StageRollbackValidating, "ParseError", fmt.Sprintf("Failed to parse formation config: %v", err))
		return
	}

	// Get environment variables for staging port
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(stagingPort, serverURL, bindHost)

	// Build spawn config for previous version on staging port
	stagingProcessID := formationID + "-rollback"
	spawnConfig := process.SpawnConfig{
		ID:                     stagingProcessID,
		Name:                   formationConfig.Name,
		Command:                formationConfig.GetDefaultCommand(),
		Args:                   formationConfig.GetDefaultArgs(),
		Port:                   stagingPort,
		WorkDir:                previousDir, // Run from previous directory
		Env:                    envVars,
		AutoRestart:            false, // Don't auto-restart staging
		RuntimeType:            "native",
		SkipInitialHealthCheck: true,
	}

	// Handle runtime resolution if specified
	if formationConfig.MuxiRuntime != "" {
		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: "Resolving runtime version...",
		})

		// Get base dir from FormationsDir (e.g., ~/.muxi/server/formations -> ~/.muxi/server)
		baseDir := filepath.Dir(s.config.Formations.FormationsDir)
		runtimesDir := filepath.Join(baseDir, "runtimes")
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
			"/formation",
			"--port", fmt.Sprintf("%d", stagingPort),
			"--host", bindHost,
		}
	}

	// Stage: Spawn previous version on staging port
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackSpawnPrevious,
		Message: fmt.Sprintf("Starting previous version on staging port %d...", stagingPort),
	})

	stagingProc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to start previous version")
		respondErr(http.StatusInternalServerError, StageRollbackSpawnPrevious, "SpawnError", fmt.Sprintf("Failed to start previous version: %v", err))
		return
	}

	// Stage: Health check staging
	progress.Emit(ProgressEvent{
		Stage:   StageHealthCheck,
		Message: "Waiting for previous version health check...",
	})

	time.Sleep(2 * time.Second)

	healthTimeout := 300 * time.Second
	healthInterval := 1 * time.Second
	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthChecker.Endpoint = "/v1/health"

	healthErr := healthChecker.WaitForHealthyWithPID(stagingPort, stagingProcessID, stagingProc.PID, stagingProc.LogFile, func(attempt, maxAttempts int) {
		progress.Emit(ProgressEvent{
			Stage:       StageHealthCheck,
			Message:     fmt.Sprintf("Health check attempt %d/%d...", attempt, maxAttempts),
			Attempt:     &attempt,
			MaxAttempts: &maxAttempts,
		})
	})

	if healthErr != nil {
		s.logger.Error().Err(healthErr).Str("id", formationID).Msg("Previous version failed health check")
		// Kill staging process
		s.processManager.ForceKill(stagingProcessID)
		respondErr(http.StatusInternalServerError, StageHealthCheck, "HealthCheckFailed", 
			fmt.Sprintf("Previous version failed health check: %v. Current version still running.", healthErr))
		return
	}

	s.logger.Info().
		Str("id", formationID).
		Int("staging_port", stagingPort).
		Msg("Previous version is healthy - proceeding with switchover")

	// Stage: Stop current version
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackStopping,
		Message: "Stopping current version...",
	})

	if err := s.processManager.Stop(formationID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to stop current formation gracefully, force killing")
		if err := s.processManager.ForceKill(formationID); err != nil {
			s.logger.Error().Err(err).Msg("Failed to force kill current formation (continuing anyway)")
		}
	}

	// Wait for port to be released (Docker can take a moment)
	s.logger.Info().Int("port", existingFormation.Port).Msg("Waiting for port to be released...")
	for i := 0; i < 10; i++ {
		if registry.IsPortAvailable(existingFormation.Port) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Stage: Swap directories
	progress.Emit(ProgressEvent{
		Stage:   StageRollbackSwapping,
		Message: "Swapping versions...",
	})

	// Three-way swap: current -> temp, previous -> current, temp -> previous
	tempDir := filepath.Join(formationBaseDir, "temp")
	if err := os.Rename(currentDir, tempDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move current to temp")
		// Try to keep staging running but this is bad state
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "SwapError", "Failed to swap directories")
		return
	}

	if err := os.Rename(previousDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move previous to current")
		os.Rename(tempDir, currentDir) // Try to restore
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "SwapError", "Failed to swap directories")
		return
	}

	if err := os.Rename(tempDir, previousDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move temp to previous")
		// Don't fail - the important swap is done
	}

	// Kill staging process (it was running from previousDir which is now currentDir)
	s.processManager.ForceKill(stagingProcessID)

	// Now start the formation properly from current dir on the original port
	finalEnvVars := formationConfig.GetEnvironmentVars(existingFormation.Port, serverURL, bindHost)
	finalSpawnConfig := process.SpawnConfig{
		ID:                     formationID,
		Name:                   formationConfig.Name,
		Command:                formationConfig.GetDefaultCommand(),
		Args:                   formationConfig.GetDefaultArgs(),
		Port:                   existingFormation.Port,
		WorkDir:                currentDir, // Now points to the previous version
		Env:                    finalEnvVars,
		AutoRestart:            s.config.Formations.AutoRestart,
		RuntimeType:            spawnConfig.RuntimeType,
		SIFPath:                spawnConfig.SIFPath,
		SkipInitialHealthCheck: true,
	}

	if formationConfig.MuxiRuntime != "" {
		finalSpawnConfig.Command = "python"
		finalSpawnConfig.Args = []string{
			"-m", "muxi.utils.run_formation",
			"/formation",
			"--port", fmt.Sprintf("%d", existingFormation.Port),
			"--host", bindHost,
		}
	}

	finalProc, err := s.processManager.Start(finalSpawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to start formation on original port")
		respondErr(http.StatusInternalServerError, StageRollbackSwapping, "SpawnError", fmt.Sprintf("Failed to restart formation: %v", err))
		return
	}

	// Update version history
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

	// Update registry
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.ProcessID = finalProc.PID
		f.Status = "running"
		f.Healthy = true
		f.Version = formationConfig.Version
	})

	// Clear staging port from defer cleanup
	stagingPort = 0

	s.logger.Info().
		Str("id", formationID).
		Int("version", history.CurrentVersion).
		Int("pid", finalProc.PID).
		Msg("Formation rolled back successfully (blue-green)")

	// Track telemetry
	telemetry.IncrementRollback()

	if wantsSSE && sseInitialized {
		progress.Complete(CompleteEvent{
			FormationID:     formationID,
			Port:            existingFormation.Port,
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
			"pid":              finalProc.PID,
			"message":          "Rolled back to previous version",
		}
		RespondSuccess(w, response)
	}
}
