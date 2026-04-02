package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	ServerID   string           `yaml:"server_id"` // Unique server identifier
	Server     ServerConfig     `yaml:"server"`
	Auth       AuthConfig       `yaml:"auth"`
	Formations FormationsConfig `yaml:"formations"`
	Runtime    RuntimeConfig    `yaml:"runtime"`
	RCE        RCEConfig        `yaml:"rce"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// RCEConfig contains Skills RCE service settings
type RCEConfig struct {
	Port      int    `yaml:"port"`       // RCE listen port (default: 7891)
	AuthToken string `yaml:"auth_token"` // Bearer token for RCE authentication
	DataDir   string `yaml:"data_dir"`   // Data directory for RCE (default: derived from GetDataDir)
}

// RuntimeConfig contains runtime download settings
type RuntimeConfig struct {
	// SIF download settings
	SIFBaseURL string `yaml:"sif_base_url"` // Base URL for SIF downloads (default: GitHub releases)

	// Docker runtime-runner settings
	RuntimeRunnerImage string `yaml:"runtime_runner_image"` // Docker image for runtime-runner (default: ghcr.io/muxi-ai/runtime-runner:latest)
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level    string `yaml:"level"`     // Log level: debug, info, warn, error (default: info)
	AuditLog string `yaml:"audit_log"` // Audit log file path (default: logs/audit.log)
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	Enabled            bool   `yaml:"enabled"`             // Enable authentication (default: false for dev)
	Key                string `yaml:"key"`                 // Public key identifier (e.g., muxi_pk_abc123) - 24 chars
	Secret             string `yaml:"secret"`              // Secret key for HMAC (e.g., muxi_sk_xyz789...) - 64 chars
	TimestampTolerance int    `yaml:"timestamp_tolerance"` // Tolerance in seconds (default: 300 = 5 min)
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port"` // HTTP server port (default: 7890)
	Host string `yaml:"host"` // Bind host (default: 0.0.0.0)
}

// FormationsConfig contains formation management settings
type FormationsConfig struct {
	// Runtime settings
	RuntimeType string `yaml:"runtime_type"` // "native", "docker", "singularity" (default: native)

	// Directories (relative to ~/.muxi/server/)
	LogsDir       string `yaml:"logs_dir"`       // Logs directory (default: logs)
	PIDsDir       string `yaml:"pids_dir"`       // PID files directory (default: pids)
	FormationsDir string `yaml:"formations_dir"` // Formations config directory (default: formations)

	// Port allocation
	PortRangeStart int    `yaml:"port_range_start"` // Start of port range (default: 8000)
	PortRangeEnd   int    `yaml:"port_range_end"`   // End of port range (default: 9000)
	BindHost       string `yaml:"bind_host"`        // Host formations bind to (default: 127.0.0.1)
	MaxFormations  int    `yaml:"max_formations"`   // Max formations (default: 100)
	KeepBackups    int    `yaml:"keep_backups"`     // Number of version backups to keep (default: 1)

	// Process management
	AutoRestart  bool `yaml:"auto_restart"`  // Enable auto-restart (default: true)
	MaxRestarts  int  `yaml:"max_restarts"`  // Max restart attempts (default: 10)
	RestartDelay int  `yaml:"restart_delay"` // Delay between restarts in seconds (default: 1)

	// Health checks
	HealthCheckInterval int `yaml:"health_check_interval"` // Health check interval in seconds (default: 30)
	HealthCheckTimeout  int `yaml:"health_check_timeout"`  // Health check timeout in seconds (default: 5)
	StartupHealthDelay  int `yaml:"startup_health_delay"`  // Delay before first health check (default: 2)

	// Zero-downtime deployment
	Deployment DeploymentConfig `yaml:"deployment"` // Zero-downtime deployment settings

	// Log rotation
	LogRotationEnabled bool   `yaml:"log_rotation_enabled"` // Enable log rotation (default: true)
	LogMaxSize         string `yaml:"log_max_size"`         // Max log file size (default: "10M")
	LogMaxFiles        int    `yaml:"log_max_files"`        // Max log files to keep (default: 10)
}

// DeploymentConfig contains zero-downtime deployment settings
type DeploymentConfig struct {
	HealthCheck        HealthCheckConfig `yaml:"health_check"`         // Health check settings
	ForceKillTimeout   int               `yaml:"force_kill_timeout"`   // Seconds to wait before force-killing old version (default: 5)
	StagingHealthDelay int               `yaml:"staging_health_delay"` // Delay before starting health checks on staging (default: 2)
}

// HealthCheckConfig contains health check settings for deployments
type HealthCheckConfig struct {
	Enabled    bool   `yaml:"enabled"`     // Enable health checks during deployment (default: true)
	Endpoint   string `yaml:"endpoint"`    // Health endpoint path (default: "/health")
	Timeout    int    `yaml:"timeout"`     // Total timeout in seconds (default: 30)
	Interval   int    `yaml:"interval"`    // Poll interval in seconds (default: 1)
	MaxRetries int    `yaml:"max_retries"` // Max health check attempts (default: 30)
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 7890,
			Host: "0.0.0.0",
		},
		Auth: AuthConfig{
			Enabled:            false, // Disabled by default for development
			TimestampTolerance: 300,   // 5 minutes
		},
		Formations: FormationsConfig{
			RuntimeType:         "native",
			LogsDir:             "logs",
			PIDsDir:             "pids",
			FormationsDir:       "formations",
			PortRangeStart:      8000,
			PortRangeEnd:        9000,
			BindHost:            "127.0.0.1",
			MaxFormations:       100,
			KeepBackups:         1,
			AutoRestart:         true,
			MaxRestarts:         10,
			RestartDelay:        1,
			HealthCheckInterval: 30,
			HealthCheckTimeout:  5,
			StartupHealthDelay:  2,
			Deployment: DeploymentConfig{
				HealthCheck: HealthCheckConfig{
					Enabled:    true,
					Endpoint:   "/v1/health",
					Timeout:    30,
					Interval:   1,
					MaxRetries: 30,
				},
				ForceKillTimeout:   5,
				StagingHealthDelay: 2,
			},
			LogRotationEnabled: true,
			LogMaxSize:         "10M",
			LogMaxFiles:        10,
		},
		Runtime: RuntimeConfig{
			SIFBaseURL:         "https://github.com/muxi-ai/runtime/releases/download",
			RuntimeRunnerImage: "ghcr.io/muxi-ai/runtime-runner:latest",
		},
		RCE: RCEConfig{
			Port: 7891,
		},
		Logging: LoggingConfig{
			Level:    "info",
			AuditLog: "logs/audit.log",
		},
	}
}

// Load loads configuration from file
// If file doesn't exist, returns default config
func Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist, return defaults
		return DefaultConfig(), nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults
	config := DefaultConfig()

	// Unmarshal YAML (will override defaults with values from file)
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for empty values (handles old configs with missing fields)
	config.applyDefaults()

	return config, nil
}

// applyDefaults fills in default values for empty fields
// This handles old config files that may be missing newer fields
func (c *Config) applyDefaults() {
	defaults := DefaultConfig()

	// Runtime defaults
	if c.Runtime.SIFBaseURL == "" {
		c.Runtime.SIFBaseURL = defaults.Runtime.SIFBaseURL
	}
	if c.Runtime.RuntimeRunnerImage == "" {
		c.Runtime.RuntimeRunnerImage = defaults.Runtime.RuntimeRunnerImage
	}

	// Formations defaults
	if c.Formations.RuntimeType == "" {
		c.Formations.RuntimeType = defaults.Formations.RuntimeType
	}
	if c.Formations.LogsDir == "" {
		c.Formations.LogsDir = defaults.Formations.LogsDir
	}
	if c.Formations.PIDsDir == "" {
		c.Formations.PIDsDir = defaults.Formations.PIDsDir
	}
	if c.Formations.FormationsDir == "" {
		c.Formations.FormationsDir = defaults.Formations.FormationsDir
	}
	if c.Formations.BindHost == "" {
		c.Formations.BindHost = defaults.Formations.BindHost
	}
	if c.Formations.PortRangeStart == 0 {
		c.Formations.PortRangeStart = defaults.Formations.PortRangeStart
	}
	if c.Formations.PortRangeEnd == 0 {
		c.Formations.PortRangeEnd = defaults.Formations.PortRangeEnd
	}

	// RCE defaults
	if c.RCE.Port == 0 {
		c.RCE.Port = defaults.RCE.Port
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = defaults.Logging.Level
	}
	if c.Logging.AuditLog == "" {
		c.Logging.AuditLog = defaults.Logging.AuditLog
	}
}

// Save saves configuration to file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigDir returns the configuration directory
// Priority: MUXI_CONFIG_DIR env var > Platform detection > User home
func GetConfigDir() (string, error) {
	// 1. Environment override (highest priority)
	if dir := os.Getenv("MUXI_CONFIG_DIR"); dir != "" {
		return dir, nil
	}

	// 2. Platform + binary location detection
	exe, err := os.Executable()
	if err == nil {
		// Linux + installed in /usr → system paths
		if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
			return "/etc/muxi/server", nil
		}

		// Windows + installed in Program Files → system paths
		if runtime.GOOS == "windows" && (strings.HasPrefix(exe, "C:\\Program Files") || strings.HasPrefix(exe, "C:\\Program Files (x86)")) {
			return "C:\\ProgramData\\muxi\\server", nil
		}
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
	if runtime.GOOS == "windows" {
		// Windows user-level install: %APPDATA%\muxi\server
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "muxi", "server"), nil
	}

	// Unix/macOS: ~/.muxi/server
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".muxi", "server"), nil
}

// GetDataDir returns the data directory (formations, registry)
// Priority: MUXI_DATA_DIR env var > Platform detection > User home
func GetDataDir() (string, error) {
	// 1. Environment override (highest priority)
	if dir := os.Getenv("MUXI_DATA_DIR"); dir != "" {
		return dir, nil
	}

	// 2. Platform + binary location detection
	exe, err := os.Executable()
	if err == nil {
		// Linux + installed in /usr → system paths
		if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
			return "/var/lib/muxi", nil
		}

		// Windows + installed in Program Files → system paths
		if runtime.GOOS == "windows" && (strings.HasPrefix(exe, "C:\\Program Files") || strings.HasPrefix(exe, "C:\\Program Files (x86)")) {
			return "C:\\ProgramData\\muxi\\data", nil
		}
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
	if runtime.GOOS == "windows" {
		// Windows user-level install: %APPDATA%\muxi\server
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "muxi", "server"), nil
	}

	// Unix/macOS: ~/.muxi/server
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".muxi", "server"), nil
}

// GetLogDir returns the logs directory
// Priority: MUXI_LOG_DIR env var > Platform detection > User home
func GetLogDir() (string, error) {
	// 1. Environment override (highest priority)
	if dir := os.Getenv("MUXI_LOG_DIR"); dir != "" {
		return dir, nil
	}

	// 2. Platform + binary location detection
	exe, err := os.Executable()
	if err == nil {
		// Linux + installed in /usr → system paths
		if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
			return "/var/log/muxi", nil
		}

		// Windows + installed in Program Files → system paths
		if runtime.GOOS == "windows" && (strings.HasPrefix(exe, "C:\\Program Files") || strings.HasPrefix(exe, "C:\\Program Files (x86)")) {
			return "C:\\ProgramData\\muxi\\logs", nil
		}
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
	if runtime.GOOS == "windows" {
		// Windows user-level install: %APPDATA%\muxi\logs
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "muxi", "logs"), nil
	}

	// Unix/macOS: ~/.muxi/server/logs
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".muxi", "server", "logs"), nil
}

// GetInstallType returns the installation type for display purposes
// Returns: "System", "User", or "Custom"
func GetInstallType() string {
	// Check if environment variable overrides are set
	if os.Getenv("MUXI_CONFIG_DIR") != "" || os.Getenv("MUXI_DATA_DIR") != "" || os.Getenv("MUXI_LOG_DIR") != "" {
		return "Custom"
	}

	// Check platform and binary location
	exe, err := os.Executable()
	if err == nil {
		// Linux + /usr → System install
		if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/") {
			return "System (Linux)"
		}

		// Windows + Program Files → System install
		if runtime.GOOS == "windows" && (strings.HasPrefix(exe, "C:\\Program Files") || strings.HasPrefix(exe, "C:\\Program Files (x86)")) {
			return "System (Windows)"
		}
	}

	// User-level install
	return "User-level"
}

// GetMuxiDir returns the MUXI server directory
// DEPRECATED: Use GetConfigDir(), GetDataDir(), or GetLogDir() instead
// Kept for backward compatibility
func GetMuxiDir() (string, error) {
	return GetConfigDir()
}

// GetConfigPath returns the default config file path
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, "config.yaml"), nil
}

// GetRegistryPath returns the default registry file path
func GetRegistryPath() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dataDir, "registry.json"), nil
}

// EnsureDirectories creates all necessary directories and normalizes config paths to absolute.
// After this call, config.Formations.LogsDir, PIDsDir, and FormationsDir will be absolute paths.
func EnsureDirectories(baseDir string, config *Config) error {
	// Normalize relative paths to absolute FIRST (before creating directories)
	// This ensures all code using config paths gets absolute paths
	config.Formations.LogsDir = filepath.Join(baseDir, config.Formations.LogsDir)
	config.Formations.PIDsDir = filepath.Join(baseDir, config.Formations.PIDsDir)
	config.Formations.FormationsDir = filepath.Join(baseDir, config.Formations.FormationsDir)

	// Now create all directories using the normalized absolute paths
	dirs := []string{
		baseDir,
		config.Formations.LogsDir,
		config.Formations.PIDsDir,
		config.Formations.FormationsDir,
		filepath.Join(baseDir, "tmp"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
