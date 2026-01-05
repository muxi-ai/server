package telemetry

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// globalConfigPath returns the path to ~/.muxi/config.yaml
func globalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".muxi", "config.yaml")
}

// loadGlobalConfig loads the global config as a raw map
func loadGlobalConfig() (map[string]interface{}, error) {
	path := globalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]interface{}), nil
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return make(map[string]interface{}), nil
	}

	return config, nil
}

// saveGlobalConfig saves the global config
func saveGlobalConfig(config map[string]interface{}) error {
	path := globalConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsEnabled checks if telemetry is enabled
func IsEnabled() bool {
	// Env var takes precedence
	if os.Getenv("MUXI_TELEMETRY") == "0" {
		return false
	}

	config, err := loadGlobalConfig()
	if err != nil {
		return true // default enabled
	}

	if enabled, ok := config["telemetry"].(bool); ok {
		return enabled
	}

	return true // default enabled
}

// getCachedMachineID returns the cached machine ID from global config
func getCachedMachineID() string {
	config, _ := loadGlobalConfig()
	if id, ok := config["machine_id"].(string); ok {
		return id
	}
	return ""
}

// cacheMachineID stores the machine ID in global config
func cacheMachineID(machineID string) {
	config, _ := loadGlobalConfig()
	config["machine_id"] = machineID
	saveGlobalConfig(config)
}

// getCachedCountry returns the cached country from global config
func getCachedCountry() string {
	config, _ := loadGlobalConfig()
	if country, ok := config["country"].(string); ok {
		return country
	}
	return ""
}

// cacheCountry stores the country in global config
func cacheCountry(country string) {
	config, _ := loadGlobalConfig()
	config["country"] = country
	saveGlobalConfig(config)
}
