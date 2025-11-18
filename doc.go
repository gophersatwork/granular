/*
Package granular provides a high-performance, content-based file cache for Go applications.

It offers deterministic caching optimized for local filesystem operations, with a fluent
builder API inspired by Zig's build cache architecture.

# Overview

Granular is a lightweight, high-performance caching library that provides deterministic,
content-based caching. Cache keys are built using actual file content (not timestamps),
making it perfect for build systems, code generation, and incremental computation.

# Core Architecture

Granular uses a two-level structure for its cache:
  - manifests/ - JSON metadata about cached computations
  - objects/ - Actual cached artifacts

The cache uses content-addressed storage with fast hashing (xxHash by default) to create
deterministic cache keys.

# Key Features

  - Fluent Builder API: Self-documenting, minimal & opinionated design
  - Multi-File Support: Cache multiple output files and data in a single entry
  - Content-Based Keys: Hash actual file content, not timestamps
  - Cache Management: Built-in stats, pruning, and introspection
  - Memory Efficient: Buffer pooling and efficient file I/O
  - Concurrent Safe: Thread-safe operations with proper locking

# Quick Start

Opening a cache:

	cache, err := granular.Open(".cache")
	if err != nil {
	    log.Fatalf("Failed to open cache: %v", err)
	}

Building a cache key with the fluent API:

	key := cache.Key().
	    File("main.go").
	    Glob("*.json").
	    Version("1.0.0").
	    Build()

Checking for a cache hit:

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
	    // Cache miss - do expensive work
	    output := runExpensiveTask()

	    // Store result
	    cache.Put(key).
	        File("output", output).
	        Meta("duration", "123ms").
	        Commit()
	} else if err != nil {
	    // Handle errors (validation, I/O, corruption)
	    log.Fatal(err)
	} else {
	    // Cache hit! Use cached output
	    output := result.File("output")
	}

# Building Cache Keys

The KeyBuilder provides a fluent API for constructing cache keys:

Single file input:

	key := cache.Key().File("src/main.go").Build()

Glob pattern (supports wildcards and recursive matching):

	key := cache.Key().Glob("*.go").Build()

Directory with exclusions (matches basenames only):

	key := cache.Key().Dir("configs", "*.tmp", "*.log").Build()

Raw byte data:

	key := cache.Key().Bytes([]byte("data")).Build()

Metadata and versioning:

	key := cache.Key().
	    File("schema.proto").
	    String("generator", "protoc").
	    Version("2.0.1").
	    Env("GOOS").
	    Build()

# Storing and Retrieving Results

Store a single file:

	cache.Put(key).
	    File("output", "./result.txt").
	    Meta("build_time", "123ms").
	    Commit()

Store multiple files and data:

	cache.Put(key).
	    File("binary", "./app").
	    File("symbols", "./app.sym").
	    Bytes("logs", logData).
	    Meta("compiler", "go1.21").
	    Commit()

Retrieve cached results:

	result, err := cache.Get(key)
	if err == nil && result != nil {
	    // Access files
	    binary := result.File("binary")
	    symbols := result.File("symbols")

	    // Access data
	    logs := result.Bytes("logs")

	    // Access metadata
	    compiler := result.Meta("compiler")

	    // Copy file back to working directory
	    result.CopyFile("binary", "./app")
	}

# Cache Management

Get statistics:

	stats, err := cache.Stats()
	fmt.Printf("Entries: %d, Total Size: %d bytes\n",
	    stats.Entries, stats.TotalSize)

Prune old entries:

	removed, err := cache.Prune(7 * 24 * time.Hour)
	fmt.Printf("Removed %d old entries\n", removed)

List all entries:

	entries, err := cache.Entries()
	for _, entry := range entries {
	    fmt.Printf("Key: %s, Age: %v\n",
	        entry.KeyHash, time.Since(entry.CreatedAt))
	}

Other operations:

	// Check if key exists
	if cache.Has(key) { ... }

	// Delete specific entry
	cache.Delete(key)

	// Clear entire cache
	cache.Clear()

# Configuration Options

Granular works with zero configuration, but offers options when needed:

Custom filesystem (for testing):

	cache, err := granular.Open(".cache",
	    granular.WithFs(afero.NewMemMapFs()))

In-memory cache for testing:

	cache := granular.OpenTemp()

Custom hash function:

	cache, err := granular.Open(".cache",
	    granular.WithHashFunc(myHashFunc))

# Error Handling

Granular uses eager validation with error accumulation. Validation happens during key
building, but errors are only surfaced when you call Get() or Commit().

Cache miss detection using sentinel error:

	result, err := cache.Get(key)
	if errors.Is(err, granular.ErrCacheMiss) {
	    // Cache miss - compute and cache result
	} else if err != nil {
	    // Other errors (validation, I/O, corruption)
	    return err
	}

Validation errors are collected and returned:

	key := cache.Key().
	    File("missing.txt").     // Validates immediately
	    Glob("bad[pattern").     // Validates immediately
	    Build()                  // Always succeeds (no error)

	result, err := cache.Get(key)  // Errors surface here
	var validationErr *granular.ValidationError
	if errors.As(err, &validationErr) {
	    for _, e := range validationErr.Errors {
	        fmt.Printf("- %v\n", e)
	    }
	}

Fail-fast vs accumulate-all-errors:

	// Default: stop after first error (fail-fast)
	cache, _ := granular.Open(".cache")

	// Development: collect all errors
	cache, _ := granular.Open(".cache", granular.WithAccumulateErrors())

# File Structure

The cache uses the following directory structure:

	.cache/
	├── manifests/
	│   └── ab/
	│       └── abcd1234....json (cache metadata)
	└── objects/
	    └── ab/
	        └── abcd1234.../
	            ├── output.txt (cached files)
	            └── data.dat (cached byte data)

# Performance Considerations

  - xxHash64: Fast, non-cryptographic hash by default
  - Buffer Pooling: Reuses buffers to reduce GC pressure
  - Two-Level Sharding: Efficient filesystem operations
  - No Global State: Fully concurrent-safe

# Common Use Cases

Build system caching:

	key := cache.Key().
	    Glob("*.go").
	    String("GOOS", runtime.GOOS).
	    Build()

Code generation:

	key := cache.Key().
	    File("schema.proto").
	    Version("protoc-3.21").
	    String("lang", "go").
	    Build()

Data processing pipelines:

	key := cache.Key().
	    File("input.csv").
	    String("transform", "normalize").
	    Build()
*/
package granular
