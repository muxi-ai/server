package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestGetArchString(t *testing.T) {
	arch := getArchString()
	if arch == "" {
		t.Error("expected non-empty arch string")
	}
	valid := map[string]bool{"x86_64": true, "arm64": true, "i386": true}
	if !valid[arch] {
		// Still valid if it's a raw GOARCH value
		t.Logf("arch string: %s (raw GOARCH)", arch)
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"muxi_sk_abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnop", "muxi_sk_...mnop"},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "12345678...6789"},
	}
	for _, tt := range tests {
		got := maskSecret(tt.input)
		if got != tt.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"muxi_pk_abcdef123456", "muxi_pk_abcd••••••••"},
		{"short", "muxi_pk_••••••••"},
		{"muxi_pk_abcd", "muxi_pk_••••••••"},
		{"muxi_pk_abcde", "muxi_pk_abcd••••••••"},
	}
	for _, tt := range tests {
		got := maskKey(tt.input)
		if got != tt.want {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateKey(t *testing.T) {
	key, err := generateKey()
	if err != nil {
		t.Fatalf("generateKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "muxi_pk_") {
		t.Errorf("key should start with muxi_pk_, got %s", key)
	}
	if len(key) != 32 { // "muxi_pk_" (8) + 24 hex chars (12 bytes)
		t.Errorf("key length = %d, want 32", len(key))
	}

	// Should generate unique keys
	key2, _ := generateKey()
	if key == key2 {
		t.Error("keys should be unique")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}
	if !strings.HasPrefix(secret, "muxi_sk_") {
		t.Errorf("secret should start with muxi_sk_, got %s", secret)
	}
	if len(secret) != 64 { // "muxi_sk_" (8) + 56 hex chars (28 bytes)
		t.Errorf("secret length = %d, want 64", len(secret))
	}

	// Should generate unique secrets
	secret2, _ := generateSecret()
	if secret == secret2 {
		t.Error("secrets should be unique")
	}
}

func TestExtractServerName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"server-myhost-abc123", "myhost"},
		{"server-test-deadbeef", "test"},
		{"single", ""},
		{"a-b", "b"},
	}
	for _, tt := range tests {
		got := extractServerName(tt.input)
		if got != tt.want {
			t.Errorf("extractServerName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateServerIDFromName(t *testing.T) {
	id := generateServerIDFromName("myhost")
	if !strings.HasPrefix(id, "server-myhost-") {
		t.Errorf("server ID should start with server-myhost-, got %s", id)
	}

	// Should generate unique IDs
	id2 := generateServerIDFromName("myhost")
	if id == id2 {
		t.Error("server IDs should be unique (random hash)")
	}
}

func TestIsPortAvailable(t *testing.T) {
	// Port 0 tells the OS to assign a free port - but isPortAvailable checks a specific port
	// Port 19999 is very likely available
	if !isPortAvailable(19999) {
		t.Log("port 19999 not available (something else using it)")
	}

	// Port 1 should not be available (privileged, or in use)
	if isPortAvailable(1) {
		t.Log("port 1 was available (running as root?)")
	}
}

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty (embedded from .version)")
	}
}

func TestCreateOrUpdateCLIProfile_NewProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	result, err := createOrUpdateCLIProfile(7890, "muxi_pk_test", "muxi_sk_test")
	if err != nil {
		t.Fatalf("createOrUpdateCLIProfile() error = %v", err)
	}
	if result != ProfileCreated {
		t.Errorf("expected ProfileCreated, got %d", result)
	}

	// Verify file was created
	profilesPath := filepath.Join(tmpDir, ".muxi", "cli", "profiles.yaml")
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatalf("profiles.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "muxi_pk_test") {
		t.Error("profiles.yaml doesn't contain the key")
	}
	if !strings.Contains(string(data), "localhost") {
		t.Error("profiles.yaml doesn't contain localhost profile")
	}
}

func TestCreateOrUpdateCLIProfile_UpdateProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create initial profile
	createOrUpdateCLIProfile(7890, "muxi_pk_old", "muxi_sk_old")

	// Update with new credentials
	result, err := createOrUpdateCLIProfile(7890, "muxi_pk_new", "muxi_sk_new")
	if err != nil {
		t.Fatalf("createOrUpdateCLIProfile() error = %v", err)
	}
	if result != ProfileUpdated {
		t.Errorf("expected ProfileUpdated, got %d", result)
	}
}

func TestCreateOrUpdateCLIProfile_Unchanged(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create profile
	createOrUpdateCLIProfile(7890, "muxi_pk_same", "muxi_sk_same")

	// Same credentials -> unchanged
	result, err := createOrUpdateCLIProfile(7890, "muxi_pk_same", "muxi_sk_same")
	if err != nil {
		t.Fatalf("createOrUpdateCLIProfile() error = %v", err)
	}
	if result != ProfileUnchanged {
		t.Errorf("expected ProfileUnchanged, got %d", result)
	}
}

func TestCmdVersion(t *testing.T) {
	// Should not panic
	err := cmdVersion()
	if err != nil {
		t.Errorf("cmdVersion() error = %v", err)
	}
}

func TestCmdHelp(t *testing.T) {
	// Should not panic
	cmdHelp()
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"unknown", zerolog.InfoLevel},
		{"", zerolog.InfoLevel},
	}
	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGetLogLevel_Default(t *testing.T) {
	origLevel := os.Getenv("MUXI_LOG_LEVEL")
	os.Unsetenv("MUXI_LOG_LEVEL")
	defer func() {
		if origLevel != "" {
			os.Setenv("MUXI_LOG_LEVEL", origLevel)
		}
	}()

	level := getLogLevel()
	if level != zerolog.InfoLevel {
		t.Errorf("getLogLevel() = %v, want InfoLevel", level)
	}
}

func TestGetLogLevel_EnvVar(t *testing.T) {
	os.Setenv("MUXI_LOG_LEVEL", "debug")
	defer os.Unsetenv("MUXI_LOG_LEVEL")

	level := getLogLevel()
	if level != zerolog.DebugLevel {
		t.Errorf("getLogLevel() = %v, want DebugLevel", level)
	}
}

func TestPrintBanner(t *testing.T) {
	// Should not panic
	printBanner()
}

func TestPrintWelcome(t *testing.T) {
	// Should not panic
	printWelcome()
}

func TestGetMuxiServerPath(t *testing.T) {
	path := getMuxiServerPath()
	if path == "" {
		t.Error("getMuxiServerPath() returned empty string")
	}
}

func TestCmdConfigShow_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("MUXI_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("MUXI_CONFIG_DIR")

	// Should use defaults when no config file exists
	err := cmdConfigShow()
	if err != nil {
		t.Errorf("cmdConfigShow() error = %v", err)
	}
}

func TestIsCLIInstalled(t *testing.T) {
	// Just verify it doesn't panic
	result := isCLIInstalled()
	t.Logf("isCLIInstalled() = %v", result)
}

func TestCheckDockerAvailable(t *testing.T) {
	result := checkDockerAvailable()
	t.Logf("checkDockerAvailable() = %v", result)
}

func TestCheckRuntimeRunnerExists(t *testing.T) {
	result := checkRuntimeRunnerExists()
	t.Logf("checkRuntimeRunnerExists() = %v", result)
}

func TestCheckSingularityAvailable(t *testing.T) {
	result := checkSingularityAvailable()
	t.Logf("checkSingularityAvailable() = %v", result)
}

func TestGetSingularityPath(t *testing.T) {
	path := getSingularityPath()
	t.Logf("getSingularityPath() = %q", path)
	// Path might be empty if neither singularity nor apptainer is installed
}

func TestGetLinuxDistro(t *testing.T) {
	distro := getLinuxDistro()
	t.Logf("getLinuxDistro() = %q", distro)
	// On non-Linux systems, this will return empty string
}

func TestGetLinuxDistroLike(t *testing.T) {
	distroLike := getLinuxDistroLike()
	t.Logf("getLinuxDistroLike() = %q", distroLike)
	// On non-Linux systems, this will return empty string
}

func TestGetLinuxDistro_MockOSRelease(t *testing.T) {
	// Create a temp file simulating /etc/os-release
	tmpDir := t.TempDir()
	osRelease := filepath.Join(tmpDir, "os-release")

	testCases := []struct {
		name     string
		content  string
		wantID   string
	}{
		{
			name: "Ubuntu",
			content: `NAME="Ubuntu"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.1 LTS"
VERSION_ID="22.04"`,
			wantID: "ubuntu",
		},
		{
			name: "Debian",
			content: `PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
NAME="Debian GNU/Linux"
VERSION_ID="12"
ID=debian`,
			wantID: "debian",
		},
		{
			name: "Fedora",
			content: `NAME="Fedora Linux"
VERSION="38 (Workstation Edition)"
ID=fedora
ID_LIKE="rhel centos"
VERSION_ID=38`,
			wantID: "fedora",
		},
		{
			name: "Rocky Linux",
			content: `NAME="Rocky Linux"
VERSION="9.1 (Blue Onyx)"
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.1"`,
			wantID: "rocky",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(osRelease, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Read the file directly to parse (simulating getLinuxDistro logic)
			data, _ := os.ReadFile(osRelease)
			var gotID string
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "ID=") {
					gotID = strings.TrimPrefix(line, "ID=")
					gotID = strings.Trim(gotID, "\"")
					gotID = strings.ToLower(gotID)
					break
				}
			}

			if gotID != tc.wantID {
				t.Errorf("Got ID=%q, want %q", gotID, tc.wantID)
			}
		})
	}
}
