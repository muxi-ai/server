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
	Logging    LoggingConfig    `yaml:"logging"`
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

	// Log rotation
	LogRotationEnabled bool   `yaml:"log_rotation_enabled"` // Enable log rotation (default: true)
	LogMaxSize         string `yaml:"log_max_size"`         // Max log file size (default: "10M")
	LogMaxFiles        int    `yaml:"log_max_files"`        // Max log files to keep (default: 10)
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
			LogRotationEnabled:  true,
			LogMaxSize:          "10M",
			LogMaxFiles:         10,
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

	return config, nil
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
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
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
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
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
	}

	// 3. User paths (macOS, Windows, or non-system Linux)
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".muxi", "server", "logs"), nil
}

// GetInstallType returns the installation type for display purposes
// Returns: "System", "User", or "Development"
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

// EnsureDirectories creates all necessary directories
func EnsureDirectories(baseDir string, config *Config) error {
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, config.Formations.LogsDir),
		filepath.Join(baseDir, config.Formations.PIDsDir),
		filepath.Join(baseDir, config.Formations.FormationsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
