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

	"github.com/muxi-ai/server/pkg/config"
	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/muxi-ai/server/pkg/runtime"
	"github.com/muxi-ai/server/pkg/telemetry"
)

// DeployResponse represents the response for a deployed formation
type DeployResponse struct {
	FormationID string `json:"formation_id"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	HealthURL   string `json:"health_url"`
	PID         int    `json:"pid"`
}

// HandleDeploy handles POST /rpc/formations
// Deploys a formation from a gzipped tarball bundle containing formation.afs (or .yaml/.yml)
func (s *Server) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// Only accept gzipped bundles
	if !strings.Contains(contentType, "application/gzip") &&
		!strings.Contains(contentType, "application/x-gzip") &&
		!strings.Contains(contentType, "application/octet-stream") {
		RespondError(w, http.StatusBadRequest,
			"Invalid Content-Type. Expected application/gzip with a formation bundle (tar.gz)")
		return
	}

	s.handleBundleDeploy(w, r)
}

// handleBundleDeploy handles formation bundle (gzipped tarball) deployment
func (s *Server) handleBundleDeploy(w http.ResponseWriter, r *http.Request) {
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

	// === Early validation from headers (before reading body) ===

	// Get and validate X-Formation-ID header
	headerFormationID := r.Header.Get("X-Formation-ID")
	if headerFormationID == "" {
		respondErr(http.StatusBadRequest, StageValidating, "MissingHeader",
			"Missing X-Formation-ID header. Provide the formation ID for early conflict detection.")
		return
	}

	// Validate formation ID format
	if err := registry.ValidateFormationID(headerFormationID); err != nil {
		respondErr(http.StatusBadRequest, StageValidating, "InvalidFormationID",
			fmt.Sprintf("Invalid X-Formation-ID: %v", err))
		return
	}

	// Check if formation already exists (early conflict detection)
	if _, err := s.registry.Get(headerFormationID); err == nil {
		s.logger.Warn().Str("id", headerFormationID).Msg("Formation already exists (early check)")
		respondErr(http.StatusConflict, StageValidating, "FormationExists",
			fmt.Sprintf("Formation '%s' already exists. Use PUT to update.", headerFormationID))
		return
	}

	// Optional: X-Formation-Version header (defaults to "1.0.0")
	headerVersion := r.Header.Get("X-Formation-Version")
	if headerVersion == "" {
		headerVersion = "1.0.0"
	}

	s.logger.Info().
		Str("formation_id", headerFormationID).
		Str("version", headerVersion).
		Msg("Deploying formation from bundle")

	// === Now proceed with body processing ===

	// Create temporary file for uploaded bundle
	tmpFile, err := os.CreateTemp("", "formation-*.tar.gz")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create temp file")
		RespondError(w, http.StatusInternalServerError, "Failed to create temp file")
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy uploaded data to temp file
	if _, err := io.Copy(tmpFile, r.Body); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save uploaded bundle")
		RespondError(w, http.StatusInternalServerError, "Failed to save bundle")
		return
	}
	tmpFile.Close()

	// Now that body is fully read, we can initialize SSE streaming
	initSSE()

	// Emit extracting progress
	progress.Emit(ProgressEvent{
		Stage:   StageExtracting,
		Message: "Extracting bundle...",
	})

	// Create temporary extraction directory
	extractDir, err := os.MkdirTemp("", "formation-extract-*")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create extraction directory")
		respondErr(http.StatusInternalServerError, StageExtracting, "ExtractDirError", "Failed to create extraction directory")
		return
	}
	defer os.RemoveAll(extractDir)

	// Extract bundle
	formationDir, err := formation.ExtractBundle(tmpFile.Name(), extractDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to extract bundle")
		respondErr(http.StatusBadRequest, StageExtracting, "ExtractError", fmt.Sprintf("Failed to extract bundle: %v", err))
		return
	}

	// Read bundle data for hash computation
	bundleData, _ := os.ReadFile(tmpFile.Name())

	// Deploy from the extracted directory
	s.deployNewFromDirectory(w, headerFormationID, headerVersion, formationDir, bundleData, wantsSSE, progress)
}

// deployNewFromDirectory deploys a new formation from a source directory.
// This is the core deployment logic shared between HTTP bundle deploy and draft deploy.
// The sourceDir will be MOVED to current/ on success.
// bundleData is optional - used for computing bundle hash for version history.
func (s *Server) deployNewFromDirectory(
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

	// Find and parse formation config (formation.afs/yaml/yml)
	formationConfigPath, err := formation.FindFormationFile(sourceDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to find formation config")
		respondErr(http.StatusBadRequest, StageValidating, "ParseError", fmt.Sprintf("Failed to find formation config: %v", err))
		return
	}
	formationConfig, err := formation.ParseFormation(formationConfigPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation config")
		respondErr(http.StatusBadRequest, StageValidating, "ParseError", fmt.Sprintf("Failed to parse formation config: %v", err))
		return
	}

	configFormationID := formationConfig.ID
	s.logger.Info().
		Str("id", configFormationID).
		Str("name", formationConfig.Name).
		Str("version", formationConfig.Version).
		Msg("Parsed formation bundle")

	// Validate secrets references
	if err := formation.ValidateSecrets(sourceDir); err != nil {
		s.logger.Error().Err(err).Msg("Secrets validation failed")
		respondErr(http.StatusBadRequest, StageValidating, "SecretsError", err.Error())
		return
	}

	// Verify config's formation ID matches the expected ID
	if configFormationID != formationID {
		s.logger.Warn().
			Str("expected_id", formationID).
			Str("config_id", configFormationID).
			Msg("Formation ID mismatch")
		respondErr(http.StatusBadRequest, StageValidating, "IDMismatch",
			fmt.Sprintf("Formation ID mismatch: expected '%s' but config contains '%s'",
				formationID, configFormationID))
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
		respondErr(http.StatusBadRequest, StageValidating, "VersionMismatch",
			fmt.Sprintf("Formation version mismatch: expected '%s' but config contains '%s'",
				expectedVersion, configVersion))
		return
	}

	// Inject server metadata into formation config for telemetry
	if err := formation.InjectMetadata(sourceDir, s.config.ServerID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to inject metadata (continuing anyway)")
	} else {
		s.logger.Debug().
			Str("server_id", s.config.ServerID).
			Msg("Injected server metadata into formation config")
	}

	// Allocate port
	port, err := s.registry.AllocatePort(formationID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to allocate port")
		respondErr(http.StatusInternalServerError, StageValidating, "PortAllocationError", fmt.Sprintf("Failed to allocate port: %v", err))
		return
	}

	// Move formation to permanent location
	muxiDir, err := getMuxiDir()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
		s.registry.ReleasePort(port)
		respondErr(http.StatusInternalServerError, StageValidating, "DirectoryError", "Failed to get MUXI directory")
		return
	}

	formationBaseDir := filepath.Join(muxiDir, "formations", formationID)
	currentDir := filepath.Join(formationBaseDir, "current")

	if err := os.MkdirAll(formationBaseDir, 0755); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create formations directory")
		s.registry.ReleasePort(port)
		respondErr(http.StatusInternalServerError, StageValidating, "DirectoryError", "Failed to create formations directory")
		return
	}

	// Remove existing current directory if it exists (leftover from failed deploy)
	if _, err := os.Stat(currentDir); err == nil {
		s.logger.Warn().
			Str("path", currentDir).
			Msg("Removing leftover formation directory from previous failed deploy")
		if err := os.RemoveAll(currentDir); err != nil {
			s.logger.Error().Err(err).Msg("Failed to remove existing formation directory")
			s.registry.ReleasePort(port)
			respondErr(http.StatusInternalServerError, StageValidating, "CleanupError", "Failed to clean up existing formation directory")
			return
		}
	}

	// Move source directory to current/
	if err := os.Rename(sourceDir, currentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move formation to permanent location")
		s.registry.ReleasePort(port)
		respondErr(http.StatusInternalServerError, StageValidating, "MoveError", "Failed to move formation")
		return
	}

	// Create initial version history
	bundleHash := formation.ComputeBundleHash(bundleData)
	versionHistory := &formation.VersionHistory{
		CurrentVersion: 1,
		Current: &formation.Version{
			Version:    1,
			DeployedAt: time.Now(),
			BundleHash: bundleHash,
			BackupPath: "current",
		},
	}
	if err := versionHistory.Save(formationBaseDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save version history")
		// Don't fail deploy for this - it's not critical
	}

	// Use currentDir as the working directory for the formation
	permanentDir := currentDir

	// Get environment variables from formation
	// Use 0.0.0.0 for macOS/Windows/containers, 127.0.0.1 for native Linux
	bindHost := getBindHost(s.config.Formations.BindHost)
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(port, serverURL, bindHost)

	// Prepare spawn configuration
	spawnConfig := process.SpawnConfig{
		ID:                     formationID,
		Name:                   formationConfig.Name,
		Command:                formationConfig.GetDefaultCommand(),
		Args:                   formationConfig.GetDefaultArgs(),
		Port:                   port,
		WorkDir:                permanentDir,
		Env:                    envVars,
		AutoRestart:            s.config.Formations.AutoRestart,
		RuntimeType:            "native", // Default to native execution
		SkipInitialHealthCheck: true,     // Deploy does its own health check with progress
		TruncateLogs:           true,     // Fresh logs for new deploy
	}

	// Handle runtime resolution if specified in formation config
	if formationConfig.MuxiRuntime != "" {
		// Emit resolving runtime progress
		progress.Emit(ProgressEvent{
			Stage:   StageResolvingRuntime,
			Message: "Resolving runtime version...",
		})

		s.logger.Info().
			Str("id", formationID).
			Str("runtime_constraint", formationConfig.MuxiRuntime).
			Msg("Resolving runtime version")

		// Initialize runtime components
		muxiDir, err := getMuxiDir()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
			s.registry.ReleasePort(port)
			os.RemoveAll(formationBaseDir)
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "DirectoryError", "Failed to get MUXI directory")
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			s.logger.Error().Err(err).Msg("Failed to create runtimes directory")
			s.registry.ReleasePort(port)
			os.RemoveAll(formationBaseDir)
			respondErr(http.StatusInternalServerError, StageResolvingRuntime, "DirectoryError", "Failed to create runtimes directory")
			return
		}

		// Create runtime registry
		runtimeRegistryPath := filepath.Join(runtimesDir, "registry.json")
		runtimeRegistry := runtime.NewRegistry(runtimeRegistryPath)
		if err := runtimeRegistry.Load(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to load runtime registry (continuing with empty registry)")
		}

		// Create resolver with available versions
		availableVersions := runtimeRegistry.List()
		resolver := runtime.NewResolver(availableVersions, runtimesDir)

		// Resolve version constraint
		resolvedVersion, err := resolver.Resolve(formationConfig.MuxiRuntime)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("constraint", formationConfig.MuxiRuntime).
				Msg("Failed to resolve runtime version")
			s.registry.ReleasePort(port)
			os.RemoveAll(formationBaseDir)
			respondErr(http.StatusBadRequest, StageResolvingRuntime, "ResolveError", fmt.Sprintf("Failed to resolve runtime version: %v", err))
			return
		}

		s.logger.Info().
			Str("constraint", formationConfig.MuxiRuntime).
			Str("resolved", resolvedVersion).
			Msg("Resolved runtime version")

		// Emit resolved version progress
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

		// Ensure SIF exists (download if missing)
		// EnsureSIF returns the actual resolved version (important when input was "latest")
		sifPath, actualVersion, sifDownloaded, err := downloader.EnsureSIF(resolvedVersion)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("version", resolvedVersion).
				Msg("Failed to ensure SIF file")
			s.registry.ReleasePort(port)
			os.RemoveAll(formationBaseDir)
			respondErr(http.StatusInternalServerError, StageDownloadingSIF, "DownloadError", fmt.Sprintf("Failed to download runtime: %v", err))
			return
		}

		// Emit progress only if we actually downloaded
		if sifDownloaded {
			progress.Emit(ProgressEvent{
				Stage:   StageDownloadingSIF,
				Message: "Downloaded runtime image",
			})
		}

		// Also ensure runtime-runner is available (macOS/Windows)
		runnerPulled, err := downloader.EnsureRuntimeRunner()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to ensure runtime-runner")
			s.registry.ReleasePort(port)
			os.RemoveAll(formationBaseDir)
			respondErr(http.StatusInternalServerError, StagePullingRunner, "PullError", fmt.Sprintf("Failed to pull runtime-runner: %v", err))
			return
		}

		// Emit progress only if we actually pulled (macOS/Windows only)
		if goruntime.GOOS != "linux" && runnerPulled {
			progress.Emit(ProgressEvent{
				Stage:   StagePullingRunner,
				Message: "Pulled runtime runner",
			})
		}

		// Update spawn config for Singularity execution
		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath

		// For Singularity/Docker, we run: python -m muxi.runtime.utils.run_formation /formation --port PORT --host HOST
		// The formation directory is mounted as /formation inside the container
		// The runtime will find formation.afs, formation.yaml, or formation.yml automatically
		// bindHost was already set earlier based on platform
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.runtime.utils.run_formation",
			"/formation",
			"--port", fmt.Sprintf("%d", port),
			"--host", bindHost,
		}

		s.logger.Info().
			Str("id", formationID).
			Str("runtime_version", actualVersion).
			Str("sif_path", sifPath).
			Strs("args", spawnConfig.Args).
			Msg("Using Singularity runtime")

		// Register runtime in registry if not already present
		if !runtimeRegistry.Exists(actualVersion) {
			fileInfo, _ := os.Stat(sifPath)
			var fileSize int64
			if fileInfo != nil {
				fileSize = fileInfo.Size()
			}
			runtimeRegistry.Add(&runtime.RuntimeInfo{
				Version:      actualVersion,
				Path:         sifPath,
				Size:         fileSize,
				DownloadedAt: time.Now(),
				Formations:   []string{},
			})
		}

		// Add formation reference to runtime registry
		if err := runtimeRegistry.AddFormation(actualVersion, formationID); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to add formation reference to runtime registry")
		}
		if err := runtimeRegistry.Save(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to save runtime registry")
		}
	}

	// Emit spawning progress
	progress.Emit(ProgressEvent{
		Stage:   StageSpawning,
		Message: "Starting formation. It might take 1-2 minutes.",
	})

	// Spawn process with environment variables
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to spawn process")
		s.registry.ReleasePort(port)
		os.RemoveAll(formationBaseDir)
		respondErr(http.StatusInternalServerError, StageSpawning, "SpawnError", fmt.Sprintf("Failed to spawn process: %v", err))
		return
	}

	// Register in registry
	reg := registry.FromProcess(proc, port)
	reg.Version = formationConfig.Version
	if err := s.registry.Register(reg); err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to register formation")
		s.processManager.Stop(formationID)
		s.registry.ReleasePort(port)
		os.RemoveAll(formationBaseDir)
		respondErr(http.StatusInternalServerError, StageSpawning, "RegisterError", fmt.Sprintf("Failed to register formation: %v", err))
		return
	}

	// Emit health check progress
	progress.Emit(ProgressEvent{
		Stage:   StageHealthCheck,
		Message: "Waiting for formation health check...",
	})

	// Wait for formation to become healthy
	healthTimeout := time.Duration(s.config.Formations.Deployment.HealthCheck.Timeout) * time.Second
	if healthTimeout == 0 {
		healthTimeout = 300 * time.Second // 5 minutes default
	}
	healthInterval := time.Duration(s.config.Formations.Deployment.HealthCheck.Interval) * time.Second
	if healthInterval == 0 {
		healthInterval = 1 * time.Second
	}

	// Wait a bit for formation to initialize
	time.Sleep(2 * time.Second)

	healthChecker := process.NewHealthChecker(healthTimeout, healthInterval)
	healthEndpoint := s.config.Formations.Deployment.HealthCheck.Endpoint
	if healthEndpoint == "" {
		healthEndpoint = "/v1/health"
	}
	healthChecker.Endpoint = healthEndpoint

	// Health check with progress callback and crash detection
	healthErr := healthChecker.WaitForHealthyWithPID(port, formationID, proc.PID, proc.LogFile, func(attempt, maxAttempts int) {
		progress.Emit(ProgressEvent{
			Stage:       StageHealthCheck,
			Message:     fmt.Sprintf("Health check attempt %d/%d...", attempt, maxAttempts),
			Attempt:     &attempt,
			MaxAttempts: &maxAttempts,
		})
	})

	if healthErr != nil {
		s.logger.Error().Err(healthErr).Str("id", formationID).Msg("Formation failed health check")
		// Clean up: force kill process and unregister
		if err := s.processManager.ForceKill(formationID); err != nil {
			s.logger.Warn().Err(err).Str("id", formationID).Msg("Failed to force kill process during cleanup (may already be dead)")
		}
		s.registry.Unregister(formationID)
		s.registry.ReleasePort(port)
		os.RemoveAll(formationBaseDir)
		respondErr(http.StatusBadRequest, StageHealthCheck, "HealthCheckFailed",
			fmt.Sprintf("Formation failed health check after %v: %v", healthTimeout, healthErr))
		return
	}

	// Update status to running after successful health check
	proc.SetStatus(process.StatusRunning)
	s.registry.UpdateHealthCheck(formationID, true)
	s.registry.Update(formationID, func(f *registry.Formation) {
		f.Status = "running"
	})

	s.logger.Info().
		Str("id", formationID).
		Int("port", port).
		Int("pid", proc.PID).
		Str("location", permanentDir).
		Msg("Formation deployed and healthy")

	// Track telemetry
	telemetry.IncrementDeploy(true)
	telemetry.SetActiveFormations(s.registry.Count())

	// Build response
	response := DeployResponse{
		FormationID: formationID,
		Port:        port,
		Status:      string(proc.Status),
		URL:         fmt.Sprintf("http://localhost:%d/api/%s", s.config.Server.Port, formationID),
		HealthURL:   fmt.Sprintf("http://localhost:%d/v1/health", port),
		PID:         proc.PID,
	}

	if wantsSSE {
		// Send complete event via SSE
		progress.Complete(CompleteEvent{
			FormationID: response.FormationID,
			Port:        response.Port,
			Status:      response.Status,
			URL:         response.URL,
			HealthURL:   response.HealthURL,
			PID:         response.PID,
		})
	} else {
		RespondCreated(w, response)
	}
}

// getMuxiDir returns the MUXI data directory path
func getMuxiDir() (string, error) {
	return config.GetDataDir()
}
