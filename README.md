# Granular - High-Performance Content-Based Cache

[![Go Reference](https://pkg.go.dev/badge/github.com/gophersatwork/granular.svg)](https://pkg.go.dev/github.com/gophersatwork/granular)
[![Go Report Card](https://goreportcard.com/badge/github.com/gophersatwork/granular)](https://goreportcard.com/report/github.com/gophersatwork/granular)

A high-performance, deterministic file cache for Go applications. Inspired by Zig's build cache architecture.

Granular is a **universal caching layer** that brings intelligent caching to tools and workflows where native caching is absent, limited, or local-only. It's designed to accelerate expensive deterministic operations by caching their results based on content hashes.

<img src="assets/granular-alpha.png" width="400" height="400">

## Features

- **Content-Based Caching** - Cache based on actual file content, not timestamps
- **Fluent Builder API** - Self-documenting, minimal & opinionated design
- **Multi-File Support** - Cache multiple output files and data in a single entry
- **Zero Configuration** - Smart defaults with xxHash64, works out of the box
- **Cache Management** - Built-in stats, pruning, and introspection
- **Fast & Efficient** - Optimized with buffer pooling and efficient hashing

## Why Granular?

Go already has an excellent build cache, so **why do you need Granular?**

Granular fills the gaps where native caching falls short:

- **Missing caching**: Tools like `protoc`, `golangci-lint`, and asset optimizers have no built-in caching
- **Local-only caching**: Go's test cache works locally but can't be shared across CI/CD runs or team members
- **Complex dependencies**: When you need explicit control over cache keys (e.g., multiple input files, environment variables, build flags)
- **Cross-tool caching**: Unified cache layer for heterogeneous tool pipelines

### Real-World Impact

From our [proof-of-concept examples](/poc):

| Use Case | Speedup | Time Saved |
|----------|---------|------------|
| **Protobuf generation** (`protoc`) | 35x faster | 3.4s → 0.1s |
| **Integration tests** (Go test caching) | 13-18x faster | 140ms → 10ms |
| **Monorepo builds** (incremental) | 8.3x faster | 1.0s → 0.12s |
| **Linting** (`golangci-lint`) | 25x faster | 2.5s → 0.1s |
| **Asset optimization** | 100x faster | 10s → 0.1s |

**For a team of 10 developers:** ~10.5 hours saved per month on these operations alone.

## When to Use Granular

Granular excels in these scenarios:

### 1. Tools Without Native Caching

Wrap deterministic tools that have no caching:
- **Code generation**: `protoc`, OpenAPI generators, GraphQL codegen
- **Linting/Formatting**: `golangci-lint`, `prettier`, `black`
- **Asset processing**: Image optimization, CSS/JS bundling

### 2. Remote Caching for CI/CD

Share cache across builds and team members:
- **CI pipelines**: Cache test results, build artifacts, generated code
- **Distributed teams**: Share cache via network storage or S3
- **Multi-stage builds**: Reuse artifacts between pipeline stages

### 3. Integration Test Caching

Cache expensive test results that Go's built-in cache can't handle:
- **Database integration tests**: Tests with external dependencies
- **E2E tests**: Slow end-to-end test suites
- **Monorepo testing**: Cache per-package test results

### 4. Complex Cache Keys

When you need explicit control over what invalidates the cache:
- **Multiple input files**: Hash specific files, globs, or entire directories
- **Environment dependencies**: Include `GOOS`, `GOARCH`, or custom env vars
- **Build configurations**: Different cache entries for different build flags

### 5. Monorepo Build Optimization

Smart incremental builds that only rebuild what changed:
- **Dependency tracking**: Automatically invalidate dependent packages
- **Selective rebuilds**: Skip unchanged packages entirely
- **Parallel builds**: Cache-enabled parallel compilation

## When NOT to Use Granular

Be honest about limitations:

### Go Build Cache

**Don't use Granular for:**
- Pure Go compilation (`go build`)
- Pure Go tests (`go test` without external dependencies)

**Why not?** Go's native build cache is excellent and deeply integrated with the toolchain. Use it.

**Exception:** If you need to share Go's build cache across CI runs or team members, consider Granular as a wrapper.

### In-Memory Caching

**Don't use Granular for:**
- HTTP response caching
- Database query results
- Short-lived session data

**Use instead:** `groupcache`, `ristretto`, `httpcache`, or in-memory key-value stores.

### Distributed Caching Systems

**Don't use Granular for:**
- Multi-server application state
- High-throughput distributed caching
- Sub-millisecond latency requirements

**Use instead:** Redis, Memcached, or other distributed caching systems.

### Non-Deterministic Operations

**Don't use Granular for:**
- Operations that depend on current time
- Network requests with dynamic responses
- Random number generation
- Any process with non-reproducible output

## Comparison to Alternatives

### vs. Go's Build Cache

| Feature | Go Build Cache | Granular |
|---------|----------------|----------|
| **Go compilation** | Excellent | Not needed |
| **Go tests (pure)** | Excellent | Not needed |
| **Go tests (integration)** | Limited | Excellent |
| **Remote caching** | No | Yes |
| **Custom cache keys** | No | Yes |
| **Non-Go tools** | No | Yes |

**Bottom line:** Use Go's cache for Go. Use Granular for everything else.

### vs. Bazel/Buck/Nx

| Feature | Bazel/Buck | Nx | Granular |
|---------|------------|-----|----------|
| **Language** | Custom DSL | JavaScript | Pure Go |
| **Setup complexity** | High | Medium | Minimal |
| **Learning curve** | Steep | Moderate | Gentle |
| **Go-native** | No | No | Yes |
| **Remote caching** | Yes | Yes | Yes (via storage backends) |
| **Build orchestration** | Full build system | Full build system | Cache layer only |

**Bottom line:** Bazel/Buck/Nx are full build systems. Granular is a caching library you integrate into your existing build process.

### vs. ccache/sccache

| Feature | ccache/sccache | Granular |
|---------|----------------|----------|
| **Target language** | C/C++/Rust | Any (Go-integrated) |
| **Integration** | Compiler wrapper | Library/custom wrapper |
| **Content hashing** | Yes | Yes |
| **Remote caching** | sccache only | Yes |
| **Flexibility** | Compiler-specific | Tool-agnostic |

**Bottom line:** ccache/sccache are compiler-specific. Granular is a general-purpose caching library.

## Installation

```bash
go get github.com/gophersatwork/granular
```

## Quick Start

### Example: Caching Protobuf Generation

Instead of running `protoc` every time, cache the generated code based on `.proto` file contents:

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "time"

    "github.com/gophersatwork/granular"
)

func main() {
    // Open a cache
    cache, err := granular.Open(".cache")
    if err != nil {
        log.Fatalf("Failed to open cache: %v", err)
    }

    // Build cache key from all .proto files
    key := cache.Key().
        Glob("proto/**/*.proto").      // Hash all proto files
        Version("protoc-v3.21.0").     // Include tool version
        String("go_out", "gen").       // Include output config
        Build()

    // Check cache
    result, err := cache.Get(key)
    if err == nil && result != nil {
        // Cache hit! Restore generated files
        fmt.Println("Cache HIT - Restoring generated code (35x faster!)")
        for _, file := range result.Files() {
            fmt.Printf("  ✓ Restored: %s\n", file)
        }
        return
    }

    // Cache miss - run protoc
    fmt.Println("Cache MISS - Running protoc...")
    cmd := exec.Command("protoc", "--go_out=gen", "proto/**/*.proto")
    if err := cmd.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "protoc failed: %v\n", err)
        os.Exit(1)
    }

    // Cache the generated files
    if err := cache.Put(key).
        File("user.pb.go", "gen/user.pb.go").
        File("product.pb.go", "gen/product.pb.go").
        Meta("generated_at", time.Now().String()).
        Commit(); err != nil {
        log.Fatalf("Failed to cache results: %v", err)
    }

    fmt.Println("Generated code cached for next run")
}
```

**Result:**
- First run: ~3.5s (runs protoc)
- Subsequent runs: ~0.1s (cache hit)
- **35x faster** for unchanged proto files

## Core API

### Opening a Cache

```go
// Production use
cache, err := granular.Open(".cache")

// In-memory cache for testing
cache := granular.OpenTemp()
```

### Building Cache Keys

The fluent KeyBuilder API makes cache keys self-documenting:

```go
key := cache.Key().
    File("src/main.go").              // Single file
    Glob("src/**/*.go").               // Glob pattern with ** support
    Dir("configs", "*.tmp").           // Directory with exclusions
    Bytes([]byte("data")).             // Raw bytes
    String("version", "1.0").          // Key-value metadata
    Version("2.0.1").                  // Sugar for String("version", ...)
    Env("GOOS").                       // Environment variable
    Build()
```

### Retrieving from Cache

```go
result, err := cache.Get(key)
if errors.Is(err, granular.ErrCacheMiss) {
    // Cache miss - do work
} else if err != nil {
    // Handle other errors (I/O, corruption, etc.)
    return err
}

// Access cached files
path := result.File("output")
allFiles := result.Files()

// Access cached data
data := result.Bytes("summary")

// Access metadata
meta := result.Meta("build_time")
```

### Storing Results

```go
err := cache.Put(key).
    File("binary", "./app").           // Cache a file with logical name
    File("symbols", "./app.sym").      // Multiple files supported
    Bytes("logs", logData).            // Store byte data
    Meta("build_time", "123ms").       // Attach metadata
    Commit()
```

### Cache Management

```go
// Get statistics
stats, _ := cache.Stats()
fmt.Printf("Entries: %d, Size: %d bytes\n", stats.Entries, stats.TotalSize)

// Prune old entries
removed, _ := cache.Prune(7 * 24 * time.Hour)

// Check if key exists
if cache.Has(key) {
    // ...
}

// Delete specific entry
cache.Delete(key)

// Clear entire cache
cache.Clear()
```

### Error Handling & Validation

Granular uses **eager validation** with error accumulation. Validation happens during key building, but errors are only surfaced when you call `Get()` or `Commit()`.

```go
// Invalid inputs are validated immediately
key := cache.Key().
    File("missing.txt").     // Validates file exists NOW
    Glob("bad[pattern").     // Validates pattern NOW
    Build()                  // Always succeeds (no error returned)

// Errors surface here
result, err := cache.Get(key)
if err != nil {
    var validationErr *granular.ValidationError
    if errors.As(err, &validationErr) {
        // Multiple validation errors
        for _, e := range validationErr.Errors {
            fmt.Printf("- %v\n", e)
        }
    }
}
```

**Fail-Fast vs Accumulate-All-Errors:**

By default, Granular stops validating after the first error (fail-fast) for better performance. You can collect all errors during development:

```go
// Enable error accumulation
cache, _ := granular.Open(".cache", granular.WithAccumulateErrors())

key := cache.Key().
    File("missing1.txt").    // Error 1
    File("missing2.txt").    // Error 2 (normally skipped in fail-fast mode)
    Glob("bad[pattern").     // Error 3 (normally skipped in fail-fast mode)
    Build()

result, err := cache.Get(key)
// err contains all 3 validation errors
```

**Cache Miss Detection:**

Use `errors.Is()` to detect cache misses:

```go
result, err := cache.Get(key)
if errors.Is(err, granular.ErrCacheMiss) {
    // Cache miss - compute and cache result
} else if err != nil {
    // Other errors (validation, I/O, corruption)
    return err
}
// result != nil - cache hit
```

## Design Philosophy

**Minimal & Opinionated** - Inspired by Zig's approach:
- Single obvious way to accomplish common tasks
- Eager validation with clear error messages (no panics)
- Self-documenting API via fluent builders
- Zero configuration for 95% of use cases

## Performance

- Uses xxHash64 by default (one of the fastest non-cryptographic hashes)
- Buffer pooling for file I/O operations
- Two-level directory structure (sharding) for efficient filesystem operations
- No global state, fully concurrent-safe

## Development & Testing

### Running Tests

```bash
# Run all tests
mise run test

# Run formatting, linting, and tests
mise run check

# Run tests with go directly
go test -v

# Run specific test
go test -v -run TestBasicCacheOperations

# Run benchmarks
go test -bench=.
```

### Local CI Testing

Test GitHub Actions workflows locally before pushing:

```bash
# Run full CI pipeline locally
mise run ci:local

# Run with verbose output
mise run ci:verbose

# Simulate pull request checks
mise run ci:pr

# List available CI jobs
mise run ci:list
```

This uses [act](https://github.com/nektos/act) to run workflows in Docker containers. See [scripts/README.md](scripts/README.md) for more details.

### Development Workflow

Before pushing changes:

```bash
# 1. Format code
mise run fmt

# 2. Run local checks
mise run check

# 3. Test CI pipeline
mise run ci:local
```

## FAQ

### When should I use Granular vs Go's built-in cache?

Go's build cache is excellent for pure Go compilation. Use Granular when:
- You need to cache tools without native caching (protoc, asset optimization)
- You need to cache integration tests (Go's cache explicitly excludes them)
- You need remote/shared caching for CI/CD
- You're building multi-tool pipelines

**Rule of thumb**: If Go's cache works for you, use it. If you're wrapping external tools or need remote caching, use Granular.

### Does Granular support remote caching?

Not yet built-in, but you can achieve remote caching today by:
- Mounting network filesystems (NFS, S3FS)
- Using rsync or similar tools to sync `.cache` directory
- Implementing a custom `afero.Fs` backend

Built-in remote cache backends (S3, GCS, Redis) are on the roadmap.

### Is this production-ready?

The core library is stable with comprehensive tests and well-defined APIs. The POC examples demonstrate real-world usage with verified performance numbers.

As with any caching system, test thoroughly in your environment first. Start with non-critical workflows and gradually expand usage.

### How do I debug cache misses?

Use `KeyBuilder.Hash()` or `Key.Hash()` to see cache key hashes:

```go
key := cache.Key().File("input.txt").Build()
fmt.Println("Cache key:", key.Hash())  // Print for debugging
```

Compare hashes between runs to understand why keys differ. Common causes:
- File content changed
- File path changed
- Additional inputs added/removed
- Version string changed

### What's the performance overhead?

- **Cache hit**: ~1-10ms (file I/O + hash lookup)
- **Cache miss**: Original operation time + ~5-10ms caching overhead
- **Hash computation**: ~1-5ms per MB of input files

For operations taking >100ms, overhead is negligible (<5%). For very fast operations (<50ms), evaluate if caching provides sufficient benefit.

### Can I use this with Docker/containers?

Yes! Common patterns:
- Mount cache directory as volume: `-v /host/.cache:/app/.cache`
- Use Docker layer caching for frequently accessed entries
- Share cache across CI jobs using cache volumes

### How much disk space does the cache use?

Cache size equals the sum of all cached output files plus small manifest overhead. Use `cache.Stats()` to monitor:

```go
stats, _ := cache.Stats()
fmt.Printf("Entries: %d, Total Size: %d MB\n",
    stats.Entries, stats.TotalSize/1024/1024)
```

Use `cache.Prune()` to remove old entries:

```go
// Remove entries older than 7 days
removed, _ := cache.Prune(7 * 24 * time.Hour)
```

## License

GPL-3.0 License - See LICENSE file for details

## Contributing

Contributions welcome! Please open an issue to discuss major changes.

## Examples & Proof-of-Concepts

Want to see Granular in action? Check out our comprehensive proof-of-concept examples:

**Note:** All POC examples use workspace-local cache directories for simplicity and self-containment. This is appropriate for non-production demonstration code. See [POC Documentation](/poc) for details on workspace mode and production cache strategies.

### [Test Result Caching](/poc/test-caching)
Cache expensive integration tests to skip re-running when code hasn't changed.
- **13-18x faster** for cached test runs
- Works with database tests, E2E tests, and any slow test suite
- [View Example →](/poc/test-caching)

### [Monorepo Build Orchestration](/poc/monorepo-build)
Intelligent incremental builds that only rebuild what changed.
- **8.3x faster** for full cache hits
- Smart dependency tracking across packages
- [View Example →](/poc/monorepo-build)

### [Tool Wrappers](/poc/tool-wrapper)
Wrap existing CLI tools with caching for instant results.

**Included wrappers:**
- `protoc-cached`: 35x faster protobuf generation
- `golint-cached`: 25x faster Go linting
- `asset-optimizer`: 100x faster asset optimization
- Generic template for wrapping ANY tool

[View Example →](/poc/tool-wrapper)

### Performance Summary

| Example | Normal | Cached | Speedup |
|---------|--------|--------|---------|
| Test Caching | ~140ms | ~10ms | 13-18x |
| Monorepo Build | ~1.0s | ~0.12s | 8.3x |
| Protoc Wrapper | ~3.5s | ~0.1s | 35x |
| Lint Wrapper | ~2.5s | ~0.1s | 25x |

**See [/poc](/poc) for complete documentation, benchmarks, and runnable code.**

---

**Built with simplicity in mind. Cache smarter, not harder.**
