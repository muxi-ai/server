package runtime

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/muxi-ai/server/pkg/config"
	"github.com/rs/zerolog/log"
)

// ValidateRuntimeAvailable checks if the required container runtime is available
// - On Linux: Requires Singularity
// - On macOS/Windows: Requires Docker
//
// runnerImage is the runtime-runner Docker image name the caller wants
// validated on non-Linux hosts. Empty falls back to
// config.DefaultRuntimeRunnerImage so callers that don't yet thread their
// config.Runtime.RuntimeRunnerImage through still get working behavior,
// but operators who override the field get the override honored — which
// closes the gap against the Docker spawn path that already reads the
// configured image.
//
// Not yet wired into startup today. cmdInit performs its own
// Docker-available / runtime-runner-image-present checks inline
// (cmd/server/commands.go) and cmdStart currently performs no
// pre-flight runtime validation — an operator with a broken Docker
// or an absent custom runtime-runner image gets a spawn-time error
// on first deploy instead of a clean startup error. Keeping this
// function correctly-shaped (config-aware image parameter, proper
// fallback) so whichever command wires it next doesn't need to
// re-design the signature. Intended eventual call site: just after
// config.Load in cmd/server/commands.go's cmdStart.
func ValidateRuntimeAvailable(runnerImage string) error {
	if runtime.GOOS == "linux" {
		return validateSingularity()
	}
	if runnerImage == "" {
		runnerImage = config.DefaultRuntimeRunnerImage
	}
	return validateDocker(runnerImage)
}

func getSingularityPath() (string, error) {
	if path, err := exec.LookPath("apptainer"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("singularity"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("neither apptainer nor singularity found")
}

// validateSingularity checks if Singularity or Apptainer is installed on Linux
func validateSingularity() error {
	singularityPath, err := getSingularityPath()
	if err != nil {
		return fmt.Errorf(`Singularity/Apptainer not found.

To install on Ubuntu/Debian:
  sudo apt update && sudo apt install -y apptainer

To install on other Linux distributions:
  https://apptainer.org/docs/admin/main/installation.html

Error: %w`, err)
	}

	log.Debug().
		Str("platform", "linux").
		Str("binary", singularityPath).
		Msg("✓ Singularity/Apptainer available for native execution")

	return nil
}

// validateDocker checks if Docker is installed and pulls runtime-runner if needed.
// runnerImage names the specific image to check/pull so an operator-configured
// override is honored rather than silently falling back to the default.
func validateDocker(runnerImage string) error {
	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf(`Docker not found.

To install Docker Desktop:
  macOS:   brew install --cask docker
           OR download from https://docker.com/products/docker-desktop
  
  Windows: Download from https://docker.com/products/docker-desktop

Error: %w`, err)
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(`Docker is installed but not running.

Please start Docker Desktop and try again.

Error: %w`, err)
	}

	log.Debug().
		Str("platform", runtime.GOOS).
		Msg("✓ Docker available")

	// Check if runtime-runner image exists, pull if not
	return ensureRuntimeRunnerImage(runnerImage)
}

// ensureRuntimeRunnerImage checks if the runtime-runner Docker image exists.
// If not, it attempts to pull it. runtimeImage is passed in so an operator's
// runtime.runtime_runner_image override is validated instead of the default.
func ensureRuntimeRunnerImage(runtimeImage string) error {
	if runtimeImage == "" {
		runtimeImage = config.DefaultRuntimeRunnerImage
	}

	// Check if image exists locally
	cmd := exec.Command("docker", "image", "inspect", runtimeImage)
	if err := cmd.Run(); err == nil {
		// Image exists
		log.Debug().
			Str("image", runtimeImage).
			Msg("✓ Runtime runner image available")
		return nil
	}

	// Image doesn't exist, try to pull it
	log.Info().
		Str("image", runtimeImage).
		Msg("Pulling runtime runner image (first time only)...")

	cmd = exec.Command("docker", "pull", runtimeImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(`Failed to pull runtime runner image.

This image is required to run MUXI formations on macOS/Windows.

You can build it manually:
  cd test/dummy-sif
  ./build-runtime-runner.sh

Or wait for the official image to be published to Docker Hub.

Error: %w
Output: %s`, err, string(output))
	}

	log.Info().
		Str("image", runtimeImage).
		Msg("✓ Runtime runner image pulled successfully")

	return nil
}

// GetRuntimeInfo returns information about the available runtime.
//
// wrapperImage is the runtime-runner Docker image name to report in
// the returned RuntimeEnvironment on non-Linux hosts. Empty falls
// back to config.DefaultRuntimeRunnerImage so callers that don't
// have the configured override handy (or are on Linux, where the
// field is ignored) still get a populated struct. Threading the
// configured image through prevents the returned value from
// misreporting the active image when an operator has overridden
// runtime.runtime_runner_image in config.yaml — a real concern if
// this struct ever feeds into logs, health endpoints, or a status
// command.
//
// Like ValidateRuntimeAvailable above, this function has no live
// callers today but is kept correctly shaped for the first consumer
// that needs it (most likely a /rpc/server/status field).
func GetRuntimeInfo(wrapperImage string) RuntimeEnvironment {
	if runtime.GOOS == "linux" {
		singularityPath, _ := getSingularityPath()
		return RuntimeEnvironment{
			Platform:    "linux",
			RuntimeType: "singularity",
			RuntimePath: singularityPath,
			Native:      true,
		}
	}

	if wrapperImage == "" {
		wrapperImage = config.DefaultRuntimeRunnerImage
	}
	dockerPath, _ := exec.LookPath("docker")
	return RuntimeEnvironment{
		Platform:     runtime.GOOS,
		RuntimeType:  "docker-wrapper",
		RuntimePath:  dockerPath,
		Native:       false,
		WrapperImage: wrapperImage,
	}
}

// RuntimeEnvironment describes the available container runtime environment
type RuntimeEnvironment struct {
	Platform     string // "linux", "darwin", "windows"
	RuntimeType  string // "singularity", "docker-wrapper"
	RuntimePath  string // Path to singularity or docker binary
	Native       bool   // True if using native Singularity, false if Docker wrapper
	WrapperImage string // Docker image name (only for docker-wrapper)
}

// String returns a human-readable description
func (r RuntimeEnvironment) String() string {
	if r.Native {
		return fmt.Sprintf("Native Singularity/Apptainer on %s (%s)", r.Platform, r.RuntimePath)
	}
	return fmt.Sprintf("Docker-wrapped Singularity on %s (via %s)", r.Platform, r.WrapperImage)
}
