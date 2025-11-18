package granular_test

import (
	"errors"
	"fmt"

	"github.com/gophersatwork/granular"
	"github.com/spf13/afero"
)

// Example_basic demonstrates the basic usage of the cache with the fluent API.
func Example_basic() {
	// Create an in-memory cache for this example
	cache := granular.OpenTemp()

	// Build a cache key
	key := cache.Key().
		String("input", "test data").
		Version("1.0.0").
		Build()

	// Check cache
	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
		fmt.Println("Cache miss - computing result")

		// Do expensive work...
		// Store result
		cache.Put(key).
			Meta("status", "computed").
			Commit()
	} else if err == nil && result != nil {
		fmt.Println("Cache hit!")
		fmt.Println("Status:", result.Meta("status"))
	}

	// Second attempt should hit
	result, err = cache.Get(key)
	if err == nil && result != nil {
		fmt.Println("Cache hit!")
		fmt.Println("Status:", result.Meta("status"))
	}

	// Output:
	// Cache miss - computing result
	// Cache hit!
	// Status: computed
}

// Example_buildSystem demonstrates using the cache for a build system.
func Example_buildSystem() {
	// Setup test filesystem
	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "main.go", []byte("package main"), 0o644)
	afero.WriteFile(memFs, "utils.go", []byte("package main"), 0o644)

	cache, _ := granular.Open("", granular.WithFs(memFs))

	// Build cache key based on source files
	key := cache.Key().
		File("main.go").
		File("utils.go").
		String("GOOS", "linux").
		String("GOARCH", "amd64").
		Build()

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
		fmt.Println("Building from source...")

		// Simulate build
		afero.WriteFile(memFs, "app", []byte("binary"), 0o755)

		// Cache the binary
		cache.Put(key).
			File("binary", "app").
			Meta("build_time", "1.23s").
			Commit()

		fmt.Println("Build complete")
	} else if err == nil && result != nil {
		fmt.Println("Using cached binary")
		fmt.Println("Previous build time:", result.Meta("build_time"))
	}

	// Output:
	// Building from source...
	// Build complete
}

// Example_codeGeneration demonstrates caching code generation results.
func Example_codeGeneration() {
	// Setup test filesystem
	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "schema.proto", []byte("syntax = \"proto3\";"), 0o644)

	cache, _ := granular.Open("", granular.WithFs(memFs))

	// Cache key based on schema and generator version
	key := cache.Key().
		File("schema.proto").
		Version("protoc-3.21.12").
		String("lang", "go").
		Build()

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
		fmt.Println("Generating code from schema...")

		// Simulate code generation
		generatedCode := []byte("// Generated code\npackage pb")

		// Cache the generated code
		cache.Put(key).
			Bytes("generated", generatedCode).
			Meta("timestamp", "2024-01-01").
			Commit()

		fmt.Println("Code generation complete")
	} else if err == nil && result != nil {
		fmt.Println("Using cached generated code")
		code := result.Bytes("generated")
		fmt.Printf("Generated %d bytes of code\n", len(code))
	}

	// Output:
	// Generating code from schema...
	// Code generation complete
}

// Example_multiFile demonstrates caching multiple output files.
func Example_multiFile() {
	// Setup test filesystem
	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "input.txt", []byte("data"), 0o644)
	afero.WriteFile(memFs, "output1.txt", []byte("result1"), 0o644)
	afero.WriteFile(memFs, "output2.json", []byte(`{"key":"value"}`), 0o644)

	cache, _ := granular.Open("", granular.WithFs(memFs))

	key := cache.Key().File("input.txt").Build()

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
		fmt.Println("Processing input...")

		// Cache multiple outputs
		cache.Put(key).
			File("text", "output1.txt").
			File("json", "output2.json").
			Meta("count", "2").
			Commit()

		fmt.Println("Processing complete")
	} else if err == nil && result != nil {
		fmt.Println("Using cached outputs")
		fmt.Println("Output count:", result.Meta("count"))
		fmt.Println("Has text:", result.HasFile("text"))
		fmt.Println("Has json:", result.HasFile("json"))
	}

	// Output:
	// Processing input...
	// Processing complete
}

// Example_cacheManagement demonstrates cache statistics and management.
func Example_cacheManagement() {
	cache := granular.OpenTemp()

	// Add some entries
	for i := 0; i < 3; i++ {
		key := cache.Key().
			String("item", fmt.Sprintf("item%d", i)).
			Build()

		cache.Put(key).
			Meta("index", fmt.Sprintf("%d", i)).
			Commit()
	}

	// Get statistics
	stats, _ := cache.Stats()
	fmt.Printf("Cache has %d entries\n", stats.Entries)

	// List entries
	entries, _ := cache.Entries()
	fmt.Printf("Listed %d entries\n", len(entries))

	// Check if key exists
	key := cache.Key().String("item", "item0").Build()
	fmt.Println("Has item0:", cache.Has(key))

	// Clear cache
	cache.Clear()
	stats, _ = cache.Stats()
	fmt.Printf("After clear: %d entries\n", stats.Entries)

	// Output:
	// Cache has 3 entries
	// Listed 3 entries
	// Has item0: true
	// After clear: 0 entries
}

// Example_globPattern demonstrates using glob patterns with the cache.
func Example_globPattern() {
	// Setup test filesystem
	memFs := afero.NewMemMapFs()
	memFs.MkdirAll("src", 0o755)
	afero.WriteFile(memFs, "src/file1.go", []byte("package main"), 0o644)
	afero.WriteFile(memFs, "src/file2.go", []byte("package main"), 0o644)
	afero.WriteFile(memFs, "src/file3.txt", []byte("text"), 0o644)

	cache, _ := granular.Open("", granular.WithFs(memFs))

	// Cache key based on all Go files
	key := cache.Key().
		Glob("src/*.go").
		Build()

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
		fmt.Println("Processing Go files...")

		cache.Put(key).
			Meta("file_count", "2").
			Commit()

		fmt.Println("Processing complete")
	} else if err == nil && result != nil {
		fmt.Println("Using cached result")
		fmt.Println("Processed files:", result.Meta("file_count"))
	}

	// Output:
	// Processing Go files...
	// Processing complete
}
