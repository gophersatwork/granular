package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

// SmartBuilder is an intelligent builder that uses Granular for caching
type SmartBuilder struct {
	rootDir      string
	cache        *granular.Cache
	dependencies map[string][]string // package -> list of dependencies
	fs           afero.Fs
}

// BuildResult represents the result of building a package
type BuildResult struct {
	PackageName string
	OutputPath  string
	BuildTime   time.Time
	Duration    time.Duration
	FromCache   bool
}

// NewSmartBuilder creates a new smart builder with Granular caching
func NewSmartBuilder(rootDir string) (*SmartBuilder, error) {
	cacheDir := filepath.Join(rootDir, ".cache", "granular")

	fs := afero.NewOsFs()
	cache, err := granular.Open(cacheDir, granular.WithFs(fs))
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Define package dependencies
	dependencies := map[string][]string{
		"models": {},
		"utils":  {},
		"api":    {"models", "utils"},
		"worker": {"models", "utils"},
		"admin":  {"models", "utils"},
	}

	return &SmartBuilder{
		rootDir:      rootDir,
		cache:        cache,
		dependencies: dependencies,
		fs:           fs,
	}, nil
}

// Build builds all packages with intelligent caching
func (b *SmartBuilder) Build() error {
	fmt.Println("=== Smart Build (Granular Caching) ===")
	fmt.Println("Analyzing dependencies and building only changed packages...")
	fmt.Println()

	packages := b.getPackages()
	totalStart := time.Now()

	var builtCount, cachedCount int
	results := make([]BuildResult, 0)

	// Build shared packages first
	sharedPackages := filterSharedPackages(packages, true)
	for _, pkg := range sharedPackages {
		result, err := b.buildPackage(pkg)
		if err != nil {
			return fmt.Errorf("failed to build %s: %w", pkg.Name, err)
		}
		results = append(results, result)
		if result.FromCache {
			cachedCount++
		} else {
			builtCount++
		}
	}

	// Build services
	servicePackages := filterSharedPackages(packages, false)
	for _, pkg := range servicePackages {
		result, err := b.buildPackage(pkg)
		if err != nil {
			return fmt.Errorf("failed to build %s: %w", pkg.Name, err)
		}
		results = append(results, result)
		if result.FromCache {
			cachedCount++
		} else {
			builtCount++
		}
	}

	totalDuration := time.Since(totalStart)

	fmt.Println()
	fmt.Println("=== Build Summary ===")
	fmt.Printf("Total packages: %d\n", len(packages))
	fmt.Printf("Built from source: %d\n", builtCount)
	fmt.Printf("Restored from cache: %d\n", cachedCount)
	fmt.Printf("Cache hit rate: %.1f%%\n", float64(cachedCount)/float64(len(packages))*100)
	fmt.Printf("Total time: %v\n", totalDuration)

	if builtCount > 0 && cachedCount > 0 {
		fmt.Printf("Time saved by caching: ~%v\n", b.estimateTimeSaved(results))
	}

	return nil
}

// buildPackage builds a single package with caching
func (b *SmartBuilder) buildPackage(pkg PackageInfo) (BuildResult, error) {
	start := time.Now()
	result := BuildResult{
		PackageName: pkg.Name,
		BuildTime:   start,
	}

	// Calculate cache key based on package contents and dependencies
	key, err := b.calculateCacheKey(pkg)
	if err != nil {
		return result, fmt.Errorf("failed to calculate cache key: %w", err)
	}

	fmt.Printf("Building %s... ", pkg.Name)

	// Check cache
	cached := b.cache.Get(key)
	if cached != nil {
		// Cache hit!
		duration := time.Since(start)

		// Restore binary if this is a service
		if !pkg.IsShared {
			binaryData := cached.Bytes("binary")
			if len(binaryData) > 0 {
				outputDir := filepath.Join(b.rootDir, "bin")
				os.MkdirAll(outputDir, 0o755)
				outputPath := filepath.Join(outputDir, pkg.Name)
				if err := afero.WriteFile(b.fs, outputPath, binaryData, 0o755); err != nil {
					fmt.Printf("FAILED (restore error)\n")
					return result, err
				}
				result.OutputPath = outputPath
			}
		}

		// Parse stored duration
		if buildDur := cached.Meta("duration"); buildDur != "" {
			if d, err := time.ParseDuration(buildDur); err == nil {
				result.Duration = d
			}
		}

		result.FromCache = true
		fmt.Printf("CACHED (%v)\n", duration)
		return result, nil
	}

	// Cache miss - build from source
	buildResult, err := b.executeBuild(pkg)
	if err != nil {
		fmt.Printf("FAILED\n")
		return result, err
	}

	// Store in cache
	writer := b.cache.Put(key).
		Meta("package", pkg.Name).
		Meta("duration", buildResult.Duration.String()).
		Meta("build_time", buildResult.BuildTime.Format(time.RFC3339))

	// Store binary if this is a service
	if !pkg.IsShared && buildResult.OutputPath != "" {
		binaryData, err := afero.ReadFile(b.fs, buildResult.OutputPath)
		if err == nil {
			writer = writer.Bytes("binary", binaryData)
		}
	}

	writer.Commit()

	fmt.Printf("BUILT (%v)\n", buildResult.Duration)
	return buildResult, nil
}

// executeBuild performs the actual build
func (b *SmartBuilder) executeBuild(pkg PackageInfo) (BuildResult, error) {
	buildStart := time.Now()
	result := BuildResult{
		PackageName: pkg.Name,
		BuildTime:   buildStart,
		FromCache:   false,
	}

	// Determine output path
	outputDir := filepath.Join(b.rootDir, "bin")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return result, err
	}

	if pkg.IsShared {
		// For shared packages, just compile
		cmd := exec.Command("go", "build", "-v", "./"+pkg.Path)
		cmd.Dir = b.rootDir
		cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(b.rootDir, "go.work"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			return result, fmt.Errorf("build error: %s", string(output))
		}
	} else {
		// For services, build binaries
		outputPath := filepath.Join(outputDir, pkg.Name)
		result.OutputPath = outputPath

		cmd := exec.Command("go", "build", "-v", "-o", outputPath, "./"+pkg.Path)
		cmd.Dir = b.rootDir
		cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(b.rootDir, "go.work"))

		output, err := cmd.CombinedOutput()
		if err != nil {
			return result, fmt.Errorf("build error: %s", string(output))
		}
	}

	result.Duration = time.Since(buildStart)
	return result, nil
}

// calculateCacheKey generates a cache key based on source files and dependencies
func (b *SmartBuilder) calculateCacheKey(pkg PackageInfo) (granular.Key, error) {
	// Hash all source files in the package
	pkgHash, err := b.hashPackageFiles(pkg.Path)
	if err != nil {
		return granular.Key{}, err
	}

	// Include dependency hashes
	depHashes := make([]string, 0)
	for _, dep := range b.dependencies[pkg.Name] {
		depPath := b.getPackagePath(dep)
		depHash, err := b.hashPackageFiles(depPath)
		if err != nil {
			return granular.Key{}, err
		}
		depHashes = append(depHashes, depHash)
	}

	// Combine hashes
	combinedHash := pkgHash
	if len(depHashes) > 0 {
		combinedHash += ":" + strings.Join(depHashes, ":")
	}

	// Create Granular key
	return b.cache.Key().
		String("package", pkg.Name).
		String("hash", combinedHash).
		String("go_version", "1.24").
		Build(), nil
}

// hashPackageFiles computes a hash of all Go files in a package
func (b *SmartBuilder) hashPackageFiles(pkgPath string) (string, error) {
	fullPath := filepath.Join(b.rootDir, pkgPath)
	hasher := sha256.New()

	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only hash .go files
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			file, err := b.fs.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(hasher, file); err != nil {
				return err
			}

			// Include file modification time in hash
			fmt.Fprintf(hasher, "%s:%d", path, info.ModTime().Unix())
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// getPackagePath returns the path for a package name
func (b *SmartBuilder) getPackagePath(name string) string {
	packages := map[string]string{
		"models": "shared/models",
		"utils":  "shared/utils",
		"api":    "services/api",
		"worker": "services/worker",
		"admin":  "services/admin",
	}
	return packages[name]
}

// PackageInfo represents package information
type PackageInfo struct {
	Name     string
	Path     string
	IsShared bool
}

// getPackages returns all packages in the monorepo
func (b *SmartBuilder) getPackages() []PackageInfo {
	return []PackageInfo{
		// Shared packages
		{Name: "models", Path: "shared/models", IsShared: true},
		{Name: "utils", Path: "shared/utils", IsShared: true},

		// Services
		{Name: "api", Path: "services/api", IsShared: false},
		{Name: "worker", Path: "services/worker", IsShared: false},
		{Name: "admin", Path: "services/admin", IsShared: false},
	}
}

// filterSharedPackages filters packages by shared status
func filterSharedPackages(packages []PackageInfo, isShared bool) []PackageInfo {
	result := make([]PackageInfo, 0)
	for _, pkg := range packages {
		if pkg.IsShared == isShared {
			result = append(result, pkg)
		}
	}
	return result
}

// estimateTimeSaved calculates time saved by caching
func (b *SmartBuilder) estimateTimeSaved(results []BuildResult) time.Duration {
	var saved time.Duration
	for _, result := range results {
		if result.FromCache {
			// Use the actual build duration that was saved
			if result.Duration > 0 {
				saved += result.Duration
			} else {
				// Assume average build time of 2 seconds for cached items
				saved += 2 * time.Second
			}
		}
	}
	return saved
}

func main() {
	// Get the directory where this script is located
	rootDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	builder, err := NewSmartBuilder(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating builder: %v\n", err)
		os.Exit(1)
	}

	if err := builder.Build(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}
}
