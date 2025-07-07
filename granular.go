package granular

import (
	"hash"
	"path/filepath"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/afero"
)

// HashFunc defines a function that creates a new hash.Hash instance.
type HashFunc func() hash.Hash

// NowFunc defines a function that return the time now.
type NowFunc func() time.Time

// Cache represents the main cache structure.
// It manages the storage and retrieval of cached artifacts.
type Cache struct {
	root string       // Root directory for the cache
	hash hash.Hash    // Hash function used for content addressing
	now  NowFunc      // time function to decide what time is now
	mu   sync.RWMutex // Mutex for concurrent access
	fs   afero.Fs     // Filesystem interface
}

// New creates a new cache instance with the given root directory.
// It uses the default hash function (xxHash) if none is provided.
// It uses the OS filesystem by default, but this can be overridden with WithFs.
func New(root string, options ...Option) (*Cache, error) {
	cache := &Cache{
		root: root,
		fs:   afero.NewOsFs(),   // Default to OS filesystem
		now:  time.Now,          // Default to stdlib time.Now
		hash: defaultHashFunc(), // Default hash provider
	}

	// Apply options
	for _, option := range options {
		option(cache)
	}

	// Create cache directories if they don't exist
	if err := cache.fs.MkdirAll(cache.manifestDir(), 0o755); err != nil {
		return nil, err
	}
	if err := cache.fs.MkdirAll(cache.objectsDir(), 0o755); err != nil {
		return nil, err
	}

	return cache, nil
}

// manifestDir returns the path to the manifests directory.
func (c *Cache) manifestDir() string {
	return filepath.Join(c.root, "manifests")
}

// objectsDir returns the path to the objects directory.
func (c *Cache) objectsDir() string {
	return filepath.Join(c.root, "objects")
}

// Input defines the interface for cache inputs.
// Inputs are used to calculate the cache key.
type Input interface {
	// Hash writes the input's content to the hash.
	Hash(h hash.Hash) error

	// String returns a string representation of the input.
	String() string
}

// Key represents a cache key composed of inputs and extra metadata.
type Key struct {
	Inputs []Input           // Files, globs, raw data
	Extra  map[string]string // Additional cache key components
}

// Option defines a function that configures a Cache.
type Option func(*Cache)

// WithHashFunc sets the hash function for the cache.
func WithHashFunc(hashFunc HashFunc) Option {
	return func(c *Cache) {
		c.hash = hashFunc()
	}
}

// WithNowFunc sets the Now() function for the cache.
func WithNowFunc(nowFunc NowFunc) Option {
	return func(c *Cache) {
		c.now = nowFunc
	}
}

// WithFs sets the filesystem implementation for the cache.
// This allows using different filesystem implementations like in-memory
// filesystems for testing or remote filesystems.
func WithFs(fs afero.Fs) Option {
	return func(c *Cache) {
		c.fs = fs
	}
}

func defaultHashFunc() hash.Hash {
	return xxhash.New()
}
