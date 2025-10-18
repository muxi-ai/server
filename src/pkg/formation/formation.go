package formation

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Formation represents a parsed formation.yaml file
type Formation struct {
	Schema      string        `yaml:"schema"`
	ID          string        `yaml:"id"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Version     string        `yaml:"version"`
	Runtime     RuntimeConfig `yaml:"runtime"`
	// We can add more fields as needed, but ID is the critical one
}

// RuntimeConfig contains runtime settings
type RuntimeConfig struct {
	BuiltInMCPs bool `yaml:"built_in_mcps"`
}

// ParseFormationYAML parses a formation.yaml file
func ParseFormationYAML(path string) (*Formation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read formation.yaml: %w", err)
	}

	var f Formation
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("failed to parse formation.yaml: %w", err)
	}

	// Validate required fields
	if f.ID == "" {
		return nil, fmt.Errorf("formation.yaml must have an 'id' field")
	}

	return &f, nil
}

// GetDefaultCommand returns the default command to run the formation
// For now, we assume app.py exists and use Python
func (f *Formation) GetDefaultCommand() string {
	return "python"
}

// GetDefaultArgs returns the default command arguments
func (f *Formation) GetDefaultArgs() []string {
	return []string{"app.py"}
}

// GetEnvironmentVars returns environment variables for the formation
func (f *Formation) GetEnvironmentVars(port int, serverURL string, bindHost string) map[string]string {
	env := make(map[string]string)

	// Network binding (CRITICAL for security)
	env["PORT"] = fmt.Sprintf("%d", port)
	env["HOST"] = bindHost // Formations must bind to this host

	// Formation metadata
	env["FORMATION_ID"] = f.ID
	env["MUXI_SERVER_URL"] = serverURL
	env["MUXI_ENV"] = "production"

	// Metadata for formation.yaml injection
	env["_bind_host"] = bindHost
	env["_port"] = fmt.Sprintf("%d", port)

	return env
}
