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

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// preserveTuningFiles copies runtime-owned tuning state (MUXI.md in either
// casing the runtime accepts, plus PENDING-MUXI.md) from currentDir into
// stagingDir, overwriting any copies shipped in the bundle: the live files
// are written by the runtime's self-improvement pass and are always newer
// than whatever an operator bundled. Missing files are skipped. Returns the
// names copied and any per-file copy failures.
func preserveTuningFiles(currentDir, stagingDir string) (preserved []string, failures map[string]error) {
	// Match on-disk names exactly via ReadDir: a plain os.Stat per candidate
	// would double-copy on case-insensitive filesystems (macOS APFS), where
	// stat("muxi.md") also matches "MUXI.md".
	wanted := map[string]bool{"MUXI.md": true, "muxi.md": true, "PENDING-MUXI.md": true}
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !wanted[name] {
			continue
		}
		if err := copyFile(filepath.Join(currentDir, name), filepath.Join(stagingDir, name)); err != nil {
			if failures == nil {
				failures = make(map[string]error)
			}
			failures[name] = err
			continue
		}
		preserved = append(preserved, name)
	}
	return preserved, failures
}

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
	_, err := s.registry.Get(formationID)
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

	// Extract new version to a temp directory under <DataDir>/tmp so the
	// subsequent rename into formations/<id>/staging is same-filesystem
	// and won't fail with EXDEV. See getServerTmpDir for full rationale.
	serverTmpDir, err := getServerTmpDir()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to resolve server tmp dir")
		if wantsSSE && sseInitialized {
			progress.Error(ErrorEvent{
				Error:   "ExtractDirError",
				Message: "Failed to resolve server tmp dir",
				Stage:   StageExtracting,
			})
		} else {
			RespondError(w, http.StatusInternalServerError, "Failed to resolve server tmp dir")
		}
		return
	}
	extractDir, err := os.MkdirTemp(serverTmpDir, "formation-extract-*")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create extraction directory")
		if wantsSSE && sseInitialized {
			progress.Error(ErrorEvent{
				Error:   "ExtractDirError",
				Message: "Failed to create extraction directory",
				Stage:   StageExtracting,
			})
		} else {
			RespondError(w, http.StatusInternalServerError, "Failed to create extraction directory")
		}
		return
	}
	defer os.RemoveAll(extractDir)

	newFormationDir, err := formation.ExtractBundle(tmpFile.Name(), extractDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to extract bundle")
		if wantsSSE && sseInitialized {
			progress.Error(ErrorEvent{
				Error:   "ExtractError",
				Message: fmt.Sprintf("Failed to extract bundle: %v", err),
				Stage:   StageExtracting,
			})
		} else {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to extract bundle: %v", err))
		}
		return
	}

	// Delegate to updateFromDirectory
	s.updateFromDirectory(w, formationID, headerVersion, newFormationDir, bundleData, wantsSSE, progress)
}

// updateFromDirectory updates an existing formation from a source directory.
// This is the core update logic shared between HTTP bundle update and draft deploy.
// The sourceDir will be MOVED to staging/ during blue-green deployment.
// bundleData is optional - used for computing bundle hash for version history.
func (s *Server) updateFromDirectory(
	w http.ResponseWriter,
	formationID string,
	expectedVersion string,
	sourceDir string,
	bundleData []byte,
	wantsSSE bool,
	progress *ProgressEmitter,
) {
	// Helper to respond with error (handles both SSE and regular responses)
	respondErr := func(status int, stage DeployStage, errType, message string) {
		if wantsSSE && progress != nil && progress.sse != nil {
			progress.Error(ErrorEvent{
				Error:   errType,
				Message: message,
				Stage:   stage,
			})
		} else {
			RespondError(w, status, message)
		}
	}

	// Get existing formation for port info
	existingFormation, err := s.registry.Get(formationID)
	if err != nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation not found")
		respondErr(http.StatusNotFound, StageValidating, "NotFound", "Formation not found")
		return
	}

	// Get formation base directory
	muxiDir, err := getMuxiDir()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
		respondErr(http.StatusInternalServerError, StageValidating, "DirectoryError", "Failed to get MUXI directory")
		return
	}
	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)

	// Load version history
	history, err := formation.LoadVersionHistory(formationBaseDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load version history")
		respondErr(http.StatusInternalServerError, StageValidating, "HistoryError", "Failed to load version history")
		return
	}

	// Check if bundle is identical to current version (no change)
	newBundleHash := formation.ComputeBundleHash(bundleData)
	if history.Current != nil && history.Current.BundleHash == newBundleHash {
		s.logger.Warn().
			Str("id", formationID).
			Str("hash", newBundleHash).
			Msg("Source is identical to current version")
		respondErr(http.StatusConflict, StageValidating, "NoChange", "Source is identical to current version - nothing to update")
		return
	}

	// Allocate NEW port for staging version (blue-green deployment)
	stagingPort, err := s.registry.AllocatePort(formationID + "-staging")
	if err != nil {
		s.logger.Error().Err(err).Msg("No available ports for staging")
		respondErr(http.StatusInsufficientStorage, StageValidating, "PortAllocationError", "No available ports for staging deployment")
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
		respondErr(http.StatusInternalServerError, StageValidating, "RegistryError", "Failed to set staging port")
		return
	}

	// Set up directories
	currentDir := filepath.Join(formationBaseDir, "current")
	previousDir := filepath.Join(formationBaseDir, "previous")
	stagingDir := filepath.Join(formationBaseDir, "staging")

	// Ensure the formation base dir exists. This guards against the
	// registry-without-dir state we hit during the 0.20260514.0 deploy:
	// `muxi server delete cicd` wiped /var/lib/muxi/formations/cicd on
	// disk but auto-save raced and left the registry entry pointing at
	// the now-missing dir, so the subsequent deploy was routed through
	// the update path (which previously assumed the dir existed) and
	// the rename below failed with ENOENT, masked as "Failed to move
	// source to staging". A trivial MkdirAll closes that hole.
	if err := os.MkdirAll(formationBaseDir, 0755); err != nil {
		s.logger.Error().Err(err).Msg("Failed to ensure formation base dir")
		respondErr(http.StatusInternalServerError, StageValidating, "DirectoryError", "Failed to ensure formation base directory")
		return
	}

	// Remove old staging if exists
	if _, err := os.Stat(stagingDir); err == nil {
		if err := os.RemoveAll(stagingDir); err != nil {
			s.logger.Error().Err(err).Msg("Failed to remove old staging")
			respondErr(http.StatusInternalServerError, StageValidating, "CleanupError", "Failed to clean staging directory")
			return
		}
	}

	// Move source directory to staging/. safeRename falls back to a
	// recursive copy + remove on EXDEV, so an operator with $TMPDIR on
	// a different FS than MUXI_DATA_DIR still gets a working deploy
	// (with a small perf hit) instead of a confusing rename failure.
	if err := safeRename(sourceDir, stagingDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move source to staging")
		respondErr(http.StatusInternalServerError, StageValidating, "MoveError", "Failed to move source to staging")
		return
	}

	// Preserve memory.db from current version if not included in upload
	// This ensures persistent memory state is carried forward across updates
	memoryDBName := "memory.db"
	currentMemoryDB := filepath.Join(currentDir, memoryDBName)
	stagingMemoryDB := filepath.Join(stagingDir, memoryDBName)

	if _, err := os.Stat(stagingMemoryDB); os.IsNotExist(err) {
		// memory.db not in upload, check if current has one to preserve
		if _, err := os.Stat(currentMemoryDB); err == nil {
			if err := copyFile(currentMemoryDB, stagingMemoryDB); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to preserve memory.db (continuing anyway)")
			} else {
				s.logger.Info().
					Str("id", formationID).
					Msg("Preserved memory.db from current version")
			}
		}
	}

	// Preserve runtime-owned tuning state (MUXI.md, PENDING-MUXI.md) so the
	// formation's accumulated learnings survive the version swap. Unlike
	// memory.db this overwrites bundle copies: the live files always win.
	tuningPreserved, tuningFailures := preserveTuningFiles(currentDir, stagingDir)
	for _, name := range tuningPreserved {
		s.logger.Info().
			Str("id", formationID).
			Str("file", name).
			Msg("Preserved tuning file from current version")
	}
	for name, copyErr := range tuningFailures {
		s.logger.Warn().Err(copyErr).
			Str("file", name).
			Msg("Failed to preserve tuning file (continuing anyway)")
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

	// Verify config's version matches the expected version
	configVersion := formationConfig.Version
	if configVersion == "" {
		configVersion = "1.0.0" // Default if not specified
	}
	if configVersion != expectedVersion {
		s.logger.Warn().
			Str("expected_version", expectedVersion).
			Str("config_version", configVersion).
			Msg("Formation version mismatch")
		os.RemoveAll(stagingDir)
		respondErr(http.StatusBadRequest, StageValidating, "VersionMismatch",
			fmt.Sprintf("Formation version mismatch: expected '%s' but config contains '%s'",
				expectedVersion, configVersion))
		return
	}

	// Inject metadata
	serverID, _ := formation.GenerateServerID()
	if err := formation.InjectMetadata(stagingDir, serverID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to inject metadata (continuing anyway)")
	}

	// Get environment variables (use staging port)
	bindHost := getBindHost(s.config.Formations.BindHost)
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(stagingPort, serverURL, bindHost)
	if s.rceManager != nil {
		s.rceManager.InjectEnvVars(envVars)
	}

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

		// Parse muxi_runtime as "<version>[:<variant>]"; see dev.go.
		parsedVersion, variant, err := runtime.ParseMuxiRuntime(formationConfig.MuxiRuntime)
		if err != nil {
			os.RemoveAll(stagingDir)
			respondErr(http.StatusBadRequest, StageResolvingRuntime, "InvalidRuntime", err.Error())
			return
		}

		resolvedVersion, err := resolver.Resolve(parsedVersion)
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

		// Ensure SIF exists (download if missing), routed by variant.
		sifPath, _, sifDownloaded, err := downloader.EnsureSIFForVariant(resolvedVersion, variant)
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

		cacheDir, err := getCacheDir()
		if err != nil {
			os.RemoveAll(stagingDir)
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "ConfigError", err.Error())
			return
		}

		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		spawnConfig.HFCacheDir = cacheDir
		spawnConfig.Variant = variant
		spawnConfig.RuntimeRunnerImage = s.config.Runtime.RuntimeRunnerImage
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
	healthTimeout := resolveHealthTimeout(s.config)
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
