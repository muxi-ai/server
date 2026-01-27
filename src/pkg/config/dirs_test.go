package config

import (
	"os"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty config dir")
	}
}

func TestGetConfigDir_EnvOverride(t *testing.T) {
	os.Setenv("MUXI_CONFIG_DIR", "/tmp/muxi-test-config")
	defer os.Unsetenv("MUXI_CONFIG_DIR")

	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error = %v", err)
	}
	if dir != "/tmp/muxi-test-config" {
		t.Errorf("expected env override, got %s", dir)
	}
}

func TestGetDataDir(t *testing.T) {
	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error = %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty data dir")
	}
}

func TestGetDataDir_EnvOverride(t *testing.T) {
	os.Setenv("MUXI_DATA_DIR", "/tmp/muxi-test-data")
	defer os.Unsetenv("MUXI_DATA_DIR")

	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error = %v", err)
	}
	if dir != "/tmp/muxi-test-data" {
		t.Errorf("expected env override, got %s", dir)
	}
}

func TestGetLogDir(t *testing.T) {
	dir, err := GetLogDir()
	if err != nil {
		t.Fatalf("GetLogDir() error = %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty log dir")
	}
}

func TestGetLogDir_EnvOverride(t *testing.T) {
	os.Setenv("MUXI_LOG_DIR", "/tmp/muxi-test-logs")
	defer os.Unsetenv("MUXI_LOG_DIR")

	dir, err := GetLogDir()
	if err != nil {
		t.Fatalf("GetLogDir() error = %v", err)
	}
	if dir != "/tmp/muxi-test-logs" {
		t.Errorf("expected env override, got %s", dir)
	}
}

func TestGetInstallType(t *testing.T) {
	installType := GetInstallType()
	if installType == "" {
		t.Error("expected non-empty install type")
	}
	valid := map[string]bool{
		"User-level":       true,
		"System (Linux)":   true,
		"System (Windows)": true,
		"Custom":           true,
	}
	if !valid[installType] {
		t.Errorf("unexpected install type: %s", installType)
	}
}

func TestGetInstallType_CustomEnv(t *testing.T) {
	os.Setenv("MUXI_CONFIG_DIR", "/custom")
	defer os.Unsetenv("MUXI_CONFIG_DIR")

	if GetInstallType() != "Custom" {
		t.Error("expected Custom install type when env override set")
	}
}
