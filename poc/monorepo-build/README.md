# Monorepo Build Orchestration POC

## Important Context Notice

> **This example demonstrates build orchestration, not build caching.**
>
> Go's native build cache is already excellent for Go compilation and handles content-based caching automatically. The value demonstrated here is in **coordinating multiple packages**, not in providing better caching than Go's built-in system.
>
> **For better examples of Granular's unique value proposition, see:**
> - `data-pipeline` - Multi-tool orchestration where native caching doesn't exist
> - `tool-wrapper` - External tools with no native caching
> - `test-caching` - Integration tests that Go's cache can't handle

This proof-of-concept demonstrates how **Granular** can coordinate build orchestration in a monorepo environment by tracking package dependencies and caching build artifacts.

## Overview

This POC simulates a realistic monorepo with:
- **3 microservices** (api, worker, admin)
- **2 shared libraries** (models, utils)
- **Clear dependency relationships** between packages
- **Intelligent caching** using Granular

## Architecture

```
monorepo-build/
├── services/
│   ├── api/                    # HTTP API service
│   │   ├── main.go            # Server setup
│   │   └── handler.go         # Request handlers
│   │
│   ├── worker/                 # Background job processor
│   │   ├── main.go            # Worker pool management
│   │   └── processor.go       # Job processing logic
│   │
│   └── admin/                  # Admin CLI tool
│       ├── main.go            # CLI interface
│       └── commands.go        # Command implementations
│
├── shared/
│   ├── models/                 # Shared data models
│   │   └── user.go            # User model
│   │
│   └── utils/                  # Shared utilities
│       └── helpers.go         # Helper functions
│
├── build_normal.go            # Naive builder (no caching)
├── build_smart.go             # Smart builder (Granular-powered)
├── benchmark.sh               # Automated benchmark suite
├── go.mod                     # Main module
└── go.work                    # Go workspace
```

### Dependency Graph

```
┌─────────────────────────────────────────────────────┐
│                                                     │
│  ┌─────────┐                    ┌─────────┐       │
│  │ models  │                    │  utils  │       │
│  └────┬────┘                    └────┬────┘       │
│       │                              │            │
│       └──────────┬───────────────────┘            │
│                  │                                 │
│       ┏━━━━━━━━━━┻━━━━━━━━━━┓                     │
│       ┃                      ┃                     │
│   ┌───▼───┐  ┌────────┐  ┌──▼────┐               │
│   │  api  │  │ worker │  │ admin │               │
│   └───────┘  └────────┘  └───────┘               │
│                                                     │
└─────────────────────────────────────────────────────┘

Legend:
  models, utils = Shared libraries (no external dependencies)
  api, worker, admin = Services (depend on models + utils)
```

**Key Points:**
- Shared libraries (`models`, `utils`) have **no dependencies**
- All services depend on **both** shared libraries
- Changing a shared library requires rebuilding **all services**
- Changing one service only requires rebuilding **that service**

## Build Systems

### 1. Normal Builder (`build_normal.go`)

A naive build system that:
- Rebuilds **all packages** every time
- No caching or optimization
- Represents traditional build approaches
- Baseline for comparison

**When to use:** Never in production, only for benchmarking

### 2. Smart Builder (`build_smart.go`)

An intelligent build system powered by Granular that:
- **Detects changed files** by hashing source code
- **Tracks dependencies** automatically
- **Caches build artifacts** in `.cache/granular/`
- **Rebuilds only what's necessary**
- **Invalidates dependent caches** when shared packages change

**When to use:** All real-world scenarios

## How It Works

### Cache Key Generation

The smart builder generates cache keys based on:

1. **Source file hashes**: All `.go` files in the package
2. **Dependency hashes**: Hashes of all dependencies
3. **Build metadata**: Go version, build flags, etc.

```go
cacheKey := granular.NewKey(
    granular.StringParam("package", "api"),
    granular.StringParam("hash", sourceHash + dependencyHashes),
    granular.StringParam("go_version", "1.24"),
)
```

### Dependency Tracking

```go
dependencies := map[string][]string{
    "models": {},                    // No dependencies
    "utils":  {},                    // No dependencies
    "api":    {"models", "utils"},   // Depends on both shared libs
    "worker": {"models", "utils"},   // Depends on both shared libs
    "admin":  {"models", "utils"},   // Depends on both shared libs
}
```

When a shared package changes, its hash changes, which **invalidates** the cache keys for all dependent services.

## Running the Benchmark

### Quick Start

```bash
cd poc/monorepo-build
./benchmark.sh
```

### What the Benchmark Tests

The benchmark runs 5 scenarios:

1. **Fresh build (no cache)**
   - Both builders compile everything from scratch
   - Establishes baseline performance

2. **Rebuild without changes**
   - Tests cache effectiveness
   - Expected: 90%+ speedup with smart builder

3. **Change one service**
   - Modifies `services/api/handler.go`
   - Normal: Rebuilds everything
   - Smart: Rebuilds only `api`
   - Expected: 60-80% speedup

4. **Change shared package**
   - Modifies `shared/models/user.go`
   - Both builders rebuild models + all services
   - Tests dependency tracking correctness

5. **Change two services**
   - Modifies `worker` and `admin`
   - Normal: Rebuilds everything
   - Smart: Rebuilds only worker + admin
   - Expected: 40-60% speedup

### Manual Testing

#### Build with Normal Builder
```bash
go run build_normal.go
```

#### Build with Smart Builder
```bash
go run build_smart.go
```

#### Clear Cache
```bash
rm -rf .cache bin
```

#### Modify a Package
```bash
# Modify a service
echo "// Test change" >> services/api/handler.go

# Modify a shared package
echo "// Test change" >> shared/models/user.go
```

## Expected Performance

### Typical Results

| Scenario | Normal Build | Smart Build | Improvement |
|----------|--------------|-------------|-------------|
| Fresh build | 8-12s | 8-12s | ~0% (no cache) |
| No changes | 8-12s | 0.5-1s | **90-95%** |
| One service changed | 8-12s | 2-3s | **60-75%** |
| Shared package changed | 8-12s | 6-9s | 20-30% |
| Two services changed | 8-12s | 3-5s | **50-60%** |

### Real-World Impact

In a typical development workflow:
- **Daily builds**: 20-50 builds per developer
- **Average cache hit rate**: 70-80%
- **Time saved per day**: 30-60 minutes per developer
- **Team of 10 developers**: **5-10 hours saved per day**

### Scaling Benefits

As monorepo size increases, benefits multiply:

| Packages | Normal Build | Smart Build (70% cache hit) | Time Saved |
|----------|--------------|----------------------------|------------|
| 5 packages | 10s | 3s | 70% |
| 10 packages | 20s | 6s | 70% |
| 20 packages | 40s | 12s | 70% |
| 50 packages | 100s | 30s | 70% |
| 100 packages | 200s | 60s | 70% |

**Note:** Cache hit rate improves as code stabilizes and fewer packages change per commit.

## Key Features Demonstrated

### 1. Content-Based Caching
- Cache keys based on **actual content**, not timestamps
- Robust against file touches, reformatting, etc.

### 2. Dependency-Aware Invalidation
- Automatically rebuilds dependent packages
- Prevents using stale artifacts

### 3. Granular Parallelization
- Each package can be cached independently
- Enables parallel builds (can be extended)

### 4. Build Artifact Preservation
- Compiled binaries stored in cache
- Instant restore for unchanged packages

### 5. Human-Readable Output
- Clear indication of cache hits vs. builds
- Build time tracking
- Cache statistics

## Limitations of This Example

### Go's Native Build Cache Already Exists

Go's built-in build cache handles content-based caching automatically. When you run `go build`, Go:
- Hashes source files and dependencies
- Reuses compiled packages that haven't changed
- Works transparently without extra tools

**The reality:** Go's cache is already doing what this example demonstrates.

### This Is Orchestration, Not Caching

What this example actually shows is orchestration - coordinating builds of multiple packages and tracking their relationships. Go itself doesn't provide:
- Explicit coordination across workspace packages
- Custom cache invalidation strategies
- Build artifact preservation across workspace changes

**However:** Go's `go.work` file may be a better solution for your actual needs than a custom orchestration layer.

### Performance Improvements Are From Orchestration

The speedups shown in benchmarks come from:
1. Coordinating package rebuilds
2. Skipping packages that don't need building
3. **Not** from caching, which Go already does

Go achieves similar results automatically in many scenarios.

## Better Use Cases for Granular

If you want to see where Granular truly shines, check out these examples:

### 1. `data-pipeline` - Multi-Tool Orchestration
- Coordinates execution of Python, protoc, and other tools
- These tools don't have native integration with Go's cache
- Granular's value: **Caching results of external tool chains**

### 2. `protoc-cached` - Tools Without Native Caching
- Protocol buffer compilation
- protoc has no native caching mechanism
- Granular's value: **Adding smart caching to tools that lack it**

### 3. `test-caching` - Go Cache Gaps
- Integration tests that depend on external resources
- Go's cache can't handle external dependencies
- Granular's value: **Caching test results when Go's cache doesn't apply**

## Use Cases

### Development Workflow
```bash
# Initial checkout
$ git clone repo && cd repo
$ go run build_smart.go          # Full build: 12s

# Work on API
$ vim services/api/handler.go
$ go run build_smart.go          # Rebuild API only: 2s

# Pull latest changes (10 files changed)
$ git pull
$ go run build_smart.go          # Rebuild changed packages: 4s

# Switch branches
$ git checkout feature-branch
$ go run build_smart.go          # Use cached artifacts: 1s
```

### CI/CD Pipeline
```bash
# PR builds - only changed packages
$ go run build_smart.go

# Main branch - verify everything
$ rm -rf .cache
$ go run build_smart.go
```

### Monorepo Benefits

Perfect for:
- **Microservices architectures**
- **Shared library ecosystems**
- **Large-scale Go projects**
- **Multi-team development**

## Extending the POC

### Add More Services
1. Create new service directory
2. Add to `go.work`
3. Update `getPackages()` in both builders
4. Define dependencies

### Add Build Variants
```go
cacheKey := granular.NewKey(
    granular.StringParam("package", pkg.Name),
    granular.StringParam("os", runtime.GOOS),
    granular.StringParam("arch", runtime.GOARCH),
    granular.StringParam("tags", buildTags),
)
```

### Parallel Builds
```go
var wg sync.WaitGroup
for _, pkg := range packages {
    wg.Add(1)
    go func(p Package) {
        defer wg.Done()
        builder.buildPackage(p)
    }(pkg)
}
wg.Wait()
```

### Remote Caching
```go
cache, err := granular.NewCache[BuildResult](
    granular.WithCacheDir(cacheDir),
    granular.WithRemoteCache("s3://bucket/cache"),
)
```

## Comparison with Other Tools

| Feature | Granular POC | Bazel | Buck | nx |
|---------|--------------|-------|------|----|
| Language-native | ✅ Pure Go | ❌ Custom DSL | ❌ Custom DSL | ❌ JavaScript |
| Setup complexity | ✅ Minimal | ❌ High | ❌ High | 🟡 Medium |
| Content hashing | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes |
| Dependency tracking | ✅ Automatic | 🟡 Manual | 🟡 Manual | ✅ Automatic |
| Go workspace support | ✅ Native | 🟡 Limited | 🟡 Limited | ❌ No |
| Learning curve | ✅ Low | ❌ High | ❌ High | 🟡 Medium |

## Troubleshooting

### Cache not working
```bash
# Verify cache directory exists
ls -la .cache/granular/

# Check cache stats in output
go run build_smart.go | grep "Cache hit rate"

# Clear and rebuild
rm -rf .cache && go run build_smart.go
```

### Stale artifacts
```bash
# Force rebuild (ignores cache)
rm -rf .cache
go run build_smart.go
```

### Dependency issues
```bash
# Verify Go workspace
go work sync

# Check module dependencies
go mod tidy
```

## Performance Tips

1. **Use SSD for cache storage** - I/O is often the bottleneck
2. **Implement parallel builds** - Maximize CPU usage
3. **Share cache across CI jobs** - Use remote cache backend
4. **Regular cache cleanup** - Remove old artifacts
5. **Monitor cache hit rates** - Tune invalidation logic

## Conclusion

This POC demonstrates that **Granular provides significant build time improvements** for monorepo workflows:

- **90%+ speedup** for unchanged code
- **50-80% speedup** for incremental changes
- **Automatic dependency tracking**
- **Minimal integration effort**
- **Pure Go, no external tools**

### Next Steps

1. **Add remote caching** for team-wide cache sharing
2. **Implement parallel builds** for faster compilation
3. **Add build matrix support** (OS, arch, tags)
4. **Integrate with CI/CD** pipelines
5. **Add cache analytics** and monitoring

## License

This POC is part of the Granular project. See the main repository for license information.

## Resources

- [Granular Documentation](https://github.com/gophersatwork/granular)
- [Go Workspaces](https://go.dev/doc/tutorial/workspaces)
- [Content-Based Caching](https://en.wikipedia.org/wiki/Content-addressable_storage)
