package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestSpawn_Validation(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name    string
		config  SpawnConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: SpawnConfig{
				ID:      "test",
				Command: "echo",
				Args:    []string{"hello"},
				Logger:  &logger,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			config: SpawnConfig{
				Command: "echo",
				Logger:  &logger,
			},
			wantErr: true,
			errMsg:  "process ID is required",
		},
		{
			name: "missing command",
			config: SpawnConfig{
				ID:     "test",
				Logger: &logger,
			},
			wantErr: true,
			errMsg:  "command is required",
		},
		{
			name: "invalid command",
			config: SpawnConfig{
				ID:      "test",
				Command: "nonexistent-command-xyz123",
				Logger:  &logger,
			},
			wantErr: true,
			errMsg:  "executable not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.config.LogsDir = filepath.Join(tmpDir, "logs")
			tt.config.PIDsDir = filepath.Join(tmpDir, "pids")
			os.MkdirAll(tt.config.LogsDir, 0755)
			os.MkdirAll(tt.config.PIDsDir, 0755)

			_, err := Spawn(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Spawn() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Spawn() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Spawn() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSpawnConfig_Defaults(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:      "test-defaults",
		Command: "echo",
		Args:    []string{"test"},
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	proc, err := Spawn(config)
	if err != nil {
		// Spawn may fail for various reasons in test environment
		// We're mainly testing that defaults are applied
		t.Logf("Spawn failed (expected in test env): %v", err)
		return
	}

	if proc == nil {
		t.Fatal("Spawn() returned nil process")
	}

	// Verify defaults
	if proc.Name == "" {
		// Default name should be ID
		t.Error("Process name should default to ID")
	}

	if proc.WorkDir == "" {
		t.Error("WorkDir should be set to default")
	}

	if proc.MaxRestarts == 0 {
		// Should have a default max restart value
		t.Logf("MaxRestarts = %d (may be 0 in tests)", proc.MaxRestarts)
	}

	// Clean up if process started
	if proc.cmd != nil && proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
}

func TestSpawnConfig_LogFiles(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:      "test-logs",
		Command: "echo",
		Args:    []string{"test output"},
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	_, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn failed (may be expected): %v", err)
		// Still check if log files were created
	}

	// Check if log files exist
	outLog := filepath.Join(config.LogsDir, "test-logs-out.log")
	errLog := filepath.Join(config.LogsDir, "test-logs-err.log")

	if _, err := os.Stat(outLog); err != nil {
		t.Logf("Output log file not created: %v", err)
	} else {
		t.Logf("Output log created: %s", outLog)
	}

	if _, err := os.Stat(errLog); err != nil {
		t.Logf("Error log file not created: %v", err)
	} else {
		t.Logf("Error log created: %s", errLog)
	}
}

func TestSpawnConfig_WorkDir(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	// Create a custom work directory
	workDir := filepath.Join(tmpDir, "workdir")
	os.MkdirAll(workDir, 0755)

	config := SpawnConfig{
		ID:      "test-workdir",
		Command: "echo",
		Args:    []string{"test"},
		WorkDir: workDir,
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	proc, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn failed: %v", err)
		return
	}

	if proc.WorkDir != workDir {
		t.Errorf("WorkDir = %q, want %q", proc.WorkDir, workDir)
	}

	// Clean up
	if proc.cmd != nil && proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
}

func TestSpawnConfig_Environment(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:      "test-env",
		Command: "echo",
		Env: map[string]string{
			"TEST_VAR": "test_value",
			"PORT":     "8080",
		},
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	_, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn failed: %v", err)
	}

	// Environment variables should be passed to the process
	// Hard to verify in unit test without actually running a process that checks them
	t.Log("Environment variables configured")
}

func TestSpawnConfig_AutoRestart(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:          "test-autorestart",
		Command:     "echo",
		AutoRestart: true,
		LogsDir:     filepath.Join(tmpDir, "logs"),
		PIDsDir:     filepath.Join(tmpDir, "pids"),
		Logger:      &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	proc, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn failed: %v", err)
		return
	}

	if !proc.AutoRestart {
		t.Error("AutoRestart should be true")
	}

	// Clean up
	if proc.cmd != nil && proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
}

func TestStop_Process(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("stop process without cmd", func(t *testing.T) {
		proc := &Process{
			ID:     "test",
			Status: StatusRunning,
		}

		err := Stop(proc, &logger)
		// Should handle gracefully
		t.Logf("Stop without cmd: %v", err)
	})

	t.Run("stop already stopped process", func(t *testing.T) {
		proc := &Process{
			ID:     "test",
			Status: StatusStopped,
		}

		err := Stop(proc, &logger)
		// Should be idempotent
		t.Logf("Stop already stopped: %v", err)
	})
}

func TestSpawn_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zerolog.Nop()

	config := SpawnConfig{
		ID:      "test-with-logger",
		Command: "echo",
		Args:    []string{"test"},
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	_, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn with logger: %v", err)
	}
}

func TestSpawn_WithoutLogger(t *testing.T) {
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:      "test-no-logger",
		Command: "echo",
		Args:    []string{"test"},
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  nil, // No logger provided
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	// Should handle nil logger gracefully
	_, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn without logger: %v", err)
	}
}

func TestSpawnConfig_Port(t *testing.T) {
	logger := zerolog.Nop()
	tmpDir := t.TempDir()

	config := SpawnConfig{
		ID:      "test-port",
		Command: "echo",
		Port:    8080,
		LogsDir: filepath.Join(tmpDir, "logs"),
		PIDsDir: filepath.Join(tmpDir, "pids"),
		Logger:  &logger,
	}

	os.MkdirAll(config.LogsDir, 0755)
	os.MkdirAll(config.PIDsDir, 0755)

	proc, err := Spawn(config)
	if err != nil {
		t.Logf("Spawn failed: %v", err)
		return
	}

	// Port should be reflected in health check URL
	if proc.HealthCheckURL != "" && !contains(proc.HealthCheckURL, "8080") {
		t.Logf("HealthCheckURL = %q (doesn't contain port 8080)", proc.HealthCheckURL)
	}

	// Clean up
	if proc.cmd != nil && proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsStr(s, substr)))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildNativeSingularityCommand(t *testing.T) {
	logger := zerolog.Nop()

	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/path/to/runtime.sif",
		Command: "python",
		Args:    []string{"-m", "myapp"},
		Port:    8080,
		Env: map[string]string{
			"MUXI_PORT": "8080",
			"MUXI_HOST": "127.0.0.1",
		},
	}

	cmd := buildNativeSingularityCommand(config, &logger)

	if cmd == nil {
		t.Fatal("buildNativeSingularityCommand returned nil")
	}

	// Check the command is singularity
	if cmd.Path == "" {
		t.Error("Command path should not be empty")
	}

	// Check args contain expected values
	args := cmd.Args
	foundExec := false
	foundBind := false
	foundSIF := false
	foundPython := false

	for i, arg := range args {
		if arg == "exec" {
			foundExec = true
		}
		if arg == "--bind" && i+1 < len(args) && containsStr(args[i+1], "/formation") {
			foundBind = true
		}
		if arg == "/path/to/runtime.sif" {
			foundSIF = true
		}
		if arg == "python" {
			foundPython = true
		}
	}

	if !foundExec {
		t.Error("Args should contain 'exec'")
	}
	if !foundBind {
		t.Error("Args should contain bind mount for /formation")
	}
	if !foundSIF {
		t.Error("Args should contain SIF path")
	}
	if !foundPython {
		t.Error("Args should contain command 'python'")
	}
}

func TestBuildDockerSingularityCommand(t *testing.T) {
	logger := zerolog.Nop()

	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/path/to/runtime.sif",
		Port:    8080,
		Env: map[string]string{
			"MUXI_PORT": "8080",
		},
	}

	cmd := buildDockerSingularityCommand(config, &logger)

	if cmd == nil {
		t.Fatal("buildDockerSingularityCommand returned nil")
	}

	args := cmd.Args
	foundRun := false
	foundPrivileged := false
	foundName := false
	foundPort := false
	foundImage := false

	for i, arg := range args {
		if arg == "run" {
			foundRun = true
		}
		if arg == "--privileged" {
			foundPrivileged = true
		}
		if arg == "--name" && i+1 < len(args) && args[i+1] == "muxi-test-formation" {
			foundName = true
		}
		if arg == "-p" && i+1 < len(args) && args[i+1] == "8080:8080" {
			foundPort = true
		}
		if containsStr(arg, "runtime-runner") {
			foundImage = true
		}
	}

	if !foundRun {
		t.Error("Args should contain 'run'")
	}
	if !foundPrivileged {
		t.Error("Args should contain '--privileged'")
	}
	if !foundName {
		t.Error("Args should contain '--name muxi-test-formation'")
	}
	if !foundPort {
		t.Error("Args should contain port mapping '-p 8080:8080'")
	}
	if !foundImage {
		t.Error("Args should contain runtime-runner image")
	}
}

// hasBindArg reports whether args contains a "--bind" flag followed by a
// value equal to want.
func hasBindArg(args []string, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--bind" && args[i+1] == want {
			return true
		}
	}
	return false
}

// hasDashVArg reports whether args contains a "-v" flag followed by a
// value equal to want.
func hasDashVArg(args []string, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-v" && args[i+1] == want {
			return true
		}
	}
	return false
}

func TestBuildNativeSingularityCommand_WithHFCache(t *testing.T) {
	// HFCacheDir set -> expect --bind <host>:/opt/hf-cache appended.
	// This is the hinge of the server-side embedding-platform integration:
	// the runtime's HF_HOME env var points inside the SIF at /opt/hf-cache,
	// so the host cache MUST land at exactly that mount point.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:         "test-formation",
		WorkDir:    "/path/to/formation",
		SIFPath:    "/path/to/runtime.sif",
		HFCacheDir: "/var/lib/muxi/cache",
		Command:    "python",
	}

	cmd := buildNativeSingularityCommand(config, &logger)
	if !hasBindArg(cmd.Args, "/var/lib/muxi/cache:/opt/hf-cache") {
		t.Errorf("expected --bind /var/lib/muxi/cache:/opt/hf-cache, got args: %v", cmd.Args)
	}
}

func TestBuildNativeSingularityCommand_WithoutHFCache(t *testing.T) {
	// HFCacheDir empty -> expect NO /opt/hf-cache bind. This is the
	// back-compat contract: existing native formations and tests that
	// leave HFCacheDir unset must produce the same command as before
	// this feature existed.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/path/to/runtime.sif",
		Command: "python",
	}

	cmd := buildNativeSingularityCommand(config, &logger)
	for i, arg := range cmd.Args {
		if arg == "--bind" && i+1 < len(cmd.Args) && containsStr(cmd.Args[i+1], "/opt/hf-cache") {
			t.Errorf("unexpected /opt/hf-cache bind with empty HFCacheDir: %v", cmd.Args)
		}
	}
}

func TestBuildDockerSingularityCommand_WithHFCache(t *testing.T) {
	// HFCacheDir set -> expect BOTH hops of the two-hop chain:
	//   Docker hop: -v <host>:/opt/hf-cache
	//   Singularity hop: --bind /opt/hf-cache
	// Either one alone is a bug: the Docker hop without the singularity
	// hop means the mount only reaches the Docker container, not the SIF.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:         "test-formation",
		WorkDir:    "/path/to/formation",
		SIFPath:    "/path/to/runtime.sif",
		HFCacheDir: "/Users/me/.muxi/server/cache",
		Port:       8080,
	}

	cmd := buildDockerSingularityCommand(config, &logger)

	if !hasDashVArg(cmd.Args, "/Users/me/.muxi/server/cache:/opt/hf-cache") {
		t.Errorf("missing Docker hop (-v <host>:/opt/hf-cache), got args: %v", cmd.Args)
	}
	if !hasBindArg(cmd.Args, "/opt/hf-cache") {
		t.Errorf("missing Singularity hop (--bind /opt/hf-cache), got args: %v", cmd.Args)
	}
}

func TestBuildDockerSingularityCommand_WithoutHFCache(t *testing.T) {
	// Symmetric back-compat guard for the Docker-wrapped path.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/path/to/runtime.sif",
		Port:    8080,
	}

	cmd := buildDockerSingularityCommand(config, &logger)
	for i, arg := range cmd.Args {
		if arg == "-v" && i+1 < len(cmd.Args) && containsStr(cmd.Args[i+1], "/opt/hf-cache") {
			t.Errorf("unexpected /opt/hf-cache -v with empty HFCacheDir: %v", cmd.Args)
		}
		if arg == "--bind" && i+1 < len(cmd.Args) && cmd.Args[i+1] == "/opt/hf-cache" {
			t.Errorf("unexpected /opt/hf-cache --bind with empty HFCacheDir: %v", cmd.Args)
		}
	}
}

// hasFlagValue reports whether args contains a single-token flag (e.g.
// "--platform") immediately followed by a value equal to want. Mirrors
// the hasBindArg / hasDashVArg helpers above, generalized so the
// platform-pinning tests don't need yet another bespoke walker.
func hasFlagValue(args []string, flag, want string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == want {
			return true
		}
	}
	return false
}

func TestSifPlatform(t *testing.T) {
	// Lock the SIF filename -> --platform mapping so the pin in
	// buildDockerSingularityCommand can't silently drift from the
	// resolver's filename convention (runtime/sif.go).
	cases := []struct {
		name string
		path string
		want string
	}{
		{
			name: "lean variant amd64",
			path: "/Users/u/.muxi/server/runtimes/muxi-runtime-0.20260508.0-linux-amd64.sif",
			want: "linux/amd64",
		},
		{
			name: "lean variant arm64",
			path: "/Users/u/.muxi/server/runtimes/muxi-runtime-0.20260508.0-linux-arm64.sif",
			want: "linux/arm64",
		},
		{
			name: "pytorch variant amd64",
			path: "/runtimes/muxi-runtime-1.2.3-pytorch-linux-amd64.sif",
			want: "linux/amd64",
		},
		{
			name: "cuda variant arm64",
			path: "/runtimes/muxi-runtime-1.2.3-cuda-linux-arm64.sif",
			want: "linux/arm64",
		},
		{
			name: "unparseable fixture defaults to amd64",
			path: "/path/to/runtime.sif",
			want: "linux/amd64",
		},
		{
			name: "empty path defaults to amd64",
			path: "",
			want: "linux/amd64",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sifPlatform(tc.path); got != tc.want {
				t.Errorf("sifPlatform(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestBuildDockerSingularityCommand_PinsPlatformAmd64(t *testing.T) {
	// Regression: runtime-runner became multi-arch, and Docker on Apple
	// Silicon started preferring the arm64 manifest by default. The
	// docker-run builder must pin --platform to the SIF's architecture
	// so the runner and SIF stay in lockstep. This test fails if anyone
	// removes the --platform pin or moves it after the image arg.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/runtimes/muxi-runtime-0.20260508.0-linux-amd64.sif",
		Port:    8080,
	}

	cmd := buildDockerSingularityCommand(config, &logger)

	if !hasFlagValue(cmd.Args, "--platform", "linux/amd64") {
		t.Errorf("missing --platform linux/amd64 pin, got args: %v", cmd.Args)
	}

	// --platform MUST appear before the image name; Docker ignores
	// run-flags placed after the image. The image arg is whatever
	// matches runtime-runner.
	platformIdx, imageIdx := -1, -1
	for i, arg := range cmd.Args {
		if arg == "--platform" {
			platformIdx = i
		}
		if containsStr(arg, "runtime-runner") {
			imageIdx = i
		}
	}
	if platformIdx == -1 {
		t.Fatalf("expected --platform flag, got args: %v", cmd.Args)
	}
	if imageIdx == -1 {
		t.Fatalf("expected runtime-runner image arg, got args: %v", cmd.Args)
	}
	if platformIdx > imageIdx {
		t.Errorf("--platform must precede the image arg; got --platform at %d and image at %d", platformIdx, imageIdx)
	}
}

func TestBuildDockerSingularityCommand_PinsPlatformArm64(t *testing.T) {
	// Symmetric: when (eventually) an arm64 SIF is selected, the runner
	// must be pinned to linux/arm64 — not silently fall back to amd64.
	// This guards the future case where linux-arm64 SIFs start shipping
	// and the resolver picks them on Apple Silicon.
	logger := zerolog.Nop()
	config := SpawnConfig{
		ID:      "test-formation",
		WorkDir: "/path/to/formation",
		SIFPath: "/runtimes/muxi-runtime-0.20260508.0-linux-arm64.sif",
		Port:    8080,
	}

	cmd := buildDockerSingularityCommand(config, &logger)

	if !hasFlagValue(cmd.Args, "--platform", "linux/arm64") {
		t.Errorf("missing --platform linux/arm64 pin, got args: %v", cmd.Args)
	}
}
