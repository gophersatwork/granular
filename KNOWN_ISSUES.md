# Known Issues and Future Improvements

This is a living document tracking known issues, limitations, and planned improvements for the Granular cache library.

---

## Design Issues

### 1. Path-Dependent Keys Break Reproducibility

- **Location:** `key.go:329`
- **Problem:** File paths are included in the hash, not just content
- **Impact:** Same content at different paths produces different cache keys; breaks CI caching across machines
- **Recommendation:** Add an option for content-only addressing

---

## Missing Features

### 2. No Remote Backend Support

- **Problem:** Local filesystem only
- **Impact:** Cannot share cache across CI workers or developer machines
- **Recommendation:** Abstract the storage backend and add S3/GCS implementations

---

## Recently Fixed

The following issues were addressed and are no longer applicable:

- ~~Entire files read into memory for hashing~~ → Now uses streaming I/O with buffer pooling
- ~~No cache size limits~~ → Added `WithMaxSize()` with LRU eviction
- ~~Hash algorithm not recorded~~ → Manifests now store version and hash algorithm
- ~~No orphan garbage collection~~ → Added `GC()` method
- ~~Glob walks repeated~~ → Glob expansion cached during key building
- ~~No compression~~ → Added `WithCompression()` supporting gzip and zstd
- ~~No cache warming/prefetching~~ → Added `Import()` and `Export()` methods
- ~~No metrics/observability~~ → Added `WithMetrics()` hooks for hit/miss/put/evict events
