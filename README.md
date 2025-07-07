# granular: High-Performance Incremental File Cache for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/gophersatwork/granular.svg)](https://pkg.go.dev/github.com/gophersatwork/granular)
[![Go Report Card](https://goreportcard.com/badge/github.com/gophersatwork/granular)](https://goreportcard.com/report/github.com/gophersatwork/granular)

`granular` is a lightweight, high-performance file caching library for Go applications that provides deterministic, content-based caching.

<img src="assets/granular-alpha.png" width="400" height="400">

## Features

- **Content-Addressed Storage**: Uses fast hashing (`xxHash` by default) to create deterministic cache keys
- **Manifest System**: JSON files tracking inputs, outputs, and metadata
- **Flexible Inputs**: Support for files, directories, glob patterns, and raw data
- **Memory Efficient**: Buffer pooling and lazy evaluation
- **Concurrent Access**: Thread-safe operations with proper locking

## Installation

```bash
go get github.com/gophersatwork/granular
```

## Quick Start

```go
package main

import (
    "errors"
    "fmt"
    "log"

    "github.com/gophersatwork/granular" // Import the repository
)

func main() {
    // Create a new cache
    cache, err := granular.New(".cache") // Use the granular package name
    if err != nil {
        log.Fatalf("Failed to create cache: %v", err)
    }

    // Define a cache key
    key := granular.Key{
        Inputs: []granular.Input{
            granular.FileInput{Path: "main.go"},
            granular.GlobInput{Pattern: "*.json"},
        },
        Extra: map[string]string{"version": "1.0.0"},
    }

    // Check if the result is in the cache
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
            Path: "output.txt",
            Metadata: map[string]string{
                "summary": "This is a summary",
            },
        }
        
        if err := cache.Store(key, result); err != nil {
            log.Fatalf("Failed to store in cache: %v", err)
        }
        
        fmt.Println("Result stored in cache")
    }
}
```

## Input Types

The library supports several input types:

### FileInput

```go
// Single file input
input := granular.FileInput{Path: "path/to/file.txt"}
```

### GlobInput

```go
// Multiple files matching a pattern
input := granular.GlobInput{Pattern: "src/*.go"}
```

### DirectoryInput

```go
// All files in a directory (recursive)
input := granular.DirectoryInput{
    Path: "src/",
    Exclude: []string{"*.tmp", "*.log"},
}
```

### RawInput

```go
// Raw data
input := granular.RawInput{
    Data: []byte("raw data"),
    Name: "config",
}
```

## Configuration Options

The cache can be configured with various options:

```go
cache, err := granular.New(
    ".cache",
    granular.WithHashFunc(myCustomHashFunc),
)
```

## Performance Considerations

- **Hash function**: xxHash is used by default for its speed, but you can provide a custom hash function.
- **Buffer pooling**: Reuses buffers to reduce memory allocations.
- **Manifest caching**: Hot manifests are cached in memory to avoid disk I/O.
- **2-level directory**: Uses first 2 characters of hash for better filesystem distribution.

## File Structure

```
.cache/
├── manifests/
│   └── [first 2 chars of hash]/
│       └── [full hash].json
└── objects/
    └── [first 2 chars of hash]/
        └── [full hash]/
            └── [cached files]
```

## Examples
Check it out more ways to use `granular` [here](examples.md).

## Contributing

Contributions are welcome! Feel free to open a discussion. An easy onboarding for contributor can be found [here](arch.md)

## License

This project is licensed under the GPL License - see the [LICENSE](LICENSE) file for details.