package runtime

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/rs/zerolog/log"
)

// ValidateRuntimeAvailable checks if the required container runtime is available
// - On Linux: Requires Singularity
// - On macOS/Windows: Requires Docker
func ValidateRuntimeAvailable() error {
	if runtime.GOOS == "linux" {
		return validateSingularity()
	}
	return validateDocker()
}

// validateSingularity checks if Singularity is installed on Linux
func validateSingularity() error {
	if _, err := exec.LookPath("singularity"); err != nil {
		return fmt.Errorf(`Singularity not found. 

To install on Ubuntu/Debian:
  sudo apt update
  sudo apt install -y singularity-container

To install on other Linux distributions:
  https://sylabs.io/guides/latest/admin-guide/installation.html

Error: %w`, err)
	}

	log.Debug().
		Str("platform", "linux").
		Msg("✓ Singularity available for native execution")

	return nil
}

// validateDocker checks if Docker is installed and pulls runtime-runner if needed
func validateDocker() error {
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
	return ensureRuntimeRunnerImage()
}

// ensureRuntimeRunnerImage checks if the runtime-runner Docker image exists
// If not, it attempts to pull it
func ensureRuntimeRunnerImage() error {
	// Using GitHub Container Registry (like faissx)
	runtimeImage := "ghcr.io/muxi-ai/runtime-runner:latest"

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

// GetRuntimeInfo returns information about the available runtime
func GetRuntimeInfo() RuntimeEnvironment {
	if runtime.GOOS == "linux" {
		singularityPath, _ := exec.LookPath("singularity")
		return RuntimeEnvironment{
			Platform:    "linux",
			RuntimeType: "singularity",
			RuntimePath: singularityPath,
			Native:      true,
		}
	}

	dockerPath, _ := exec.LookPath("docker")
	return RuntimeEnvironment{
		Platform:    runtime.GOOS,
		RuntimeType: "docker-wrapper",
		RuntimePath: dockerPath,
		Native:      false,
		WrapperImage: "ghcr.io/muxi-ai/runtime-runner:latest",
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
		return fmt.Sprintf("Native Singularity on %s (%s)", r.Platform, r.RuntimePath)
	}
	return fmt.Sprintf("Docker-wrapped Singularity on %s (via %s)", r.Platform, r.WrapperImage)
}
