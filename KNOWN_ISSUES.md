# Known Issues and Future Improvements

This is a living document tracking known issues, limitations, and planned improvements for the Granular cache library. These issues are documented for transparency and future prioritization but are not being addressed in the current release.

---

## Serious Issues

### 1. Entire Files Read Into Memory for Hashing

- **Location:** `key.go:47-53` in `fileInput.hash()`
- **Problem:** `afero.ReadFile()` loads the entire file into memory before hashing
- **Impact:** Hashing a 1GB file allocates 1GB of memory; parallel builds can cause out-of-memory errors
- **Recommendation:** Use streaming with the existing `hashFile()` function from `hash.go`

### 2. No Cache Size Limits

- **Problem:** Cache grows unbounded until disk is full
- **Impact:** Production systems run out of disk space over time
- **Recommendation:** Add `WithMaxSize(bytes)` and `WithMaxEntries(n)` options with LRU eviction

### 3. Hash Algorithm Not Recorded in Manifest

- **Location:** `manifest.go`
- **Problem:** Changing `WithHashFunc()` makes old cache entries unreachable without warning
- **Impact:** Silent cache misses after algorithm change
- **Recommendation:** Store hash algorithm identifier in manifest and provide a migration path

---

## Design Issues

### 4. Path-Dependent Keys Break Reproducibility

- **Location:** `key.go:329`
- **Problem:** File paths are included in the hash, not just content
- **Impact:** Same content at different paths produces different cache keys; breaks CI caching across machines
- **Recommendation:** Add an option for content-only addressing

### 5. Glob Walks Are Repeated

- **Location:** `key.go:64-89`, `key.go:200-215`
- **Problem:** `expandGlob()` is called during validation AND during hash computation
- **Impact:** O(n*m) filesystem walks for n globs over m files
- **Recommendation:** Cache glob expansion results during key building

### 6. No Orphan Garbage Collection

- **Problem:** Failed `Put()` operations can leave orphaned object files
- **Impact:** Disk space leak over time
- **Recommendation:** Add a `GC()` method to clean orphaned objects

---

## Missing Features

### 7. No Compression

- **Problem:** Large text-based artifacts (JS bundles, logs) are stored uncompressed
- **Impact:** 5-10x storage overhead for compressible content
- **Recommendation:** Add `WithCompression(algorithm)` option supporting gzip and zstd

### 8. No Remote Backend Support

- **Problem:** Local filesystem only
- **Impact:** Cannot share cache across CI workers or developer machines
- **Recommendation:** Abstract the storage backend and add S3/GCS implementations

### 9. No Cache Warming/Prefetching

- **Problem:** No way to pre-populate cache from a known-good state
- **Recommendation:** Add `Import()` and `Export()` methods for cache snapshots

### 10. No Metrics/Observability

- **Problem:** No way to monitor cache hit rates, sizes, or latencies
- **Recommendation:** Add metrics hooks compatible with Prometheus and OpenTelemetry
