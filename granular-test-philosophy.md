# Granular Testing Philosophy

## Overview

This document explains **why** we test specific areas of the Granular cache library with particular testing techniques. Understanding the rationale behind our testing strategy helps maintain high code quality and guides future testing decisions.

## Core Principles

### 1. Meaningful Tests Over Coverage Metrics

We aim for **85%+ coverage**, but prioritize meaningful tests that catch real bugs over achieving 100% coverage. Tests should:
- Verify behavior, not implementation details
- Catch regression bugs
- Document expected behavior
- Fail when code changes break contracts

### 2. Match Testing Technique to Risk Profile

Different code has different risk profiles. We use specialized testing techniques where they provide the most value:
- **Property-based tests**: For invariants that must hold for all inputs
- **Fuzz tests**: For parsing and input validation
- **Concurrency tests**: For thread-safety guarantees
- **Benchmarks**: For performance-critical paths

### 3. Fail Fast, Learn Fast

Tests should:
- Run quickly in CI/CD pipelines
- Provide clear error messages
- Use deterministic test data when possible
- Isolate failures to specific components

---

## Why We Test What We Test

### Validation & Error Handling (`validation_test.go`)

**Why Critical:**
- Error handling is a **primary feature** of Granular, not an edge case
- The `WithAccumulateErrors()` option changes behavior fundamentally
- Validation errors affect user experience directly
- Go 1.20+ multi-error unwrapping must work correctly with `errors.Is()` and `errors.As()`

**Why These Techniques:**
- **Unit tests**: Verify error formatting, unwrapping, and accumulation logic
- **Integration tests**: Test validation across the full key-building pipeline
- **State-based testing**: Verify fail-fast vs accumulate-all behavior differences

**Real-World Impact:**
If validation errors are broken, users get:
- Confusing error messages
- Missing context about multiple failures
- Inability to use standard Go error handling patterns

---

### Glob Pattern Matching (`glob_test.go`)

**Why Critical:**
- Glob matching has **0% coverage** in matchGlobParts/matchesGlobPattern
- Recursive `**` patterns are complex with many edge cases
- Bugs cause silent failures (wrong files cached or missed)
- Used in build pipelines where correctness is essential

**Why These Techniques:**
- **Edge case testing**: Cover `**`, `**/`, empty matches, deep nesting
- **Property-based testing**: Verify glob results are always subset of directory walk
- **Fuzz testing**: Find unexpected pattern/path combinations that break logic

**Real-World Impact:**
Broken glob matching means:
- Build tools cache wrong files
- Security-sensitive files accidentally included
- Builds aren't reproducible
- Silent data corruption in cached results

**Why Property Tests:**
The invariant "glob matches ⊆ all files" must hold for **every** pattern. A single counterexample breaks the contract. Property tests efficiently search for counterexamples.

---

### Concurrency Safety (`concurrency_test.go`)

**Why Critical:**
- Granular is designed for use in concurrent build systems
- Race conditions cause **non-deterministic failures**
- Data corruption from races can silently break builds
- RWMutex usage must be correct (deadlocks are catastrophic)

**Why testing/synctest (Go 1.25):**
- **Deterministic concurrency testing**: Traditional concurrent tests are flaky
- **Controlled scheduling**: synctest.Run() controls goroutine interleaving
- **Reproducible failures**: Race conditions become consistently reproducible
- **No sleeps or timeouts**: Tests run fast and reliably

**Scenarios Tested:**
1. **Concurrent reads**: Multiple goroutines reading same key (RLock correctness)
2. **Concurrent writes**: Multiple goroutines writing different keys (no contention)
3. **Read during write**: Verify no partial/corrupted reads
4. **Delete during read**: Ensure safe cleanup
5. **Clear during operations**: Test bulk deletion safety
6. **Prune during operations**: Background maintenance doesn't corrupt state

**Real-World Impact:**
Race conditions cause:
- Builds that pass sometimes, fail others (Heisenbug hell)
- Corrupted cache manifests
- Deadlocks that freeze CI/CD pipelines
- Data races that pass tests but fail in production

---

### Configuration Options (`options_test.go`)

**Why Critical:**
- Options had **0% coverage** (WithHashFunc, WithNowFunc)
- Changing hash function invalidates entire cache
- Custom time functions affect cache expiration and pruning
- Wrong option behavior breaks cache semantics

**Why These Techniques:**
- **Functional testing**: Verify each option changes cache behavior
- **Combination testing**: Options must compose correctly
- **Deterministic testing**: WithNowFunc enables reproducible time-based tests

**Real-World Impact:**
Broken options cause:
- Cache invalidation after updates (all entries become misses)
- Incorrect pruning (keeping too much or too little)
- Flaky tests due to time-based behavior
- Inability to test cache in isolation

**Design Insight:**
WithNowFunc() and WithHashFunc() are **dependency injection** for testability. We test that the injection works correctly, enabling deterministic testing of cache behavior.

---

### Property-Based Tests (`property_test.go`)

**Why Property Testing:**
Property tests verify **invariants** that must hold for **all** inputs, not just examples. They're ideal for Granular because:

1. **Hash determinism**: Same inputs → same hash (always)
2. **Idempotency**: Put(K,V); Put(K,V) → Get(K) == V
3. **Round-trip**: Save → Load preserves all data
4. **Immutability**: Bytes() copies data (no external mutation)

**Why testing/quick (stdlib):**
- **No external dependencies**: Keeps codebase lean
- **Good-enough fuzzing**: 100-1000 iterations catch most bugs
- **Simple API**: Easy for contributors to add property tests
- **Fast execution**: Quick tests run in CI without timeout issues

**What Property Tests Catch:**
- **Hash collisions**: Different inputs producing same hash
- **Mutation bugs**: Data corruption via aliasing
- **Serialization bugs**: Lost data in JSON round-trip
- **Order sensitivity**: Behavior depending on map iteration order (bad!)

**Example:**
```go
// Property: Hash is deterministic
property := func(seed int64) bool {
    // For any random inputs...
    hash1 := computeHash(inputs)
    hash2 := computeHash(inputs)
    return hash1 == hash2  // Always equal
}
```

If this property fails even once, hash determinism is broken, and caching becomes unreliable.

---

### Fuzz Testing (`fuzz_test.go`)

**Why Fuzz Testing:**
Fuzzing finds **unexpected inputs** that developers don't think to test. Critical for:

1. **Manifest JSON parsing**: Untrusted data from filesystem
2. **Glob pattern parsing**: User-provided patterns with complex rules
3. **Key hashing**: Arbitrary file paths and metadata

**Why Native Go Fuzzing:**
- **Built into Go 1.18+**: No external tools
- **Coverage-guided**: Intelligently explores code paths
- **Corpus management**: Automatically saves interesting inputs
- **CI integration**: Can run continuously to find bugs

**What Fuzzing Catches:**
- **Panic on malformed JSON**: Corrupted manifest files
- **Infinite loops**: Pathological glob patterns like `**/**/**/**`
- **Buffer overflows**: (Impossible in Go, but finds logic errors)
- **Unexpected nil dereferences**: Missing nil checks
- **String escaping bugs**: Special characters in paths

**Fuzzing Targets:**

#### 1. Manifest JSON (`FuzzManifestJSON`)
**Why**: Manifest files are persisted to disk and can be:
- Corrupted by filesystem errors
- Maliciously crafted
- Truncated mid-write
- Contain very large values (DoS potential)

**Goal**: Never panic, always return error for invalid JSON

#### 2. Glob Patterns (`FuzzGlobPattern`, `FuzzGlobMatching`)
**Why**: User-provided patterns can be:
- Maliciously complex (`****/****/****/`)
- Contain special regex characters (`[`, `]`, `\`)
- Have platform-specific path separators
- Be empty or have only wildcards

**Goal**: Never infinite loop, never panic, handle all syntax gracefully

#### 3. Cache Operations (`FuzzCachePutGet`)
**Why**: End-to-end fuzzing finds interactions between components:
- Special characters in filenames
- Binary data in metadata
- Large data blobs

**Goal**: Round-trip always preserves data, no corruption

**Running Fuzzing:**
```bash
# Run for 1 minute
go test -fuzz=FuzzManifestJSON -fuzztime=1m

# Run overnight in CI
go test -fuzz=. -fuzztime=8h
```

---

### Benchmarks (`benchmark_test.go`)

**Why Benchmarking:**
Granular is a **performance-sensitive** library. Slow caching defeats the purpose of caching. Benchmarks:
- Establish performance baselines
- Detect regressions in PRs
- Guide optimization efforts
- Verify buffer pooling works

**Critical Paths Benchmarked:**

#### 1. Cache Get/Put Operations
**Why**: These are called on **every build operation**. Even small slowdowns multiply across large projects.

- `BenchmarkCacheGet_Hit`: Measures hot path (cache effectiveness)
- `BenchmarkCacheGet_Miss`: Measures overhead of miss detection
- `BenchmarkCachePut_*`: Measures write performance at different scales

#### 2. Key Hash Computation
**Why**: Hash is computed on **every cache lookup**. Slow hashing = slow builds.

- `BenchmarkKeyHash_SingleFile`: Baseline performance
- `BenchmarkKeyHash_Glob100Files`: Scales to real projects
- `BenchmarkKeyHash_LargeFile`: Tests buffer pooling efficiency

**Buffer Pool Insight**: We use `sync.Pool` for file I/O buffers. Benchmarks verify this reduces allocations (B/op should be low).

#### 3. Concurrent Operations
**Why**: Build systems are highly parallel. Contention on locks kills performance.

- `BenchmarkConcurrentReads`: Should scale linearly (RWMutex works)
- `BenchmarkConcurrentWrites`: Measures lock contention
- `BenchmarkConcurrentMixedOperations`: Realistic workload

**What to Watch:**
- **ns/op**: Time per operation (lower is better)
- **B/op**: Bytes allocated (lower is better, indicates less GC pressure)
- **allocs/op**: Number of allocations (fewer = better)
- **MB/s**: Throughput for file hashing (higher is better)

**Performance Targets:**
- Cache hit: < 100µs (microseconds)
- Cache miss: < 500µs
- Hash 1MB file: > 500 MB/s
- Concurrent reads: near-linear scaling

---

## Testing Strategy Decision Tree

### When to Use Each Technique

```
Is it parsing external input?
├─ YES → Fuzz test
└─ NO
    │
    Does it have an invariant that must always hold?
    ├─ YES → Property test
    └─ NO
        │
        Is it performance-critical?
        ├─ YES → Benchmark
        └─ NO
            │
            Is it concurrent?
            ├─ YES → testing/synctest
            └─ NO
                │
                Regular unit/integration test
```

### Coverage Goals by Component

| Component | Target Coverage | Rationale |
|-----------|----------------|-----------|
| Error handling | 100% | User-facing, critical paths |
| Glob matching | 100% | Complex logic, many edge cases |
| Concurrency | 100% (scenarios) | Race conditions are catastrophic |
| Options | 100% | Small surface area, easy to test |
| Cache operations | 95% | Core functionality |
| Stats/Prune | 90% | Less critical, background operations |
| Close() | Can be 0% | Currently a no-op |

---

## Anti-Patterns We Avoid

### ❌ Testing Implementation Details
**Bad:**
```go
// Testing internal hash buffer size
if len(cache.hashBuffer) != 8192 { ... }
```

**Good:**
```go
// Testing hash correctness
if hash1 != hash2 { ... }
```

### ❌ Flaky Time-Based Tests
**Bad:**
```go
time.Sleep(100 * time.Millisecond)
// Hope operation completed...
```

**Good:**
```go
cache, _ := Open(".cache", WithFs(fs), WithNowFunc(fixedTime))
// Deterministic time for testing
```

### ❌ Overly Specific Assertions
**Bad:**
```go
if err.Error() != "validation failed: file does not exist: /exact/path/file.txt" { ... }
```

**Good:**
```go
var ve *ValidationError
if !errors.As(err, &ve) || !strings.Contains(ve.Error(), "file.txt") { ... }
```

### ❌ Ignoring Race Detector
Always run:
```bash
go test -race
```

Race detector catches bugs tests miss.

---

## Continuous Testing

### CI/CD Pipeline

```yaml
# Every commit
- go test -v -race -coverprofile=coverage.out
- go test -bench=. -benchtime=100ms

# Nightly
- go test -fuzz=. -fuzztime=1h
- go test -bench=. -benchtime=10s -benchmem

# Monthly
- Review coverage reports
- Update fuzz corpus
- Check benchmark trends
```

### Coverage Reporting

We use standard Go coverage:
```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Coverage is a metric, not a goal**. 85% coverage with meaningful tests beats 100% coverage with weak assertions.

---

## Contributing Tests

### Adding a New Test

1. **Choose the right file**:
   - Validation errors → `validation_test.go`
   - Glob patterns → `glob_test.go`
   - Concurrency → `concurrency_test.go`
   - New invariant → `property_test.go`
   - Performance → `benchmark_test.go`

2. **Write a clear test name**:
   - `TestValidationError_EmptyErrors` ✅
   - `TestVE` ❌

3. **Document why the test exists**:
   ```go
   // TestConcurrentReadWrite verifies that concurrent reads during
   // a write operation never return corrupted data. This is critical
   // for build systems that parallelize cache access.
   func TestConcurrentReadWrite(t *testing.T) { ... }
   ```

4. **Add to relevant section** of this document

---

## Testing Philosophy in Practice

### Example: Hash Determinism

**Why it matters:**
If hash is non-deterministic, cache becomes useless. Same inputs must produce same hash **every single time**.

**How we test:**
1. **Unit test**: Hash file twice, compare results
2. **Property test**: Hash random inputs 100 times, all identical
3. **Integration test**: Put → Get → Put → Get, verify hashes match

**Why multiple approaches:**
- Unit test: Fast, runs on every commit
- Property test: Explores input space, finds edge cases
- Integration test: Verifies real-world usage

This **defense in depth** catches bugs that single-technique testing would miss.

---

## Lessons Learned

### 1. Glob Matching is Harder Than It Looks
Initially had 0% coverage on `matchGlobParts` because we thought it was "obviously correct." Fuzz testing found edge cases with `**` at the end of patterns and empty path components.

**Lesson**: Complex recursive algorithms need exhaustive testing.

### 2. Concurrency Bugs Are Sneaky
Traditional concurrent tests using `sync.WaitGroup` were flaky. Only after switching to `testing/synctest` did we get reproducible concurrency tests.

**Lesson**: Use modern tooling for hard problems.

### 3. Validation Errors Are First-Class
Users spend more time debugging validation errors than any other part of the API. Investing in clear, well-tested error messages pays dividends in support time saved.

**Lesson**: Error handling is a feature, not an afterthought.

---

## Future Testing Directions

### Potential Additions

1. **Mutation testing**: Use `go-mutesting` to verify tests catch intentional bugs
2. **Chaos testing**: Randomly inject filesystem errors, verify graceful degradation
3. **Load testing**: Simulate cache with 100k+ entries, measure performance
4. **Integration tests**: Test with real tools (protoc, golangci-lint)

### Metrics to Track

- **Coverage trend**: Should stay above 85%
- **Benchmark trend**: Should not regress > 5%
- **Fuzz crash count**: Should be zero
- **Race detector warnings**: Should be zero

---

## Conclusion

Testing Granular comprehensively requires multiple techniques because the library has diverse risk profiles:

- **Correctness-critical**: Validation, glob matching → exhaustive unit + property tests
- **Concurrency-critical**: Cache operations → testing/synctest
- **Security-critical**: Input parsing → fuzz testing
- **Performance-critical**: Hash computation → benchmarks

By matching testing technique to risk, we achieve high confidence without excessive test maintenance burden.

**The goal**: Ship a cache library that users can trust in production build systems handling millions of dollars of engineering time.

Tests are the contract that guarantees this trust.
