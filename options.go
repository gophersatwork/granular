package granular

import (
	"crypto/sha256"
	"hash"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/afero"
)

// DefaultHashAlgoName is the name of the default hash algorithm (xxhash64).
const DefaultHashAlgoName = "xxhash64"

// WithFs sets a custom filesystem for the cache.
// This is primarily useful for testing with in-memory filesystems.
//
// Example:
//
//	cache, err := granular.Open(".cache", granular.WithFs(afero.NewMemMapFs()))
func WithFs(fs afero.Fs) Option {
	return func(c *Cache) {
		c.fs = fs
	}
}

// WithHashFunc sets a custom hash function for the cache with a named algorithm.
// The name is stored in the manifest to detect algorithm changes.
// The default is xxHash64, which provides excellent performance.
//
// Note: Changing the hash function will cause existing cache entries to be
// treated as misses (ErrHashAlgoMismatch) since the key hash would differ.
//
// Example:
//
//	cache, err := granular.Open(".cache", granular.WithHashFunc("fnv128", fnv.New128))
func WithHashFunc(name string, hashFunc func() hash.Hash) Option {
	return func(c *Cache) {
		c.hashFunc = hashFunc
		c.hashAlgoName = name
	}
}

// WithXXHash configures the cache to use xxHash64 (the default).
// xxHash64 provides excellent performance for cache key hashing.
func WithXXHash() Option {
	return WithHashFunc("xxhash64", func() hash.Hash { return xxhash.New() })
}

// WithSHA256 configures the cache to use SHA-256 for hashing.
// SHA-256 is slower than xxHash64 but provides cryptographic properties.
func WithSHA256() Option {
	return WithHashFunc("sha256", sha256.New)
}

// WithNowFunc sets a custom time function for the cache.
// This is primarily useful for testing with deterministic timestamps.
func WithNowFunc(nowFunc NowFunc) Option {
	return func(c *Cache) {
		c.nowFunc = nowFunc
	}
}

// WithAccumulateErrors configures the cache to accumulate all validation errors
// instead of stopping at the first error (fail-fast).
//
// By default, the cache uses fail-fast mode: validation stops after the first error
// to save computation time. With this option enabled, all inputs are validated and
// all errors are collected and returned together.
//
// This is useful during development to see all validation problems at once.
//
// Example:
//
//	cache, err := granular.Open(".cache", granular.WithAccumulateErrors())
func WithAccumulateErrors() Option {
	return func(c *Cache) {
		c.accumulateErrors = true
	}
}

// WithMaxSize sets the maximum total size of the cache in bytes.
// When the cache exceeds this size, least-recently-accessed entries
// are evicted to make room for new entries.
//
// A value of 0 or negative means no size limit (default behavior).
//
// Example:
//
//	// Create a cache with a 1GB size limit
//	cache, err := granular.Open(".cache", granular.WithMaxSize(1<<30))
func WithMaxSize(bytes int64) Option {
	return func(c *Cache) {
		c.maxSize = bytes
	}
}

// WithCompression sets the compression algorithm for stored data.
// Supported types are CompressionGzip and CompressionZstd.
// CompressionNone (empty string) disables compression (default).
//
// Example:
//
//	cache, err := granular.Open(".cache", granular.WithCompression(granular.CompressionZstd))
func WithCompression(ct CompressionType) Option {
	return func(c *Cache) {
		c.compression = ct
	}
}

// WithMetrics sets the metrics hooks for observability.
// The hooks are called for cache events like hits, misses, puts, and evictions.
// All hooks are optional - nil hooks are ignored.
//
// Example:
//
//	cache, err := granular.Open(".cache", granular.WithMetrics(&granular.MetricsHooks{
//		OnHit: func(keyHash string, size int64) {
//			hitCounter.Inc()
//		},
//		OnMiss: func(keyHash string) {
//			missCounter.Inc()
//		},
//	}))
func WithMetrics(hooks *MetricsHooks) Option {
	return func(c *Cache) {
		c.metrics = hooks
	}
}
