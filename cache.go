package granular

import (
	"fmt"
	"hash"
	"path/filepath"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/afero"
)

// Cache represents the main cache structure.
// It provides content-addressed storage for files and data.
type Cache struct {
	root             string
	hashFunc         HashFunc
	nowFunc          NowFunc
	mu               sync.RWMutex
	fs               afero.Fs
	accumulateErrors bool // If true, accumulate all validation errors; if false, fail-fast
}

// HashFunc defines a function that creates a new hash.Hash instance.
type HashFunc func() hash.Hash

// NowFunc defines a function that returns the current time.
type NowFunc func() time.Time

// Option defines a function that configures a Cache.
type Option func(*Cache)

// Open creates a new cache at the given root directory.
// The directory will be created if it doesn't exist.
func Open(root string, options ...Option) (*Cache, error) {
	cache := &Cache{
		root:     root,
		fs:       afero.NewOsFs(),
		nowFunc:  time.Now,
		hashFunc: defaultHashFunc,
	}

	// Apply options
	for _, option := range options {
		option(cache)
	}

	// Create cache directories
	if err := cache.fs.MkdirAll(cache.manifestDir(), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create manifests directory: %w", err)
	}
	if err := cache.fs.MkdirAll(cache.objectsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create objects directory: %w", err)
	}

	return cache, nil
}

// OpenTemp creates a temporary in-memory cache for testing.
func OpenTemp() *Cache {
	cache, err := Open("", WithFs(afero.NewMemMapFs()))
	if err != nil {
		panic(fmt.Sprintf("failed to create temp cache: %v", err))
	}
	return cache
}

// Key creates a new KeyBuilder for building cache keys.
func (c *Cache) Key() *KeyBuilder {
	return &KeyBuilder{
		cache:            c,
		inputs:           nil,
		extras:           nil,
		errors:           nil,
		accumulateErrors: c.accumulateErrors,
	}
}

// Get retrieves a cached result for the given key.
// Returns (result, nil) on cache hit.
// Returns (nil, ErrCacheMiss) if the key is not found in the cache.
// Returns (nil, ValidationError) if the key has validation errors.
// Returns (nil, error) for other errors (I/O, corruption, etc.).
func (c *Cache) Get(key Key) (*Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check for key validation errors first
	if len(key.errors) > 0 {
		return nil, newValidationError(key.errors)
	}

	// Compute key hash
	keyHash, err := key.computeHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Check if manifest exists
	manifestPath := c.manifestPath(keyHash)
	exists, err := afero.Exists(c.fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check manifest: %w", err)
	}
	if !exists {
		return nil, ErrCacheMiss
	}

	// Load manifest
	m, err := c.loadManifest(keyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Build result
	result := &Result{
		keyHash:    keyHash,
		cache:      c,
		files:      m.OutputFiles,
		data:       m.OutputData,
		metadata:   m.OutputMeta,
		createdAt:  m.CreatedAt,
		accessedAt: m.AccessedAt,
	}

	// Initialize maps if nil
	if result.files == nil {
		result.files = make(map[string]string)
	}
	if result.data == nil {
		result.data = make(map[string][]byte)
	}
	if result.metadata == nil {
		result.metadata = make(map[string]string)
	}

	return result, nil
}

// Put creates a WriteBuilder for storing a cache entry.
func (c *Cache) Put(key Key) *WriteBuilder {
	// Copy key errors to the write builder
	var errors []error
	if len(key.errors) > 0 {
		errors = append([]error{}, key.errors...)
	}

	return &WriteBuilder{
		cache:            c,
		key:              key,
		files:            nil,
		data:             nil,
		metadata:         nil,
		errors:           errors,
		accumulateErrors: c.accumulateErrors,
	}
}

// Has checks if a key exists in the cache.
// Returns false if the key doesn't exist or if there's an error.
func (c *Cache) Has(key Key) bool {
	result, err := c.Get(key)
	return err == nil && result != nil
}

// Delete removes a cache entry by key.
func (c *Cache) Delete(key Key) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyHash, err := key.computeHash()
	if err != nil {
		return fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Remove manifest
	manifestPath := c.manifestPath(keyHash)
	if exists, _ := afero.Exists(c.fs, manifestPath); exists {
		if err := c.fs.Remove(manifestPath); err != nil {
			return fmt.Errorf("failed to remove manifest: %w", err)
		}
	}

	// Remove object directory
	objectDir := c.objectPath(keyHash)
	if exists, _ := afero.Exists(c.fs, objectDir); exists {
		if err := c.fs.RemoveAll(objectDir); err != nil {
			return fmt.Errorf("failed to remove objects: %w", err)
		}
	}

	return nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove everything
	if err := c.fs.RemoveAll(c.manifestDir()); err != nil {
		return fmt.Errorf("failed to remove manifests: %w", err)
	}
	if err := c.fs.RemoveAll(c.objectsDir()); err != nil {
		return fmt.Errorf("failed to remove objects: %w", err)
	}

	// Recreate directories
	if err := c.fs.MkdirAll(c.manifestDir(), 0o755); err != nil {
		return fmt.Errorf("failed to recreate manifests directory: %w", err)
	}
	if err := c.fs.MkdirAll(c.objectsDir(), 0o755); err != nil {
		return fmt.Errorf("failed to recreate objects directory: %w", err)
	}

	return nil
}

// Close closes the cache and releases any resources.
// Currently a no-op, but provided for future extensibility.
func (c *Cache) Close() error {
	return nil
}

// manifestDir returns the path to the manifests directory.
func (c *Cache) manifestDir() string {
	return filepath.Join(c.root, "manifests")
}

// objectsDir returns the path to the objects directory.
func (c *Cache) objectsDir() string {
	return filepath.Join(c.root, "objects")
}

// manifestPath returns the path to a manifest file for a given key hash.
func (c *Cache) manifestPath(keyHash string) string {
	if len(keyHash) < 2 {
		panic(fmt.Sprintf("key hash too short: %s", keyHash))
	}
	prefix := keyHash[:2]
	return filepath.Join(c.manifestDir(), prefix, keyHash+".json")
}

// objectPath returns the path to the object directory for a given key hash.
func (c *Cache) objectPath(keyHash string) string {
	if len(keyHash) < 2 {
		panic(fmt.Sprintf("key hash too short: %s", keyHash))
	}
	prefix := keyHash[:2]
	return filepath.Join(c.objectsDir(), prefix, keyHash)
}

// newHash creates a new hash instance.
func (c *Cache) newHash() hash.Hash {
	return c.hashFunc()
}

// now returns the current time.
func (c *Cache) now() time.Time {
	return c.nowFunc()
}

// defaultHashFunc returns the default hash function (xxHash64).
func defaultHashFunc() hash.Hash {
	return xxhash.New()
}
