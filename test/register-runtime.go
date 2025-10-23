// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/muxi-ai/server/pkg/runtime"
)

func main() {
	// Get runtime directory
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	runtimesDir := filepath.Join(home, ".muxi", "server", "runtimes")
	registryPath := filepath.Join(runtimesDir, "registry.json")

	// Create registry
	registry := runtime.NewRegistry(registryPath)
	if err := registry.Load(); err != nil {
		fmt.Printf("Warning: Failed to load registry: %v (creating new)\n", err)
	}

	// Create downloader
	downloader := runtime.NewDownloader(runtimesDir, registry)

	// Register the SIF
	version := "0.1.0"
	sifPath := downloader.GetSIFPath(version)

	fmt.Printf("Registering runtime %s...\n", version)
	fmt.Printf("SIF path: %s\n", sifPath)

	if err := downloader.Register(sifPath, version); err != nil {
		fmt.Fprintf(os.Stderr, "Error registering runtime: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Runtime registered successfully!")

	// Show registry
	versions := registry.List()
	fmt.Printf("\nAvailable runtimes: %v\n", versions)
}
