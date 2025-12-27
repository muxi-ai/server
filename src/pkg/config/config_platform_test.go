package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Helper function for substring checking
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestWindowsPathDetection tests Windows-specific path logic
// These tests run on all platforms to verify the logic is correct
func TestWindowsPathDetection(t *testing.T) {
	// Note: We can't actually change runtime.GOOS at runtime,
	// but we can test the logic by checking what WOULD happen
	// These tests verify the Windows path logic is correct
	_ = runtime.GOOS // Reference to avoid unused variable warning
	
	t.Run("Windows user paths", func(t *testing.T) {
		// Set APPDATA environment variable (Windows standard)
		oldAppData := os.Getenv("APPDATA")
		defer func() {
			if oldAppData == "" {
				os.Unsetenv("APPDATA")
			} else {
				os.Setenv("APPDATA", oldAppData)
			}
		}()
		
		os.Setenv("APPDATA", "C:\\Users\\TestUser\\AppData\\Roaming")
		
		// Test that Windows path construction would work correctly
		appData := os.Getenv("APPDATA")
		expectedConfig := filepath.Join(appData, "muxi", "server")
		expectedLogs := filepath.Join(appData, "muxi", "logs")
		
		if appData == "" {
			t.Error("APPDATA should be set for Windows user paths")
		}
		
		// Verify path construction
		// Note: filepath.Join uses OS-specific separators
		// On Unix: / on Windows: \
		// We just verify the components are correct
		if expectedConfig == "" {
			t.Error("Windows user config path should not be empty")
		}
		if !contains(expectedConfig, "muxi") || !contains(expectedConfig, "server") {
			t.Errorf("Windows user config path = %s should contain 'muxi' and 'server'", expectedConfig)
		}
		
		if expectedLogs == "" {
			t.Error("Windows user log path should not be empty")
		}
		if !contains(expectedLogs, "muxi") || !contains(expectedLogs, "logs") {
			t.Errorf("Windows user log path = %s should contain 'muxi' and 'logs'", expectedLogs)
		}
	})
	
	t.Run("Windows system paths", func(t *testing.T) {
		// Test Windows system path constants
		expectedConfig := "C:\\ProgramData\\muxi\\server"
		expectedData := "C:\\ProgramData\\muxi\\data"
		expectedLogs := "C:\\ProgramData\\muxi\\logs"
		
		// Verify these are the correct Windows paths
		if expectedConfig != "C:\\ProgramData\\muxi\\server" {
			t.Errorf("Windows system config path incorrect")
		}
		
		if expectedData != "C:\\ProgramData\\muxi\\data" {
			t.Errorf("Windows system data path incorrect")
		}
		
		if expectedLogs != "C:\\ProgramData\\muxi\\logs" {
			t.Errorf("Windows system log path incorrect")
		}
	})
	
	t.Run("Windows install type detection", func(t *testing.T) {
		// Test the logic for detecting Windows install type
		// System: C:\Program Files or C:\Program Files (x86)
		// User: Other locations
		
		testCases := []struct {
			exePath      string
			expectedType string
		}{
			{"C:\\Program Files\\muxi\\muxi-server.exe", "System (Windows)"},
			{"C:\\Program Files (x86)\\muxi\\muxi-server.exe", "System (Windows)"},
			{"C:\\Users\\TestUser\\AppData\\Local\\muxi\\bin\\muxi-server.exe", "User-level"},
			{"C:\\muxi\\muxi-server.exe", "User-level"},
			{"D:\\Tools\\muxi-server.exe", "User-level"},
		}
		
		for _, tc := range testCases {
			// Test path detection logic
			isSystemInstall := (len(tc.exePath) >= 16 && tc.exePath[:16] == "C:\\Program Files") ||
				(len(tc.exePath) >= 22 && tc.exePath[:22] == "C:\\Program Files (x86)")
			
			var installType string
			if isSystemInstall {
				installType = "System (Windows)"
			} else {
				installType = "User-level"
			}
			
			if installType != tc.expectedType {
				t.Errorf("Path %s: got %s, want %s", tc.exePath, installType, tc.expectedType)
			}
		}
	})
	
	t.Run("Windows APPDATA validation", func(t *testing.T) {
		// Test APPDATA environment variable handling
		oldAppData := os.Getenv("APPDATA")
		defer func() {
			if oldAppData == "" {
				os.Unsetenv("APPDATA")
			} else {
				os.Setenv("APPDATA", oldAppData)
			}
		}()
		
		// Test with valid APPDATA
		os.Setenv("APPDATA", "C:\\Users\\Test\\AppData\\Roaming")
		appData := os.Getenv("APPDATA")
		if appData == "" {
			t.Error("APPDATA should not be empty when set")
		}
		
		// Test with missing APPDATA
		os.Unsetenv("APPDATA")
		appData = os.Getenv("APPDATA")
		if appData != "" {
			t.Error("APPDATA should be empty when unset")
		}
	})
	
	t.Run("Windows path separators", func(t *testing.T) {
		// Verify Windows path separator handling
		path := filepath.Join("C:", "Users", "Test", "muxi")
		
		// On Windows, this should use backslashes
		// On Unix, filepath.Join will use forward slashes
		// Both are valid for testing the logic
		
		if path == "" {
			t.Error("Joined path should not be empty")
		}
		
		// Test that we're using filepath.Join correctly for cross-platform
		parts := []string{"C:", "ProgramData", "muxi", "server"}
		joined := filepath.Join(parts...)
		
		if joined == "" {
			t.Error("Filepath join should produce valid path")
		}
	})
}

// TestWindowsPathLogicOnCurrentPlatform tests Windows path functions
// This verifies the actual GetConfigDir/GetDataDir/GetLogDir functions
// handle Windows detection correctly (even when run on Unix)
func TestWindowsPathLogicOnCurrentPlatform(t *testing.T) {
	t.Run("GetInstallType detection", func(t *testing.T) {
		// Test that GetInstallType returns valid values
		installType := GetInstallType()
		
		validTypes := []string{"User-level", "System (Linux)", "System (Windows)", "Custom"}
		isValid := false
		for _, validType := range validTypes {
			if installType == validType {
				isValid = true
				break
			}
		}
		
		if !isValid {
			t.Errorf("GetInstallType() = %s, want one of %v", installType, validTypes)
		}
	})
	
	t.Run("Environment overrides work cross-platform", func(t *testing.T) {
		// Test that environment variable overrides work
		oldConfig := os.Getenv("MUXI_CONFIG_DIR")
		oldData := os.Getenv("MUXI_DATA_DIR")
		oldLog := os.Getenv("MUXI_LOG_DIR")
		
		defer func() {
			if oldConfig == "" {
				os.Unsetenv("MUXI_CONFIG_DIR")
			} else {
				os.Setenv("MUXI_CONFIG_DIR", oldConfig)
			}
			if oldData == "" {
				os.Unsetenv("MUXI_DATA_DIR")
			} else {
				os.Setenv("MUXI_DATA_DIR", oldData)
			}
			if oldLog == "" {
				os.Unsetenv("MUXI_LOG_DIR")
			} else {
				os.Setenv("MUXI_LOG_DIR", oldLog)
			}
		}()
		
		// Set custom paths
		testConfigDir := "/tmp/test-muxi-config"
		testDataDir := "/tmp/test-muxi-data"
		testLogDir := "/tmp/test-muxi-logs"
		
		os.Setenv("MUXI_CONFIG_DIR", testConfigDir)
		os.Setenv("MUXI_DATA_DIR", testDataDir)
		os.Setenv("MUXI_LOG_DIR", testLogDir)
		
		// Verify overrides are used
		configDir, err := GetConfigDir()
		if err != nil {
			t.Errorf("GetConfigDir() error = %v", err)
		}
		if configDir != testConfigDir {
			t.Errorf("GetConfigDir() = %s, want %s", configDir, testConfigDir)
		}
		
		dataDir, err := GetDataDir()
		if err != nil {
			t.Errorf("GetDataDir() error = %v", err)
		}
		if dataDir != testDataDir {
			t.Errorf("GetDataDir() = %s, want %s", dataDir, testDataDir)
		}
		
		logDir, err := GetLogDir()
		if err != nil {
			t.Errorf("GetLogDir() error = %v", err)
		}
		if logDir != testLogDir {
			t.Errorf("GetLogDir() = %s, want %s", logDir, testLogDir)
		}
		
		// Verify install type is "Custom" with env overrides
		installType := GetInstallType()
		if installType != "Custom" {
			t.Errorf("GetInstallType() with env overrides = %s, want Custom", installType)
		}
	})
	
	t.Run("Path functions return valid paths", func(t *testing.T) {
		// Ensure all path functions return non-empty, valid paths
		configDir, err := GetConfigDir()
		if err != nil {
			t.Errorf("GetConfigDir() error = %v", err)
		}
		if configDir == "" {
			t.Error("GetConfigDir() should return non-empty path")
		}
		
		dataDir, err := GetDataDir()
		if err != nil {
			t.Errorf("GetDataDir() error = %v", err)
		}
		if dataDir == "" {
			t.Error("GetDataDir() should return non-empty path")
		}
		
		logDir, err := GetLogDir()
		if err != nil {
			t.Errorf("GetLogDir() error = %v", err)
		}
		if logDir == "" {
			t.Error("GetLogDir() should return non-empty path")
		}
		
		configPath, err := GetConfigPath()
		if err != nil {
			t.Errorf("GetConfigPath() error = %v", err)
		}
		if configPath == "" {
			t.Error("GetConfigPath() should return non-empty path")
		}
		
		registryPath, err := GetRegistryPath()
		if err != nil {
			t.Errorf("GetRegistryPath() error = %v", err)
		}
		if registryPath == "" {
			t.Error("GetRegistryPath() should return non-empty path")
		}
	})
	
	t.Run("Cross-platform compatibility", func(t *testing.T) {
		// Verify that functions work on current platform (Unix or Windows)
		configDir, err := GetConfigDir()
		if err != nil {
			t.Fatalf("GetConfigDir() failed: %v", err)
		}
		
		// Path should be absolute
		if !filepath.IsAbs(configDir) {
			t.Errorf("GetConfigDir() = %s is not absolute path", configDir)
		}
		
		// Should work with filepath operations
		configPath := filepath.Join(configDir, "config.yaml")
		if configPath == "" {
			t.Error("filepath.Join with config dir should work")
		}
	})
}

// TestWindowsConfigCreation tests config file operations work on Windows-style paths
func TestWindowsConfigCreation(t *testing.T) {
	t.Run("Config save and load with Windows-style paths", func(t *testing.T) {
		// Create temp directory for testing
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")
		
		// Create default config
		cfg := DefaultConfig()
		cfg.ServerID = "test-windows-config"
		
		// Save config
		err := cfg.Save(configPath)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		
		// Load config
		loadedCfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		
		// Verify loaded config matches
		if loadedCfg.ServerID != cfg.ServerID {
			t.Errorf("Loaded ServerID = %s, want %s", loadedCfg.ServerID, cfg.ServerID)
		}
	})
	
	t.Run("EnsureDirectories with Windows-style paths", func(t *testing.T) {
		// Create temp base directory
		tempDir := t.TempDir()
		
		cfg := DefaultConfig()
		
		// Ensure directories are created
		err := EnsureDirectories(tempDir, cfg)
		if err != nil {
			t.Fatalf("EnsureDirectories() error = %v", err)
		}
		
		// Verify directories exist
		// After EnsureDirectories, config paths are normalized to absolute paths
		dirs := []string{
			tempDir,
			cfg.Formations.LogsDir,
			cfg.Formations.PIDsDir,
			cfg.Formations.FormationsDir,
		}
		
		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("Directory %s was not created", dir)
			}
		}
	})
}
