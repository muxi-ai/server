package api

import (
	"fmt"
	"io"
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

// HandleUpdate handles PUT /rpc/formations/{id}
// Updates a formation to a new version using zero-downtime blue-green deployment
func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	formationID := mux.Vars(r)["id"]

	// Check if client wants SSE streaming
	acceptHeader := r.Header.Get("Accept")
	wantsSSE := strings.Contains(acceptHeader, "text/event-stream")

	var sse *SSEWriter
	sseInitialized := false

	// Initialize progress emitter (no-op until SSE is initialized)
	progress := NewProgressEmitter(nil)

	// Helper to initialize SSE (delayed until after body is read)
	initSSE := func() {
		if wantsSSE && !sseInitialized {
			var ok bool
			sse, ok = NewSSEWriter(w)
			if ok {
				sse.Init()
				sseInitialized = true
				progress = NewProgressEmitter(sse)
			}
		}
	}

	// Helper to respond with error (handles both SSE and regular responses)
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

	// Optional: X-Formation-Version header (defaults to "1.0.0")
	headerVersion := r.Header.Get("X-Formation-Version")
	if headerVersion == "" {
		headerVersion = "1.0.0"
	}

	s.logger.Info().
		Str("id", formationID).
		Str("version", headerVersion).
		Msg("Starting zero-downtime deployment")

	// Get existing formation
	existingFormation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation not found")
		RespondError(w, http.StatusNotFound, "Formation not found")
		return
	}

	// Mark formation as deploying (prevents concurrent updates)
	if err := s.registry.SetDeploying(formationID, true); err != nil {
		s.logger.Warn().Err(err).Str("id", formationID).Msg("Formation is already being deployed")
		RespondError(w, http.StatusConflict, "Formation is already being updated")
		return
	}
	defer s.registry.SetDeploying(formationID, false)

	// Create temporary file for uploaded bundle
	tmpFile, err := os.CreateTemp("", "formation-update-*.tar.gz")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create temp file")
		RespondError(w, http.StatusInternalServerError, "Failed to create temp file")
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy uploaded data to temp file
	bundleData, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to read bundle")
		RespondError(w, http.StatusInternalServerError, "Failed to read bundle")
		return
	}

	if _, err := tmpFile.Write(bundleData); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save bundle")
		RespondError(w, http.StatusInternalServerError, "Failed to save bundle")
		return
	}
	tmpFile.Close()

	// Now that body is fully read, we can initialize SSE streaming
	initSSE()

	// Emit extracting progress
	progress.Emit(ProgressEvent{
		Stage:   StageExtracting,
		Message: "Extracting bundle to staging...",
	})

	// Get formation base directory
	muxiDir, err := getMuxiDir()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
		respondErr(http.StatusInternalServerError, StageExtracting, "DirectoryError", "Failed to get MUXI directory")
		return
	}
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)

	// Load version history
	history, err := formation.LoadVersionHistory(formationBaseDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load version history")
		respondErr(http.StatusInternalServerError, StageExtracting, "HistoryError", "Failed to load version history")
		return
	}

	// Check if bundle is identical to current version (no change)
	newBundleHash := formation.ComputeBundleHash(bundleData)
	if history.Current != nil && history.Current.BundleHash == newBundleHash {
		s.logger.Warn().
			Str("id", formationID).
			Str("hash", newBundleHash).
			Msg("Bundle is identical to current version")
		respondErr(http.StatusConflict, StageValidating, "NoChange", "Bundle is identical to current version - nothing to update")
		return
	}

	// Allocate NEW port for staging version (blue-green deployment)
	stagingPort, err := s.registry.AllocatePort(formationID + "-staging")
	if err != nil {
		s.logger.Error().Err(err).Msg("No available ports for staging")
		respondErr(http.StatusInsufficientStorage, StageExtracting, "PortAllocationError", "No available ports for staging deployment")
		return
	}
	defer func() {
		// Clean up staging port if deployment fails
		if stagingPort != 0 {
			s.registry.ReleasePort(stagingPort)
		}
	}()

	s.logger.Info().
		Str("id", formationID).
		Int("current_port", existingFormation.Port).
		Int("staging_port", stagingPort).
		Msg("Allocated staging port for blue-green deployment")

	// Update registry with staging port
	if err := s.registry.SetStagingPort(formationID, stagingPort); err != nil {
		s.logger.Error().Err(err).Msg("Failed to set staging port")
		respondErr(http.StatusInternalServerError, StageExtracting, "RegistryError", "Failed to set staging port")
		return
	}

	// Create staging directory for new version
	currentDir := filepath.Join(formationBaseDir, "current")
	previousDir := filepath.Join(formationBaseDir, "previous")
	stagingDir := filepath.Join(formationBaseDir, "staging")

	// Remove old staging if exists
	if _, err := os.Stat(stagingDir); err == nil {
		if err := os.RemoveAll(stagingDir); err != nil {
			s.logger.Error().Err(err).Msg("Failed to remove old staging")
			respondErr(http.StatusInternalServerError, StageExtracting, "CleanupError", "Failed to clean staging directory")
			return
		}
	}

	// Extract new version to staging/
	extractDir, err := os.MkdirTemp("", "formation-extract-*")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create extraction directory")
		respondErr(http.StatusInternalServerError, StageExtracting, "ExtractDirError", "Failed to create extraction directory")
		return
	}
	defer os.RemoveAll(extractDir)

	newFormationDir, err := formation.ExtractBundle(tmpFile.Name(), extractDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to extract bundle")
		respondErr(http.StatusBadRequest, StageExtracting, "ExtractError", fmt.Sprintf("Failed to extract bundle: %v", err))
		return
	}

	// Move extracted formation to staging/
	if err := os.Rename(newFormationDir, stagingDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move new version to staging")
		respondErr(http.StatusInternalServerError, StageExtracting, "MoveError", "Failed to move new version to staging")
		return
	}

	// Emit validating progress
	progress.Emit(ProgressEvent{
		Stage:   StageValidating,
		Message: "Validating formation config...",
	})

	// Find and parse staging formation config (formation.afs/yaml/yml)
	formationConfigPath, err := formation.FindFormationFile(stagingDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to find formation config")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusBadRequest, StageValidating, "ParseError", fmt.Sprintf("Failed to find formation config: %v", err))
		return
	}
	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation config")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusBadRequest, StageValidating, "ParseError", fmt.Sprintf("Failed to parse formation config: %v", err))
		return
	}

	// Validate secrets references
	if err := formation.ValidateSecrets(stagingDir); err != nil {
		s.logger.Error().Err(err).Msg("Secrets validation failed")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusBadRequest, StageValidating, "SecretsError", err.Error())
		return
	}

	// Verify bundle's version matches the header
	bundleVersion := formationConfig.Version
	if bundleVersion == "" {
		bundleVersion = "1.0.0" // Default if not specified in bundle
	}
	if bundleVersion != headerVersion {
		s.logger.Warn().
			Str("header_version", headerVersion).
			Str("bundle_version", bundleVersion).
			Msg("Formation version mismatch between header and bundle")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusBadRequest, StageValidating, "VersionMismatch",
			fmt.Sprintf("Formation version mismatch: header says '%s' but bundle contains '%s'",
				headerVersion, bundleVersion))
		return
	}

	// Inject metadata
	serverID, _ := formation.GenerateServerID()
	if err := formation.InjectMetadata(stagingDir, serverID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to inject metadata (continuing anyway)")
	}

	// Get environment variables (use staging port)
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(stagingPort, serverURL, bindHost)

	// Build spawn config
	stagingProcessID := formationID + "-staging"
	spawnConfig := process.SpawnConfig{
		ID:                     stagingProcessID,
		Name:                   formationConfig.Name + " (staging)",
		Command:                formationConfig.GetDefaultCommand(),
		Args:                   formationConfig.GetDefaultArgs(),
		Port:                   stagingPort,
		WorkDir:                stagingDir,
		Env:                    envVars,
		AutoRestart:            false, // Don't auto-restart staging
		SkipInitialHealthCheck: true,  // Update handler does its own health check with progress
		RuntimeType:            "native",
	}

	// Handle runtime resolution if specified in formation config
	if formationConfig.MuxiRuntime != "" {
		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: fmt.Sprintf("Resolving runtime version: %s", formationConfig.MuxiRuntime),
		})

		muxiDir, err := getMuxiDir()
		if err != nil {
			os.RemoveAll(stagingDir)
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "DirectoryError", err.Error())
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			os.RemoveAll(stagingDir)
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
			os.RemoveAll(stagingDir)
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

		var sifPath string
		if s.config.Runtime.AutoDownload {
			var sifDownloaded bool
			sifPath, sifDownloaded, err = downloader.EnsureSIF(resolvedVersion)
			if err != nil {
				os.RemoveAll(stagingDir)
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
				os.RemoveAll(stagingDir)
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
				os.RemoveAll(stagingDir)
				respondErr(http.StatusNotFound, StageDownloadingSIF, "RuntimeNotFound",
					fmt.Sprintf("Runtime %s not found at %s. Enable auto_download or manually install.", resolvedVersion, sifPath))
				return
			}
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.runtime.utils.run_formation",
			"/formation",
			"--port", fmt.Sprintf("%d", stagingPort),
			"--host", bindHost,
		}
	}

	// Emit spawning staging progress
	progress.Emit(ProgressEvent{
		Stage:       StageSpawningStaging,
		Message:     fmt.Sprintf("Starting staging version on port %d", stagingPort),
		StagingPort: &stagingPort,
	})

	// Start staging formation on new port
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to start staging formation")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusInternalServerError, StageSpawningStaging, "SpawnError", fmt.Sprintf("Failed to start staging formation: %v", err))
		return
	}

	s.logger.Info().
		Str("id", formationID).
		Int("staging_port", stagingPort).
		Int("staging_pid", proc.PID).
		Msg("Staging formation started, beginning health checks")

	// Emit health check progress
	progress.Emit(ProgressEvent{
		Stage:   StageHealthCheck,
		Message: "Waiting for staging health check...",
	})

	// Health check staging formation
	healthTimeout := time.Duration(s.config.Formations.Deployment.HealthCheck.Timeout) * time.Second
	if healthTimeout == 0 {
		healthTimeout = 300 * time.Second // 5 minutes default
	}
	healthInterval := time.Duration(s.config.Formations.Deployment.HealthCheck.Interval) * time.Second
	if healthInterval == 0 {
		healthInterval = 1 * time.Second // Default 1 second
	}
	
	// Wait a bit for formation to initialize
	stagingDelay := time.Duration(s.config.Formations.Deployment.StagingHealthDelay) * time.Second
	if stagingDelay == 0 {
		stagingDelay = 2 * time.Second // Default 2 seconds
	}
	time.Sleep(stagingDelay)
	
	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthEndpoint := s.config.Formations.Deployment.HealthCheck.Endpoint
	if healthEndpoint == "" {
		healthEndpoint = "/v1/health"
	}
	healthChecker.Endpoint = healthEndpoint

	// Health check with progress callback and crash detection
	healthErr := healthChecker.WaitForHealthyWithPID(stagingPort, formationID, proc.PID, proc.LogFile, func(attempt, maxAttempts int) {
		progress.Emit(ProgressEvent{
			Stage:       StageHealthCheck,
			Message:     fmt.Sprintf("Health check attempt %d/%d...", attempt, maxAttempts),
			Attempt:     &attempt,
			MaxAttempts: &maxAttempts,
		})
	})

	if healthErr != nil {
		// FAILURE PATH: Staging failed health check
		s.logger.Error().
			Err(healthErr).
			Str("id", formationID).
			Int("staging_port", stagingPort).
			Msg("Staging formation failed health check - keeping old version running")

		// Force kill staging process
		if err := s.processManager.ForceKill(stagingProcessID); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to kill staging process (will clean up anyway)")
		}

		// Clean up staging directory
		os.RemoveAll(stagingDir)

		respondErr(http.StatusBadRequest, StageHealthCheck, "HealthCheckFailed",
			fmt.Sprintf("New version failed health check: %v. Old version still running - zero downtime maintained.", healthErr))
		return
	}

	// SUCCESS PATH: Staging is healthy! Switch to new version
	s.logger.Info().
		Str("id", formationID).
		Int("staging_port", stagingPort).
		Msg("Staging formation is healthy - switching to new version")

	// Emit swapping progress
	progress.Emit(ProgressEvent{
		Stage:   StageSwapping,
		Message: "Switching traffic to new version...",
	})

	// Switch active port in registry (atomic operation)
	oldPort, err := s.registry.SwitchToStagingPort(formationID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to switch ports")
		// Kill staging and clean up
		s.processManager.ForceKill(stagingProcessID)
		os.RemoveAll(stagingDir)
		respondErr(http.StatusInternalServerError, StageSwapping, "SwitchError", "Failed to switch ports")
		return
	}

	// Emit stopping old progress
	progress.Emit(ProgressEvent{
		Stage:   StageStoppingOld,
		Message: "Stopping old version...",
	})

	// Stop old formation (with timeout for force kill)
	stopErr := s.processManager.Stop(formationID)
	if stopErr != nil {
		s.logger.Warn().Err(stopErr).Msg("Failed to stop old formation gracefully, force killing")
		// Force kill if graceful stop fails
		if err := s.processManager.ForceKill(formationID); err != nil {
			s.logger.Error().Err(err).Msg("Failed to force kill old formation (continuing anyway)")
		}
	}

	// Move directories: staging → current, old current → previous
	// Remove old previous if exists
	if _, err := os.Stat(previousDir); err == nil {
		os.RemoveAll(previousDir)
	}
	// Move current → previous
	if _, err := os.Stat(currentDir); err == nil {
		os.Rename(currentDir, previousDir)
	}
	// Move staging → current
	if err := os.Rename(stagingDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move staging to current (formation is running, but directory structure inconsistent)")
	}

	// Update version history
	newVersion := history.CurrentVersion + 1

	history.Previous = history.Current
	history.PreviousVersion = history.CurrentVersion
	history.Current = &formation.Version{
		Version:    newVersion,
		DeployedAt: time.Now(),
		BundleHash: newBundleHash,
		BackupPath: "current",
	}
	if history.Previous != nil {
		history.Previous.BackupPath = "previous"
	}
	history.CurrentVersion = newVersion

	if err := history.Save(formationBaseDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save version history")
	}

	// Update registry with new version and process info
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.Version = formationConfig.Version // Semantic version from formation config
		f.ProcessID = proc.PID
		f.Status = "running"
	})

	// Rename staging process to primary process
	// (Update process manager's internal tracking)
	// Note: This is a logical rename - the process continues running

	// Clear staging port from defer cleanup
	stagingPort = 0

	s.logger.Info().
		Str("id", formationID).
		Int("version", newVersion).
		Int("new_port", oldPort).
		Int("old_port", oldPort).
		Int("pid", proc.PID).
		Msg("✓ Zero-downtime deployment successful")

	// Track telemetry
	telemetry.IncrementUpdate(true)

	if wantsSSE {
		// Send complete event via SSE
		progress.Complete(CompleteEvent{
			FormationID:     formationID,
			Port:            oldPort, // Now using the port that was staging
			Status:          "running",
			URL:             fmt.Sprintf("http://localhost:%d/api/%s", s.config.Server.Port, formationID),
			PreviousVersion: fmt.Sprintf("%d", history.PreviousVersion),
			NewVersion:      fmt.Sprintf("%d", newVersion),
		})
	} else {
		response := map[string]interface{}{
			"id":               formationID,
			"status":           "running",
			"version":          newVersion,
			"previous_version": history.PreviousVersion,
			"port":             oldPort, // Now using the port that was staging
			"pid":              proc.PID,
			"message":          "Formation updated with zero downtime",
			"deployment_type":  "blue-green",
		}

		RespondSuccess(w, response)
	}
}
