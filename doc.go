/*
	Package granular provides a high-performance incremental file cache library for Go applications.

It offers deterministic, content-based caching optimized for local filesystem operations.

# Overview

granular is a lightweight, high-performance file caching library that provides deterministic,
content-based caching. It is inspired by Zig's build cache architecture and optimized for
local filesystem operations without distributed system complexity.

# Core Architecture

granular uses a two-level structure for its cache:
  - manifests/ - Metadata about cached computations
  - objects/ - Actual cached artifacts

The cache uses content-addressed storage with fast hashing (xxHash by default) to create
deterministic cache keys, and a manifest system with JSON files tracking inputs, outputs,
and metadata.

# Key Features

  - Content-Addressed Storage: Uses fast hashing (xxHash by default) for deterministic cache keys
  - Manifest System: JSON files tracking inputs, outputs, and metadata
  - Flexible Inputs: Support for files, directories, glob patterns, and raw data
  - Memory Efficient: Buffer pooling and lazy evaluation
  - Concurrent Access: Thread-safe operations with proper locking

# Basic Usage

Creating a cache:

	cache, err := granular.New(".cache")
	if err != nil {
	    log.Fatalf("Failed to create cache: %v", err)
	}

Defining a cache key:

	key := granular.Key{
	    Inputs: []granular.Input{
	        granular.FileInput{Path: "main.go"},
	        granular.GlobInput{Pattern: "*.json"},
	    },
	    Extra: map[string]string{"version": "1.0.0"},
	}

Checking for a cache hit:

	result, hit, err := cache.Get(key)
	if err != nil && !errors.Is(err, granular.ErrCacheMiss) {
	    log.Fatalf("Cache error: %v", err)
	}

	if hit {
	    fmt.Println("Cache hit!")
	    // Use cached result
	    fmt.Printf("Cached file: %s\n", result.Path)
	} else {
	    fmt.Println("Cache miss, computing result...")

	    // Perform your computation here
	    // ...

	    // Store the result in the cache
	    result := granular.Result{
	        Path: outputFile,
	        Metadata: map[string]string{
	            "summary": "Computation complete",
	        },
	    }

	    err = cache.Store(key, result)
	    if err != nil {
	        log.Fatalf("Failed to store in cache: %v", err)
	    }
	}

# Input Types

granular supports several input types:

FileInput - Single file input:

	input := granular.FileInput{Path: "path/to/file.txt"}

GlobInput - Multiple files matching a pattern:

	input := granular.GlobInput{Pattern: "src/*.go"}

DirectoryInput - All files in a directory (recursive):

	input := granular.DirectoryInput{
	    Path: "src/",
	    Exclude: []string{"*.tmp", "*.log"},
	}

RawInput - Raw data:

	input := granular.RawInput{
	    Data: []byte("raw data"),
	    Name: "config",
	}

# Configuration Options

granular can be configured with various options:

	cache, err := granular.New(
	    ".cache",
	    granular.WithHashFunc(myCustomHashFunc),
	    granular.WithFs(myCustomFs),
	)

# Performance Considerations

  - Hash Function: xxHash is used by default for its speed, but you can provide a custom hash function
  - Buffer Pooling: Reuses buffers to reduce memory allocations

# File Structure

The cache uses the following directory structure:

	.cache/
	├── manifests/
	│   └── [first 2 chars of hash]/
	│       └── [full hash].json
	└── objects/
	    └── [first 2 chars of hash]/
	        └── [full hash]/
	            └── [cached files]

# Error Handling

The package defines several error types:

  - ErrCacheMiss: Returned when a cache key is not found
  - ErrInvalidKey: Returned when a key is invalid

Always check for these errors when using the cache:

	result, hit, err := cache.Get(key)
	if err != nil {
	    if errors.Is(err, granular.ErrCacheMiss) {
	        // Handle cache miss
	    } else {
	        // Handle other errors
	    }
	}
*/
package granular
