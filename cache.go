package granular

import (
	"cmp"
	"fmt"
	"hash"
	"iter"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/afero"
)

// hashPrefixLen is the number of characters from the key hash used for
// two-level directory sharding (e.g., "ab" from "abcdef123...").
const hashPrefixLen = 2

// defaultMaxDataSize is the maximum size for a single decompressed data read (1 GiB).
// Prevents OOM from corrupted or malicious compressed data (gzip/zstd bombs).
const defaultMaxDataSize = 1 << 30

// Cache represents the main cache structure.
// It provides content-addressed storage for files and data.
//
// Lock hierarchy (acquire in this order to prevent deadlocks):
//  1. c.mu        — global RWMutex for bulk operations (Clear, Prune, GC, eviction)
//  2. c.keyLocks  — per-key sharded Mutex for individual entry operations (Get, Put, Delete, Has)
//
// Never acquire c.mu while holding a keyLock.
type Cache struct {
	root             string
	hashFunc         HashFunc
	hashAlgoName     string // Name of the hash algorithm for manifest compatibility
	nowFunc          NowFunc
	mu               sync.RWMutex // Global lock for operations needing consistency (Clear, Stats, Prune, Entries)
	pendingSize      atomic.Int64 // Sum of in-flight Commit sizes, used by eviction to avoid TOCTOU overflows
	keyLocks         *keyLocks    // Per-key locking for concurrent access to different keys
	fs               afero.Fs
	accumulateErrors bool            // If true, accumulate all validation errors; if false, fail-fast
	maxSize          int64           // Maximum cache size in bytes; 0 means no limit
	maxDataSize      int64           // Maximum size for a single decompressed data read; 0 uses defaultMaxDataSize
	compression      CompressionType // Compression algorithm for stored data
	metrics          *MetricsHooks   // Optional metrics hooks for observability
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
		root:         root,
		fs:           afero.NewOsFs(),
		nowFunc:      time.Now,
		hashFunc:     defaultHashFunc,
		hashAlgoName: DefaultHashAlgoName,
		keyLocks:     newKeyLocks(),
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
	// Check for key validation errors first (no lock needed)
	if len(key.errors) > 0 {
		return nil, newValidationError(key.errors)
	}

	// Compute key hash BEFORE locking (pure computation, no lock needed)
	keyHash, err := key.computeHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Use per-key lock for concurrent access to different keys
	c.keyLocks.lockKey(keyHash)
	defer c.keyLocks.unlockKey(keyHash)

	// Check if manifest exists
	manifestPath, err := c.manifestPath(keyHash)
	if err != nil {
		return nil, err
	}
	exists, err := afero.Exists(c.fs, manifestPath)
	if err != nil {
		c.metrics.error("get", err)
		return nil, fmt.Errorf("failed to check manifest: %w", err)
	}
	if !exists {
		c.metrics.miss(keyHash)
		return nil, ErrCacheMiss
	}

	// Load manifest — treat parse failures as corruption and auto-clean
	m, err := c.loadManifest(keyHash)
	if err != nil {
		_ = c.deleteByKeyHash(keyHash)
		c.metrics.error("get", ErrCacheCorrupted)
		return nil, ErrCacheCorrupted
	}

	// Validate hash algorithm compatibility
	// For legacy manifests (version 0) without HashAlgo, assume the default (xxhash64)
	manifestHashAlgo := m.HashAlgo
	if manifestHashAlgo == "" {
		manifestHashAlgo = DefaultHashAlgoName
	}
	if manifestHashAlgo != c.hashAlgoName {
		return nil, ErrHashAlgoMismatch
	}

	// Validate compression type compatibility
	// Reading data with the wrong decompressor would fail or produce garbage.
	if m.Compression != c.compression {
		return nil, ErrCompressionMismatch
	}

	// Verify output hash to detect corruption
	if err := c.verifyOutputHash(m); err != nil {
		// Delete corrupted entry
		_ = c.deleteByKeyHash(keyHash)
		c.metrics.error("get", ErrCacheCorrupted)
		return nil, ErrCacheCorrupted
	}

	// Update access time — best effort, does not affect cache hit validity
	m.AccessedAt = c.now()
	if err := c.saveManifest(m); err != nil {
		c.metrics.error("get:update_access", err)
	}

	// Build result with lazy-loading for data
	// m.OutputData stores paths to .dat files, which are loaded on demand
	result := &Result{
		keyHash:     keyHash,
		cache:       c,
		files:       m.OutputFiles,
		dataPaths:   m.OutputData, // Paths to .dat files for lazy loading
		dataCache:   nil,          // Initialized on first data access
		metadata:    m.OutputMeta,
		compression: m.Compression,
		createdAt:   m.CreatedAt,
		accessedAt:  m.AccessedAt,
	}

	// Initialize maps if nil
	if result.files == nil {
		result.files = make(map[string]string)
	}
	if result.dataPaths == nil {
		result.dataPaths = make(map[string]string)
	}
	if result.metadata == nil {
		result.metadata = make(map[string]string)
	}

	// Report cache hit with entry size
	objectDir, err := c.objectPath(keyHash)
	if err != nil {
		return nil, err
	}
	entrySize, _ := c.dirSize(objectDir)
	c.metrics.hit(keyHash, entrySize)

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
//
// Unlike Get, Has does not update the entry's access time and does not
// verify output hash integrity. It only checks for manifest existence.
//
// Note: Has is advisory — the result may be stale by the time the caller acts on it.
// Another goroutine could delete or overwrite the entry between Has() and a subsequent Get().
// For atomic check-and-use, call Get() directly and handle ErrCacheMiss.
func (c *Cache) Has(key Key) bool {
	if len(key.errors) > 0 {
		return false
	}

	keyHash, err := key.computeHash()
	if err != nil {
		return false
	}

	c.keyLocks.lockKey(keyHash)
	defer c.keyLocks.unlockKey(keyHash)

	manifestPath, err := c.manifestPath(keyHash)
	if err != nil {
		return false
	}
	exists, err := afero.Exists(c.fs, manifestPath)
	return err == nil && exists
}

// Delete removes a cache entry by key.
func (c *Cache) Delete(key Key) error {
	// Compute key hash BEFORE locking (pure computation, no lock needed)
	keyHash, err := key.computeHash()
	if err != nil {
		return fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Use per-key lock for concurrent access to different keys
	c.keyLocks.lockKey(keyHash)
	defer c.keyLocks.unlockKey(keyHash)

	// Get entry size before deleting for metrics
	objectDir, err := c.objectPath(keyHash)
	if err != nil {
		return err
	}
	entrySize, _ := c.dirSize(objectDir)

	if err := c.deleteByKeyHash(keyHash); err != nil {
		c.metrics.error("delete", err)
		return err
	}

	c.metrics.evict(keyHash, entrySize, EvictReasonManual)
	return nil
}

// deleteByKeyHash removes a cache entry by key hash.
// Caller must hold the key lock.
func (c *Cache) deleteByKeyHash(keyHash string) error {
	return c.removeByHash(keyHash)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect entries for metrics before removing
	var entriesToEvict []Entry
	if c.metrics != nil && c.metrics.OnEvict != nil {
		var walkErr error
		entriesToEvict = slices.Collect(c.entriesUnlocked(&walkErr, nil))
	}

	// Remove objects first, then manifests.
	// Orphaned objects (objects without manifests) are recoverable via GC,
	// but orphaned manifests (manifests without objects) cause corrupted reads.
	if err := c.fs.RemoveAll(c.objectsDir()); err != nil {
		c.metrics.error("clear", err)
		return fmt.Errorf("failed to remove objects: %w", err)
	}
	if err := c.fs.RemoveAll(c.manifestDir()); err != nil {
		c.metrics.error("clear", err)
		return fmt.Errorf("failed to remove manifests: %w", err)
	}

	// Recreate directories
	if err := c.fs.MkdirAll(c.manifestDir(), 0o755); err != nil {
		return fmt.Errorf("failed to recreate manifests directory: %w", err)
	}
	if err := c.fs.MkdirAll(c.objectsDir(), 0o755); err != nil {
		return fmt.Errorf("failed to recreate objects directory: %w", err)
	}

	// Report evictions
	for _, entry := range entriesToEvict {
		c.metrics.evict(entry.KeyHash, entry.Size, EvictReasonClear)
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

// ErrInvalidKeyHash is returned when a key hash is too short for sharding.
var ErrInvalidKeyHash = fmt.Errorf("key hash shorter than %d characters", hashPrefixLen)

// manifestPath returns the path to a manifest file for a given key hash.
// Returns an error if the hash is too short for two-level sharding.
func (c *Cache) manifestPath(keyHash string) (string, error) {
	if len(keyHash) < hashPrefixLen {
		return "", fmt.Errorf("%w: %q", ErrInvalidKeyHash, keyHash)
	}
	prefix := keyHash[:hashPrefixLen]
	return filepath.Join(c.manifestDir(), prefix, keyHash+".json"), nil
}

// objectPath returns the path to the object directory for a given key hash.
// Returns an error if the hash is too short for two-level sharding.
func (c *Cache) objectPath(keyHash string) (string, error) {
	if len(keyHash) < hashPrefixLen {
		return "", fmt.Errorf("%w: %q", ErrInvalidKeyHash, keyHash)
	}
	prefix := keyHash[:hashPrefixLen]
	return filepath.Join(c.objectsDir(), prefix, keyHash), nil
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

// evictIfNeeded removes least-recently-accessed entries if adding requiredSpace
// would exceed the cache's maximum size. If maxSize is 0 or negative, this is a no-op.
// Caller must hold the global lock (c.mu).
func (c *Cache) evictIfNeeded(requiredSpace int64) error {
	if c.maxSize <= 0 {
		return nil // No limit set
	}

	// Reject entries that exceed the entire cache size on their own.
	// Check early to avoid an expensive manifest walk and sort.
	if requiredSpace > c.maxSize {
		return fmt.Errorf("entry size %d exceeds max cache size %d", requiredSpace, c.maxSize)
	}

	// Get all entries with their sizes
	var walkErr error
	var corruptedKeys []string
	entries := slices.Collect(c.entriesUnlocked(&walkErr, &corruptedKeys))
	if walkErr != nil {
		return fmt.Errorf("failed to get cache entries for eviction: %w", walkErr)
	}

	c.cleanupCorrupted(corruptedKeys)

	// Calculate current total size
	var currentSize int64
	for _, entry := range entries {
		currentSize += entry.Size
	}

	// Include pending (in-flight) Commit sizes to prevent concurrent
	// Commits from all passing eviction and exceeding maxSize.
	pending := c.pendingSize.Load()
	if currentSize+pending+requiredSpace <= c.maxSize {
		return nil // Enough space
	}

	// Sort by AccessedAt ascending (oldest/least recently accessed first).
	// Use KeyHash as tiebreaker for deterministic eviction when timestamps are equal.
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(
			cmp.Compare(a.AccessedAt.UnixNano(), b.AccessedAt.UnixNano()),
			cmp.Compare(a.KeyHash, b.KeyHash),
		)
	})

	// Evict until we have enough space.
	// Acquire per-key lock for each entry to prevent races with concurrent Get().
	for _, entry := range entries {
		if currentSize+requiredSpace <= c.maxSize {
			break
		}
		c.keyLocks.lockKey(entry.KeyHash)
		if err := c.removeByHash(entry.KeyHash); err != nil {
			c.keyLocks.unlockKey(entry.KeyHash)
			return fmt.Errorf("failed to evict entry %s: %w", entry.KeyHash, err)
		}
		c.keyLocks.unlockKey(entry.KeyHash)
		c.metrics.evict(entry.KeyHash, entry.Size, EvictReasonLRU)
		currentSize -= entry.Size
	}

	return nil
}

// entriesUnlocked returns an iterator over all cache entries without acquiring locks.
// Walk errors are captured in walkErr. Caller must hold at least a read lock on c.mu.
// Corrupted keyHashes are appended to corrupted if non-nil (see manifests()).
func (c *Cache) entriesUnlocked(walkErr *error, corrupted *[]string) iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		for keyHash, m := range c.manifests(walkErr, corrupted) {
			entry := Entry{
				KeyHash:    keyHash,
				CreatedAt:  m.CreatedAt,
				AccessedAt: m.AccessedAt,
				Size:       c.manifestEntrySize(m),
				FileCount:  len(m.OutputFiles) + len(m.OutputData),
			}
			if !yield(entry) {
				return
			}
		}
	}
}

// MaxSize returns the maximum cache size in bytes.
// Returns 0 if no size limit is set.
func (c *Cache) MaxSize() int64 {
	return c.maxSize
}

// effectiveMaxDataSize returns the configured max data size, or the default.
func (c *Cache) effectiveMaxDataSize() int64 {
	if c.maxDataSize > 0 {
		return c.maxDataSize
	}
	return defaultMaxDataSize
}
