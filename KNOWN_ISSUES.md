# Known Issues and Future Improvements

This is a living document tracking known issues, limitations, and planned improvements for the Granular cache library.

---

## Design Issues

### 1. Path-Dependent Keys Break Reproducibility

- **Location:** `key.go:329`
- **Problem:** File paths are included in the hash, not just content
- **Impact:** Same content at different paths produces different cache keys; breaks CI caching across machines
- **Recommendation:** Add an option for content-only addressing

### 2. Glob Walks Are Repeated

- **Location:** `key.go:64-89`, `key.go:200-215`
- **Problem:** `expandGlob()` is called during validation AND during hash computation
- **Impact:** O(n*m) filesystem walks for n globs over m files
- **Recommendation:** Cache glob expansion results during key building

---

## Missing Features

### 3. No Compression

- **Problem:** Large text-based artifacts (JS bundles, logs) are stored uncompressed
- **Impact:** 5-10x storage overhead for compressible content
- **Recommendation:** Add `WithCompression(algorithm)` option supporting gzip and zstd

### 4. No Remote Backend Support

- **Problem:** Local filesystem only
- **Impact:** Cannot share cache across CI workers or developer machines
- **Recommendation:** Abstract the storage backend and add S3/GCS implementations

### 5. No Cache Warming/Prefetching

- **Problem:** No way to pre-populate cache from a known-good state
- **Recommendation:** Add `Import()` and `Export()` methods for cache snapshots

### 6. No Metrics/Observability

- **Problem:** No way to monitor cache hit rates, sizes, or latencies
- **Recommendation:** Add metrics hooks compatible with Prometheus and OpenTelemetry

---

## Recently Fixed

The following issues were addressed and are no longer applicable:

- ~~Entire files read into memory for hashing~~ → Now uses streaming I/O with buffer pooling
- ~~No cache size limits~~ → Added `WithMaxSize()` with LRU eviction
- ~~Hash algorithm not recorded~~ → Manifests now store version and hash algorithm
- ~~No orphan garbage collection~~ → Added `GC()` method
