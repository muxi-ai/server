package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/muxi-ai/server/pkg/formation"
	"github.com/muxi-ai/server/pkg/process"
	"github.com/muxi-ai/server/pkg/registry"
	"github.com/muxi-ai/server/pkg/runtime"
)

// DeployRequest represents the request body for deploying a formation
type DeployRequest struct {
	ID      string   `json:"id"`      // Optional custom ID
	Command string   `json:"command"` // Executable (e.g., "python3")
	Args    []string `json:"args"`    // Arguments (e.g., ["test/dummy_app.py", "--port", "8001"])
}

// DeployResponse represents the response for a deployed formation
type DeployResponse struct {
	FormationID string `json:"formation_id"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	HealthURL   string `json:"health_url"`
	PID         int    `json:"pid"`
}

// HandleDeploy handles POST /formations/deploy
// Supports both JSON (legacy) and gzipped formation bundles
func (s *Server) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// Route based on content type
	if strings.Contains(contentType, "application/gzip") ||
		strings.Contains(contentType, "application/x-gzip") ||
		strings.Contains(contentType, "application/octet-stream") {
		s.handleBundleDeploy(w, r)
		return
	}

	// Default to JSON for backward compatibility
	s.handleJSONDeploy(w, r)
}

// handleJSONDeploy handles the original JSON-based deployment
func (s *Server) handleJSONDeploy(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to parse deploy request")
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.Command == "" {
		RespondError(w, http.StatusBadRequest, "command is required")
		return
	}

	// Generate ID if not provided
	if req.ID == "" {
		req.ID = generateFormationID()
	}
	
	// Validate formation ID
	if err := registry.ValidateFormationID(req.ID); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.logger.Info().
		Str("id", req.ID).
		Str("command", req.Command).
		Strs("args", req.Args).
		Msg("Deploying formation")

	// Allocate port
	port, err := s.registry.AllocatePort(req.ID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to allocate port")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to allocate port: %v", err))
		return
	}

	// Spawn process
	proc, err := s.processManager.Start(process.SpawnConfig{
		ID:          req.ID,
		Name:        req.ID,
		Command:     req.Command,
		Args:        req.Args,
		Port:        port,
		AutoRestart: s.config.Formations.AutoRestart,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("id", req.ID).Msg("Failed to spawn process")
		s.registry.ReleasePort(port)
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to spawn process: %v", err))
		return
	}

	// Register in registry
	formation := registry.FromProcess(proc, port)
	if err := s.registry.Register(formation); err != nil {
		s.logger.Error().Err(err).Str("id", req.ID).Msg("Failed to register formation")
		// Try to stop the process
		s.processManager.Stop(req.ID)
		s.registry.ReleasePort(port)
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to register formation: %v", err))
		return
	}

	s.logger.Info().
		Str("id", req.ID).
		Int("port", port).
		Int("pid", proc.PID).
		Msg("Formation deployed successfully")

	// Build response
	response := DeployResponse{
		FormationID: req.ID,
		Port:        port,
		Status:      string(proc.Status),
		URL:         fmt.Sprintf("http://localhost:%d", s.config.Server.Port),
		HealthURL:   fmt.Sprintf("http://localhost:%d/health", port),
		PID:         proc.PID,
	}

	RespondCreated(w, response)
}

// handleBundleDeploy handles formation bundle (gzipped tarball) deployment
func (s *Server) handleBundleDeploy(w http.ResponseWriter, r *http.Request) {
	s.logger.Info().Msg("Deploying formation from bundle")

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

	// Create temporary extraction directory
	extractDir, err := os.MkdirTemp("", "formation-extract-*")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create extraction directory")
		RespondError(w, http.StatusInternalServerError, "Failed to create extraction directory")
		return
	}
	defer os.RemoveAll(extractDir)

	// Extract bundle
	formationDir, err := formation.ExtractBundle(tmpFile.Name(), extractDir)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to extract bundle")
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to extract bundle: %v", err))
		return
	}

	// Parse formation.yaml
	formationYAMLPath := filepath.Join(formationDir, "formation.yaml")
	formationConfig, err := formation.ParseFormationYAML(formationYAMLPath)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse formation.yaml")
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse formation.yaml: %v", err))
		return
	}

	formationID := formationConfig.ID
	s.logger.Info().
		Str("id", formationID).
		Str("name", formationConfig.Name).
		Str("version", formationConfig.Version).
		Msg("Parsed formation bundle")

	// Inject server metadata into formation.yaml for telemetry
	if err := formation.InjectMetadata(formationDir, s.config.ServerID); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to inject metadata (continuing anyway)")
		// Don't fail deployment if metadata injection fails
	} else {
		s.logger.Debug().
			Str("server_id", s.config.ServerID).
			Msg("Injected server metadata into formation.yaml")
	}

	// Check if formation already exists
	if _, err := s.registry.Get(formationID); err == nil {
		s.logger.Warn().Str("id", formationID).Msg("Formation already exists")
		RespondError(w, http.StatusConflict, fmt.Sprintf("Formation '%s' already exists", formationID))
		return
	}

	// Allocate port
	port, err := s.registry.AllocatePort(formationID)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to allocate port")
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to allocate port: %v", err))
		return
	}

	// Move formation to permanent location
	muxiDir, err := getMuxiDir()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
		s.registry.ReleasePort(port)
		RespondError(w, http.StatusInternalServerError, "Failed to get MUXI directory")
		return
	}

	permanentDir := filepath.Join(muxiDir, "formations", formationID)
	if err := os.MkdirAll(filepath.Dir(permanentDir), 0755); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create formations directory")
		s.registry.ReleasePort(port)
		RespondError(w, http.StatusInternalServerError, "Failed to create formations directory")
		return
	}

	// Move extracted directory to permanent location
	if err := os.Rename(formationDir, permanentDir); err != nil {
		s.logger.Error().Err(err).Msg("Failed to move formation to permanent location")
		s.registry.ReleasePort(port)
		RespondError(w, http.StatusInternalServerError, "Failed to move formation")
		return
	}

	// Get environment variables from formation
	// On macOS/Windows (Docker), use 0.0.0.0 so formation is accessible from host
	// On Linux (Singularity), use 127.0.0.1 for security
	bindHost := s.config.Formations.BindHost
	if goruntime.GOOS == "darwin" || goruntime.GOOS == "windows" {
		bindHost = "0.0.0.0"
	}
	serverURL := fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
	envVars := formationConfig.GetEnvironmentVars(port, serverURL, bindHost)

	// Prepare spawn configuration
	spawnConfig := process.SpawnConfig{
		ID:          formationID,
		Name:        formationConfig.Name,
		Command:     formationConfig.GetDefaultCommand(),
		Args:        formationConfig.GetDefaultArgs(),
		Port:        port,
		WorkDir:     permanentDir,
		Env:         envVars,
		AutoRestart: s.config.Formations.AutoRestart,
		RuntimeType: "native", // Default to native execution
	}

	// Handle runtime resolution if specified in formation.yaml
	if formationConfig.Runtime != "" {
		s.logger.Info().
			Str("id", formationID).
			Str("runtime_constraint", formationConfig.Runtime).
			Msg("Resolving runtime version")

		// Initialize runtime components
		muxiDir, err := getMuxiDir()
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to get MUXI directory")
			s.registry.ReleasePort(port)
			os.RemoveAll(permanentDir)
			RespondError(w, http.StatusInternalServerError, "Failed to get MUXI directory")
			return
		}

		runtimesDir := filepath.Join(muxiDir, "runtimes")
		if err := os.MkdirAll(runtimesDir, 0755); err != nil {
			s.logger.Error().Err(err).Msg("Failed to create runtimes directory")
			s.registry.ReleasePort(port)
			os.RemoveAll(permanentDir)
			RespondError(w, http.StatusInternalServerError, "Failed to create runtimes directory")
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
		resolvedVersion, err := resolver.Resolve(formationConfig.Runtime)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("constraint", formationConfig.Runtime).
				Msg("Failed to resolve runtime version")
			s.registry.ReleasePort(port)
			os.RemoveAll(permanentDir)
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to resolve runtime version: %v", err))
			return
		}

		s.logger.Info().
			Str("constraint", formationConfig.Runtime).
			Str("resolved", resolvedVersion).
			Msg("Resolved runtime version")

		// Get SIF path
		sifPath := resolver.GetSIFPath(resolvedVersion)

		// Check if SIF exists
		if _, err := os.Stat(sifPath); os.IsNotExist(err) {
			s.logger.Error().
				Str("version", resolvedVersion).
				Str("path", sifPath).
				Msg("Runtime SIF file not found")
			s.registry.ReleasePort(port)
			os.RemoveAll(permanentDir)
			RespondError(w, http.StatusNotFound,
				fmt.Sprintf("Runtime %s not found at %s. Please ensure runtime SIF is installed.", resolvedVersion, sifPath))
			return
		}

		// Update spawn config for Singularity execution
		spawnConfig.RuntimeType = "singularity"
		spawnConfig.SIFPath = sifPath
		
		// For Singularity/Docker, we run: python -m muxi.utils.run_formation /formation/formation.yaml --port PORT --host HOST
		// The formation directory is mounted as /formation inside the container
		// bindHost was already set earlier based on platform
		spawnConfig.Command = "python"
		spawnConfig.Args = []string{
			"-m", "muxi.utils.run_formation",
			"/formation/formation.yaml",
			"--port", fmt.Sprintf("%d", port),
			"--host", bindHost,
		}

		s.logger.Info().
			Str("id", formationID).
			Str("runtime_version", resolvedVersion).
			Str("sif_path", sifPath).
			Strs("args", spawnConfig.Args).
			Msg("Using Singularity runtime")

		// Add formation reference to runtime registry
		if err := runtimeRegistry.AddFormation(resolvedVersion, formationID); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to add formation reference to runtime registry")
		}
		if err := runtimeRegistry.Save(); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to save runtime registry")
		}
	}

	// Spawn process with environment variables
	proc, err := s.processManager.Start(spawnConfig)
	if err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to spawn process")
		s.registry.ReleasePort(port)
		os.RemoveAll(permanentDir)
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to spawn process: %v", err))
		return
	}

	// Register in registry
	reg := registry.FromProcess(proc, port)
	if err := s.registry.Register(reg); err != nil {
		s.logger.Error().Err(err).Str("id", formationID).Msg("Failed to register formation")
		s.processManager.Stop(formationID)
		s.registry.ReleasePort(port)
		os.RemoveAll(permanentDir)
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to register formation: %v", err))
		return
	}

	s.logger.Info().
		Str("id", formationID).
		Int("port", port).
		Int("pid", proc.PID).
		Str("location", permanentDir).
		Msg("Formation deployed successfully from bundle")

	// Build response
	response := DeployResponse{
		FormationID: formationID,
		Port:        port,
		Status:      string(proc.Status),
		URL:         fmt.Sprintf("http://localhost:%d/v1/%s", s.config.Server.Port, formationID),
		HealthURL:   fmt.Sprintf("http://localhost:%d/health", port),
		PID:         proc.PID,
	}

	RespondCreated(w, response)
}

// getMuxiDir returns the MUXI directory path
func getMuxiDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".muxi", "server"), nil
}

// generateFormationID generates a unique formation ID
func generateFormationID() string {
	// Simple implementation: timestamp-based
	// In production, use UUIDs or nanoids
	return fmt.Sprintf("formation-%d", time.Now().Unix())
}
