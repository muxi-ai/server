package formation

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Formation represents a parsed formation.yaml file
type Formation struct {
	Schema      string `yaml:"schema"`
	ID          string `yaml:"id"`
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description"`
	Version     string `yaml:"version,omitempty"`
	MuxiRuntime string `yaml:"muxi_runtime,omitempty"` // MUXI Runtime SIF version (e.g., "0.2025.0", "latest")
	Author      string `yaml:"author,omitempty"`
	URL         string `yaml:"url,omitempty"`
	License     string `yaml:"license,omitempty"`
	// Note: "runtime" field is reserved for MUXI runtime's own configuration
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
	if f.Description == "" {
		return nil, fmt.Errorf("formation.yaml must have a 'description' field")
	}

	// Set defaults
	if f.MuxiRuntime == "" {
		f.MuxiRuntime = "latest"
	}
	if f.License == "" {
		f.License = "Unlicense"
	}

	// Validate ID (check for reserved words)
	if err := f.ValidateID(); err != nil {
		return nil, err
	}

	return &f, nil
}

// ValidateID checks if the formation ID is valid
func (f *Formation) ValidateID() error {
	if f.ID == "" {
		return fmt.Errorf("formation ID cannot be empty")
	}

	// Check for reserved IDs
	reserved := map[string]bool{
		"health":  true,
		"ping":    true,
		"rpc":     true,
		"server":  true,
		"admin":   true,
		"metrics": true,
		"api":     true,
	}

	if reserved[f.ID] {
		return fmt.Errorf("formation ID '%s' is reserved", f.ID)
	}

	return nil
}

// ValidateSecrets checks if secrets are referenced in formation.yaml and validates them
// Returns an error if secrets are referenced but the secrets file doesn't exist or is missing required secrets
func ValidateSecrets(formationDir string) error {
	formationPath := formationDir + "/formation.yaml"
	secretsPath := formationDir + "/secrets"

	// Read formation.yaml content
	data, err := os.ReadFile(formationPath)
	if err != nil {
		return fmt.Errorf("failed to read formation.yaml: %w", err)
	}

	// Find all secrets references: ${{ secrets.XXX }}
	content := string(data)
	secretRefs := findSecretsReferences(content)
	if len(secretRefs) == 0 {
		return nil // No secrets referenced
	}

	// Check if secrets file exists
	secretsData, err := os.ReadFile(secretsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("formation references secrets (%v) but 'secrets' file not found", secretRefs)
		}
		return fmt.Errorf("failed to read secrets file: %w", err)
	}

	// Parse secrets file (format: KEY=value or KEY: value per line)
	providedSecrets := parseSecretsFile(string(secretsData))

	// Check all referenced secrets are provided
	var missing []string
	for _, ref := range secretRefs {
		if _, ok := providedSecrets[ref]; !ok {
			missing = append(missing, ref)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing secrets in 'secrets' file: %v", missing)
	}

	return nil
}

// findSecretsReferences finds all ${{ secrets.XXX }} patterns in content
func findSecretsReferences(content string) []string {
	var refs []string
	seen := make(map[string]bool)

	// Simple pattern matching for ${{ secrets.XXX }}
	start := 0
	for {
		idx := indexOf(content[start:], "${{ secrets.")
		if idx == -1 {
			break
		}
		idx += start + len("${{ secrets.")

		// Find the end: " }}" or just "}}"
		end := indexOf(content[idx:], "}}")
		if end == -1 {
			break
		}

		secretName := content[idx : idx+end]
		// Trim any whitespace
		secretName = trimSpace(secretName)

		if !seen[secretName] {
			refs = append(refs, secretName)
			seen[secretName] = true
		}
		start = idx + end
	}

	return refs
}

// parseSecretsFile parses a secrets file (KEY=value or KEY: value format)
func parseSecretsFile(content string) map[string]string {
	secrets := make(map[string]string)
	lines := splitLines(content)

	for _, line := range lines {
		line = trimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		// Try KEY=value format
		if idx := indexOf(line, "="); idx > 0 {
			key := trimSpace(line[:idx])
			value := trimSpace(line[idx+1:])
			secrets[key] = value
			continue
		}

		// Try KEY: value format (YAML-style)
		if idx := indexOf(line, ":"); idx > 0 {
			key := trimSpace(line[:idx])
			value := trimSpace(line[idx+1:])
			secrets[key] = value
		}
	}

	return secrets
}

// Helper functions to avoid importing strings package
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
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
