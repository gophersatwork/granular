package granular

import (
	"github.com/spf13/afero"
)

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

// WithHashFunc sets a custom hash function for the cache.
// The default is xxHash64, which provides excellent performance.
// Only change this if you have specific requirements.
//
// Note: Changing the hash function will invalidate existing cache entries.
func WithHashFunc(hashFunc HashFunc) Option {
	return func(c *Cache) {
		c.hashFunc = hashFunc
	}
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
