# Tool Wrapper Caching POC

This proof-of-concept demonstrates how to wrap existing command-line tools with Granular caching to dramatically improve build and development workflows.

## Overview

Many development tools are expensive to run but produce deterministic output based on their inputs. By wrapping these tools with Granular caching, we can:

- **Skip redundant work** when inputs haven't changed
- **Speed up builds** by 10-100x on cache hits
- **Improve developer experience** with instant feedback
- **Reduce CI/CD costs** by caching across builds

## What's Included

This POC contains three complete tool wrapper examples:

1. **golint-cached** - Cached linter wrapper
   - Wraps `golangci-lint` or similar Go linters
   - Caches lint results based on source code content
   - ~25x speedup on cache hits

2. **protoc-cached** - Cached protobuf compiler wrapper
   - Wraps `protoc` for code generation
   - Caches generated code based on .proto files
   - ~35x speedup on cache hits

3. **asset-optimizer** - Cached asset optimization wrapper
   - Simulates image/CSS/JS optimization
   - Caches optimized assets based on source files
   - ~100x speedup on cache hits (for 5 files)

Plus:

4. **wrapper-template** - Generic template for wrapping any tool
5. **test-project** - Sample files for testing wrappers
6. **benchmark.sh** - Automated benchmark script

## Quick Start

### 1. Build the Wrappers

```bash
# Build all wrappers
cd golint-cached && go build
cd ../protoc-cached && go build
cd ../asset-optimizer && go build
```

### 2. Run the Benchmark

```bash
./benchmark.sh
```

This will:
- Build all three wrappers
- Run each wrapper multiple times
- Demonstrate cache hits and misses
- Show detailed performance metrics
- Calculate time savings

### 3. Try the Wrappers

```bash
# Lint Go code
cd test-project/go-code
../../golint-cached/golint-cached

# Generate protobuf code
cd ../proto-files
../../protoc-cached/protoc-cached --go_out=generated *.proto

# Optimize assets
cd ..
../asset-optimizer/asset-optimizer --input=assets --output=dist
```

## How It Works

### The Wrapper Pattern

Each wrapper follows the same pattern:

```
┌─────────────────────────────────────────────────────────┐
│ 1. Parse Arguments                                      │
│    - Extract input files                                │
│    - Extract configuration                              │
│    - Extract output location                            │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│ 2. Build Cache Key                                      │
│    - Hash input file contents                           │
│    - Include tool version                               │
│    - Include configuration/flags                        │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│ 3. Check Cache                                          │
│    cache.Get(key)                                       │
└────────────────┬────────────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
    Cache Hit        Cache Miss
        │                 │
        ▼                 ▼
┌───────────────┐  ┌──────────────┐
│ 4a. Restore   │  │ 4b. Run Tool │
│    - Extract  │  │    - Execute │
│      files    │  │    - Capture │
│    - Print    │  │      output  │
│      output   │  │              │
│    - Fast!    │  │    - Cache   │
│               │  │      results │
│               │  │    - Slow    │
└───────────────┘  └──────────────┘
```

### Cache Key Strategy

The cache key includes everything that affects the output:

```go
key := cache.Key().
    Version("tool-v1.0.0").              // Tool version
    File("input1.go").                   // Input file content (hashed)
    File("input2.go").                   // Input file content (hashed)
    String("flag", "--optimize").        // Configuration flags
    String("target", "linux-amd64").     // Build target
    Build()
```

Changes to any of these will invalidate the cache, ensuring correctness.

### Cache Storage

The cache stores:

1. **Output files** - Generated code, compiled binaries, optimized assets
2. **Console output** - stdout and stderr from the tool
3. **Exit code** - Tool's exit status
4. **Metadata** - Timestamps, file counts, etc.

## Performance Results

Based on the benchmark script:

| Tool | Cache Miss | Cache Hit | Speedup | Time Saved |
|------|------------|-----------|---------|------------|
| **golint-cached** | ~2.5s | ~0.1s | **25x** | 2.4s |
| **protoc-cached** | ~3.5s | ~0.1s | **35x** | 3.4s |
| **asset-optimizer** (5 files) | ~10s | ~0.1s | **100x** | 9.9s |

### Real-World Impact

For a typical development workflow:

```
Daily runs:
- Linting: 50 runs/day × 2.4s saved = 120s/day
- Code generation: 20 runs/day × 3.4s saved = 68s/day
- Asset optimization: 10 runs/day × 9.9s saved = 99s/day

Total time saved: ~287s/day (~4.8 minutes/day)
```

For a team of 10 developers over a year:
```
4.8 min/day × 10 devs × 250 work days = 12,000 minutes
                                       = 200 hours
                                       = 5 work weeks
```

## Integration Guide

### Makefile Integration

Replace your existing tool commands with cached wrappers:

```makefile
# Before
generate:
	protoc --go_out=. *.proto

lint:
	golangci-lint run ./...

optimize:
	optimize-images assets/

# After
generate:
	protoc-cached --go_out=. *.proto

lint:
	golint-cached run ./...

optimize:
	asset-optimizer --input=assets --output=dist
```

### CI/CD Integration

Cache the `.granular-cache` directory across CI runs:

#### GitHub Actions

```yaml
name: Build

on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Cache Granular
        uses: actions/cache@v3
        with:
          path: .granular-cache
          key: tools-${{ runner.os }}-${{ hashFiles('**/*.proto', '**/*.go') }}
          restore-keys: |
            tools-${{ runner.os }}-

      - name: Generate code
        run: ./tools/protoc-cached --go_out=. *.proto

      - name: Lint
        run: ./tools/golint-cached run ./...
```

#### GitLab CI

```yaml
variables:
  CACHE_DIR: .granular-cache

cache:
  key: tools-${CI_COMMIT_REF_SLUG}
  paths:
    - ${CACHE_DIR}

build:
  script:
    - ./tools/protoc-cached --go_out=. *.proto
    - ./tools/golint-cached run ./...
```

#### CircleCI

```yaml
jobs:
  build:
    steps:
      - checkout
      - restore_cache:
          keys:
            - tools-{{ checksum "go.sum" }}
            - tools-

      - run: ./tools/protoc-cached --go_out=. *.proto
      - run: ./tools/golint-cached run ./...

      - save_cache:
          key: tools-{{ checksum "go.sum" }}
          paths:
            - .granular-cache
```

### Shell Script Integration

Create a script that uses the cached wrapper if available:

```bash
#!/bin/bash

# Use cached wrapper if available, fall back to original tool
if command -v protoc-cached &> /dev/null; then
    protoc-cached "$@"
else
    protoc "$@"
fi
```

### Docker Integration

```dockerfile
FROM golang:1.21

# Install wrappers
COPY tools/protoc-cached /usr/local/bin/
COPY tools/golint-cached /usr/local/bin/

# Use cached volume
VOLUME /workspace/.granular-cache

# Your build commands use the wrappers automatically
```

## Creating Your Own Wrapper

### 1. Copy the Template

```bash
cp -r wrapper-template my-tool-cached
cd my-tool-cached
```

### 2. Customize for Your Tool

Edit `main.go` and update:

```go
const (
    toolName       = "mytool"        // Your tool name
    actualToolName = "mytool"        // Command to execute
    toolVersion    = "1.0.0"         // Wrapper version
)
```

### 3. Implement Tool-Specific Logic

Customize these functions:

- **parseToolArgs()** - Parse your tool's arguments
- **runTool()** - Execute the actual tool (usually no changes needed)
- **cacheResults()** - Store tool outputs
- **restoreResults()** - Restore cached outputs

See [wrapper-template/README.md](wrapper-template/README.md) for detailed instructions.

### 4. Test Your Wrapper

```bash
# Build
go build -o my-tool-cached

# Test cache miss
time ./my-tool-cached [args]

# Test cache hit
time ./my-tool-cached [args]  # Should be instant!

# Test invalidation
echo "change" >> input.txt
time ./my-tool-cached [args]  # Should be slow again
```

## Real-World Examples

Here are more tools that would benefit from caching:

### Build Tools

```bash
# Webpack
webpack-cached build --production

# Terraform
terraform-cached plan

# Docker build
docker-build-cached -t myapp:latest .

# CMake
cmake-cached --build .
```

### Linters & Formatters

```bash
# ESLint
eslint-cached src/

# Prettier
prettier-cached --write src/

# Black (Python)
black-cached .

# RuboCop
rubocop-cached
```

### Code Generators

```bash
# OpenAPI Generator
openapi-generator-cached generate -i api.yaml

# GraphQL Codegen
graphql-codegen-cached

# Swagger
swagger-cached generate

# gRPC
grpc-cached --proto_path=. *.proto
```

### Test Runners

```bash
# Jest
jest-cached --coverage

# Pytest
pytest-cached tests/

# Go test with coverage
go-test-cached -coverprofile=coverage.out ./...
```

### Asset Processing

```bash
# Image optimization
imageoptim-cached assets/

# SVG optimization
svgo-cached icons/

# PostCSS
postcss-cached src/ -d dist/

# Babel
babel-cached src -d lib
```

## Advanced Features

### Glob Patterns

Cache based on multiple files using glob patterns:

```go
key := cache.Key().
    Glob("src/**/*.go").         // All Go files
    Glob("proto/**/*.proto").    // All proto files
    Build()
```

### Metadata

Store and retrieve metadata about cached results:

```go
// Store metadata
cache.Put(key).
    File("output", "result.txt").
    Meta("build_time", "1.23s").
    Meta("warnings", "5").
    Meta("file_count", "42").
    Commit()

// Retrieve metadata
result := cache.Get(key)
if result != nil {
    buildTime := result.Meta("build_time")
    warnings := result.Meta("warnings")
}
```

### Selective Caching

Only cache successful runs:

```go
stdout, stderr, exitCode, err := runTool(toolPath, args)

// Only cache if tool succeeded
if exitCode == 0 {
    cache.Put(key).
        Bytes("stdout", stdout).
        Meta("exit_code", "0").
        Commit()
}
```

### Cache Statistics

Monitor cache effectiveness:

```go
stats, _ := cache.Stats()
fmt.Printf("Entries: %d\n", stats.Entries)
fmt.Printf("Total size: %s\n", stats.TotalSize)

entries, _ := cache.Entries()
for _, entry := range entries {
    fmt.Printf("Key: %s, Created: %s\n", entry.KeyHash, entry.CreatedAt)
}
```

## Best Practices

### 1. Cache Location

Add `.granular-cache` to `.gitignore`:

```gitignore
# Granular cache
.granular-cache/
```

But **do cache** in CI/CD (see integration examples above).

### 2. Cache Keys

Include everything that affects output:
- ✅ Input file contents (automatically hashed by `File()`)
- ✅ Tool version
- ✅ Configuration files
- ✅ Environment variables that affect output
- ✅ Command-line flags

Don't include:
- ❌ Timestamps
- ❌ User names
- ❌ Absolute paths (use relative paths)
- ❌ Non-deterministic data

### 3. Error Handling

Preserve the original tool's behavior:
- Return the same exit codes
- Output the same stdout/stderr
- Handle tool not found gracefully

```go
// Good: Fall back to original behavior
toolPath, err := exec.LookPath(actualToolName)
if err != nil {
    return fmt.Errorf("%s not found: %w", actualToolName, err)
}
```

### 4. Testing

Test all scenarios:
- ✅ Cache miss (first run)
- ✅ Cache hit (second run)
- ✅ Cache invalidation (after input change)
- ✅ Multiple inputs
- ✅ Different configurations
- ✅ Error cases

### 5. Documentation

Document for your team:
- How to install the wrapper
- How to use it in their workflow
- Expected performance improvements
- How to debug cache issues

## Troubleshooting

### Cache Not Hitting

**Problem**: Second run still takes full time

**Solutions**:
1. Check if inputs are actually the same
2. Verify tool version hasn't changed
3. Check if configuration files changed
4. Look for timestamps in cache key

### Cache Growing Too Large

**Problem**: `.granular-cache` directory is huge

**Solutions**:
1. Implement cache size limits
2. Add cache expiration
3. Clean old entries periodically

```bash
# Clear old cache entries
find .granular-cache -type f -mtime +7 -delete
```

### Cache Not Portable

**Problem**: Cache doesn't work across machines

**Solutions**:
1. Use relative paths in cache keys
2. Don't include machine-specific data
3. Ensure consistent tool versions

### Debugging

Enable verbose logging:

```go
// Add debug output
if os.Getenv("DEBUG") != "" {
    fmt.Printf("Cache key: %+v\n", key)
    fmt.Printf("Cache hit: %v\n", result != nil)
}
```

## Performance Tips

### 1. Optimize Cache Key

Only include what's necessary:

```go
// Too broad - caches too little
key := cache.Key().
    File("go.mod").              // Changes rarely
    Glob("**/*.go").             // Too many files

// Better - focused on relevant inputs
key := cache.Key().
    File("main.go").
    File("utils.go").
    Version("tool-v1.0.0")
```

### 2. Parallel Execution

Cache is thread-safe, so you can run wrappers in parallel:

```bash
# Run multiple wrappers concurrently
protoc-cached *.proto &
golint-cached ./... &
asset-optimizer --input=assets &
wait
```

### 3. Shared Cache

Share cache across team members via:
- Network file system (NFS)
- Shared CI cache
- Cache server (future enhancement)

### 4. Warm Cache in CI

Pre-populate cache before main build:

```yaml
- name: Warm cache
  run: |
    protoc-cached *.proto
    golint-cached ./...

- name: Main build
  run: make build  # Uses pre-warmed cache
```

## Architecture

### Cache Structure

```
.granular-cache/
├── manifests/           # Cache metadata
│   └── ab/
│       └── abc123....json
└── objects/             # Cached content
    └── ab/
        └── abc123.../
            ├── file1
            └── file2
```

### Key Computation

Cache keys are computed from:

```
key_hash = xxhash64(
    "version:" + tool_version + "\n" +
    "file:input1.go:" + hash(content) + "\n" +
    "file:input2.go:" + hash(content) + "\n" +
    "string:flag:value" + "\n"
)
```

### File Deduplication

Files are content-addressed, so identical content is stored once:

```
objects/
└── 12/
    └── 123abc.../      # Hash of file content
        └── data        # Actual file

# Multiple cache entries can reference the same object
```

## Future Enhancements

- [ ] Distributed cache server
- [ ] Cache analytics dashboard
- [ ] Automatic cache size management
- [ ] Remote cache backends (S3, GCS)
- [ ] Cache warming strategies
- [ ] Multi-tool orchestration

## FAQ

**Q: Is the cache safe for concurrent access?**
A: Yes, the cache is thread-safe and can be used by multiple processes.

**Q: What if the tool has non-deterministic output?**
A: Don't cache tools with non-deterministic output (e.g., tools that include timestamps). Or strip the non-deterministic parts.

**Q: Can I share the cache across machines?**
A: Yes, via CI/CD caching or shared file systems. The cache is portable as long as tool versions match.

**Q: What's the cache size overhead?**
A: Typically 1-2x the size of cached outputs, due to metadata and content-addressing.

**Q: How do I clear the cache?**
A: `rm -rf .granular-cache` or `cache.Clear()` programmatically.

**Q: Can I cache across different tool versions?**
A: No, different versions create different cache keys. This ensures correctness.

## Contributing

To add more wrapper examples:

1. Copy `wrapper-template/`
2. Implement tool-specific logic
3. Add test cases
4. Update benchmark script
5. Document in this README

## License

Same as the Granular project.

## Resources

- [Granular Documentation](../../README.md)
- [Wrapper Template](wrapper-template/README.md)
- [Benchmark Script](benchmark.sh)
- [Test Project](test-project/)

---

**Ready to speed up your builds?** Start with the benchmark:

```bash
./benchmark.sh
```

Then integrate the wrappers into your workflow and watch your build times plummet!
