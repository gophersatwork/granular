# Quick Start Guide

Get up and running with tool wrapper caching in 5 minutes.

## 1. Build the Wrappers

```bash
cd poc/tool-wrapper

# Build all three wrappers
cd golint-cached && go build
cd ../protoc-cached && go build
cd ../asset-optimizer && go build
cd ..
```

## 2. Run the Benchmark

The benchmark script demonstrates all three wrappers with before/after comparisons:

```bash
./benchmark.sh
```

Expected output:
```
=====================================
Tool Wrapper Caching Benchmark
=====================================

Building wrappers...
All wrappers built successfully!

=====================================
1. GOLINT-CACHED BENCHMARK
=====================================

Run 1: First run (cache miss)
  Time: 2.51s

Run 2: Second run (cache hit)
  Time: 0.00s [CACHED]

Golint-cached speedup: ~25x faster on cache hit

... (more results)
```

## 3. Try Each Wrapper Manually

### Linter Wrapper

```bash
cd test-project/go-code

# First run (cache miss)
time ../../golint-cached/golint-cached

# Second run (cache hit - instant!)
time ../../golint-cached/golint-cached
```

### Protobuf Wrapper

```bash
cd ../proto-files

# First run (cache miss)
time ../../protoc-cached/protoc-cached --go_out=generated *.proto

# Check generated files
ls generated/

# Clean and run again (cache hit)
rm -rf generated
time ../../protoc-cached/protoc-cached --go_out=generated *.proto
```

### Asset Optimizer

```bash
cd ..

# First run (cache miss)
time ../asset-optimizer/asset-optimizer --input=assets --output=dist

# Check optimized files
ls dist/

# Clean and run again (cache hit)
rm -rf dist
time ../asset-optimizer/asset-optimizer --input=assets --output=dist
```

## 4. Create Your Own Wrapper

Copy the template and customize it:

```bash
cp -r wrapper-template my-tool-cached
cd my-tool-cached

# Edit main.go and customize:
# - toolName
# - actualToolName
# - parseToolArgs()
# - cacheResults()
# - restoreResults()

# Build and test
go build
./my-tool-cached [tool arguments]
```

See [wrapper-template/README.md](wrapper-template/README.md) for detailed instructions.

## 5. Integrate Into Your Workflow

### Option A: Replace Commands in Makefile

```makefile
# Before
generate:
	protoc --go_out=. *.proto

# After
generate:
	protoc-cached --go_out=. *.proto
```

### Option B: Use in CI/CD

```yaml
# GitHub Actions
- uses: actions/cache@v3
  with:
    path: .granular-cache
    key: tools-${{ hashFiles('**/*.proto') }}

- run: protoc-cached --go_out=. *.proto
```

### Option C: Shell Wrapper Script

```bash
#!/bin/bash
# wrapper.sh - Use cached version if available

if command -v protoc-cached &> /dev/null; then
    protoc-cached "$@"
else
    protoc "$@"
fi
```

## Expected Performance

| Tool | Cache Miss | Cache Hit | Speedup |
|------|------------|-----------|---------|
| golint-cached | ~2.5s | ~0.01s | **25x** |
| protoc-cached | ~3.5s | ~0.01s | **35x** |
| asset-optimizer (5 files) | ~10s | ~0.01s | **100x** |

## Troubleshooting

### Cache not hitting?

Check if inputs actually match:
```bash
# View cache entries
ls -la .granular-cache/manifests/
```

### Cache too large?

Clear it periodically:
```bash
rm -rf .granular-cache
```

Or add to `.gitignore`:
```bash
echo ".granular-cache/" >> .gitignore
```

## Next Steps

1. Read the full [README.md](README.md) for detailed documentation
2. Check out the [wrapper-template](wrapper-template/) for creating custom wrappers
3. Browse the [test-project](test-project/) for example usage
4. Run [benchmark.sh](benchmark.sh) to see full performance metrics

## More Examples

```bash
# Test cache invalidation
cd test-project/go-code
../../golint-cached/golint-cached          # Cache hit

echo "// Modified" >> main.go              # Modify file
../../golint-cached/golint-cached          # Cache miss!

git checkout main.go                       # Restore file
../../golint-cached/golint-cached          # Cache hit again
```

## Key Features Demonstrated

- **Content-based caching** - Changes to input files invalidate cache
- **Instant cache hits** - 10-100x speedup on repeated runs
- **Drop-in replacement** - Same CLI as original tools
- **Preserves behavior** - Exit codes, stdout, stderr all maintained
- **Easy integration** - Works with Makefiles, CI/CD, scripts

## Resources

- [Main README](README.md) - Full documentation
- [Wrapper Template](wrapper-template/README.md) - Create your own
- [Granular Docs](../../README.md) - Core caching library
- [Benchmark Script](benchmark.sh) - Performance testing

---

**Ready to speed up your builds? Start with the benchmark:**

```bash
./benchmark.sh
```
