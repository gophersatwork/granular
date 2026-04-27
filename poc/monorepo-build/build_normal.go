package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// NormalBuilder is a naive builder that builds everything
type NormalBuilder struct {
	rootDir string
}

// Package represents a buildable package
type Package struct {
	Name     string
	Path     string
	IsShared bool
}

// NewNormalBuilder creates a new normal builder
func NewNormalBuilder(rootDir string) *NormalBuilder {
	return &NormalBuilder{
		rootDir: rootDir,
	}
}

// Build builds all packages without any optimization
func (b *NormalBuilder) Build() error {
	fmt.Println("=== Normal Build (Naive) ===")
	fmt.Println("Building ALL packages without any caching...")
	fmt.Println()

	packages := b.getPackages()
	totalStart := time.Now()

	// Build shared packages first
	sharedPackages := filterShared(packages, true)
	for _, pkg := range sharedPackages {
		if err := b.buildPackage(pkg); err != nil {
			return fmt.Errorf("failed to build %s: %w", pkg.Name, err)
		}
	}

	// Build services
	servicePackages := filterShared(packages, false)
	for _, pkg := range servicePackages {
		if err := b.buildPackage(pkg); err != nil {
			return fmt.Errorf("failed to build %s: %w", pkg.Name, err)
		}
	}

	totalDuration := time.Since(totalStart)

	fmt.Println()
	fmt.Println("=== Build Summary ===")
	fmt.Printf("Total packages built: %d\n", len(packages))
	fmt.Printf("Total time: %v\n", totalDuration)
	fmt.Printf("Average time per package: %v\n", totalDuration/time.Duration(len(packages)))

	return nil
}

// buildPackage builds a single package
func (b *NormalBuilder) buildPackage(pkg Package) error {
	start := time.Now()
	fmt.Printf("Building %s... ", pkg.Name)

	// Determine output path
	outputDir := filepath.Join(b.rootDir, "bin")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	var outputPath string
	if pkg.IsShared {
		// For shared packages, just compile (no binary output)
		cmd := exec.Command("go", "build", "-v", "./"+pkg.Path)
		cmd.Dir = b.rootDir
		cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(b.rootDir, "go.work"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("FAILED\n")
			fmt.Printf("Error output: %s\n", string(output))
			return err
		}
	} else {
		// For services, build binaries
		outputPath = filepath.Join(outputDir, pkg.Name)
		cmd := exec.Command("go", "build", "-v", "-o", outputPath, "./"+pkg.Path)
		cmd.Dir = b.rootDir
		cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(b.rootDir, "go.work"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("FAILED\n")
			fmt.Printf("Error output: %s\n", string(output))
			return err
		}
	}

	duration := time.Since(start)
	fmt.Printf("DONE (%v)\n", duration)

	return nil
}

// getPackages returns all packages in the monorepo
func (b *NormalBuilder) getPackages() []Package {
	return []Package{
		// Shared packages
		{Name: "models", Path: "shared/models", IsShared: true},
		{Name: "utils", Path: "shared/utils", IsShared: true},

		// Services
		{Name: "api", Path: "services/api", IsShared: false},
		{Name: "worker", Path: "services/worker", IsShared: false},
		{Name: "admin", Path: "services/admin", IsShared: false},
	}
}

// filterShared filters packages by shared status
func filterShared(packages []Package, isShared bool) []Package {
	result := make([]Package, 0)
	for _, pkg := range packages {
		if pkg.IsShared == isShared {
			result = append(result, pkg)
		}
	}
	return result
}

func main() {
	// Get the directory where this script is located
	rootDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	builder := NewNormalBuilder(rootDir)
	if err := builder.Build(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}
}
