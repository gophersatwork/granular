package granular

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// Stats represents cache statistics.
type Stats struct {
	Entries     int           // Total number of cache entries
	TotalSize   int64         // Total size of all cached files in bytes
	OldestEntry time.Duration // Age of the oldest entry
	NewestEntry time.Duration // Age of the newest entry
}

// Entry represents a single cache entry for iteration.
type Entry struct {
	KeyHash    string
	CreatedAt  time.Time
	AccessedAt time.Time
	Size       int64
	FileCount  int
}

// Stats returns statistics about the cache.
func (c *Cache) Stats() (Stats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := Stats{}
	var oldest, newest time.Time

	var walkErr error
	for _, m := range c.manifests(&walkErr, nil) {
		stats.Entries++

		// Track oldest and newest
		if oldest.IsZero() || m.CreatedAt.Before(oldest) {
			oldest = m.CreatedAt
		}
		if newest.IsZero() || m.CreatedAt.After(newest) {
			newest = m.CreatedAt
		}

		// Calculate size from manifest file references to avoid O(N^2) directory walks.
		stats.TotalSize += c.manifestEntrySize(m)
	}
	if walkErr != nil {
		return Stats{}, walkErr
	}

	now := c.now()
	if !oldest.IsZero() {
		stats.OldestEntry = now.Sub(oldest)
	}
	if !newest.IsZero() {
		stats.NewestEntry = now.Sub(newest)
	}

	return stats, nil
}

// Prune removes cache entries older than the given duration.
// Returns the number of entries removed.
func (c *Cache) Prune(olderThan time.Duration) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	cutoff := c.now().Add(-olderThan)

	type entryToRemove struct {
		keyHash string
		size    int64
	}
	var toRemove []entryToRemove

	var walkErr error
	var corruptedKeys []string
	for keyHash, m := range c.manifests(&walkErr, &corruptedKeys) {
		if m.CreatedAt.Before(cutoff) {
			toRemove = append(toRemove, entryToRemove{keyHash: keyHash, size: c.manifestEntrySize(m)})
		}
	}
	if walkErr != nil {
		return 0, walkErr
	}

	c.cleanupCorrupted(corruptedKeys)

	// Remove entries, acquiring per-key lock for each to prevent races with concurrent Get()
	for _, entry := range toRemove {
		c.keyLocks.lockKey(entry.keyHash)
		if err := c.removeByHash(entry.keyHash); err != nil {
			c.keyLocks.unlockKey(entry.keyHash)
			return count, fmt.Errorf("failed to remove entry %s: %w", entry.keyHash, err)
		}
		c.keyLocks.unlockKey(entry.keyHash)
		c.metrics.evict(entry.keyHash, entry.size, EvictReasonExpired)
		count++
	}

	return count, nil
}

// PruneUnused removes cache entries not accessed since the given duration.
// Returns the number of entries removed.
func (c *Cache) PruneUnused(notAccessedSince time.Duration) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := 0
	cutoff := c.now().Add(-notAccessedSince)

	type entryToRemove struct {
		keyHash string
		size    int64
	}
	var toRemove []entryToRemove

	var walkErr error
	var corruptedKeys []string
	for keyHash, m := range c.manifests(&walkErr, &corruptedKeys) {
		if m.AccessedAt.Before(cutoff) {
			toRemove = append(toRemove, entryToRemove{keyHash: keyHash, size: c.manifestEntrySize(m)})
		}
	}
	if walkErr != nil {
		return 0, walkErr
	}

	c.cleanupCorrupted(corruptedKeys)

	// Remove entries, acquiring per-key lock for each to prevent races with concurrent Get()
	for _, entry := range toRemove {
		c.keyLocks.lockKey(entry.keyHash)
		if err := c.removeByHash(entry.keyHash); err != nil {
			c.keyLocks.unlockKey(entry.keyHash)
			return count, fmt.Errorf("failed to remove entry %s: %w", entry.keyHash, err)
		}
		c.keyLocks.unlockKey(entry.keyHash)
		c.metrics.evict(entry.keyHash, entry.size, EvictReasonExpired)
		count++
	}

	return count, nil
}

// Entries returns all cache entries as a slice.
func (c *Cache) Entries() ([]Entry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var walkErr error
	entries := slices.Collect(c.entriesUnlocked(&walkErr, nil))
	if walkErr != nil {
		return nil, walkErr
	}
	return entries, nil
}

// EntriesIter returns an iterator over all cache entries.
// It holds a read lock during iteration, released when the iterator
// completes or the caller breaks. Walk errors are silently skipped;
// use Entries() for explicit error handling.
func (c *Cache) EntriesIter() iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		c.mu.RLock()
		defer c.mu.RUnlock()

		var walkErr error
		for entry := range c.entriesUnlocked(&walkErr, nil) {
			if !yield(entry) {
				return
			}
		}
	}
}

// errStopWalk is a sentinel error used to break out of afero.Walk
// when the iterator consumer stops early.
var errStopWalk = errors.New("stop walk")

// manifests returns an iterator over all manifest files in the cache.
// Walk errors are captured in walkErr. Corrupted manifest keyHashes are
// appended to corrupted (if non-nil) and skipped. Callers holding a write
// lock should pass a non-nil slice and clean up corrupted entries after
// iteration. Callers holding only a read lock should pass nil.
func (c *Cache) manifests(walkErr *error, corrupted *[]string) iter.Seq2[string, *manifest] {
	return func(yield func(string, *manifest) bool) {
		manifestDir := c.manifestDir()

		err := afero.Walk(c.fs, manifestDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Only process .json files
			if !strings.HasSuffix(path, ".json") {
				return nil
			}

			// Extract key hash from filename
			keyHash := strings.TrimSuffix(filepath.Base(path), ".json")

			// Load manifest
			m, err := c.loadManifest(keyHash)
			if err != nil {
				c.metrics.error("manifests", fmt.Errorf("corrupted manifest %s: %w", keyHash, err))
				if corrupted != nil {
					*corrupted = append(*corrupted, keyHash)
				}
				return nil
			}

			if !yield(keyHash, m) {
				return errStopWalk
			}

			return nil
		})
		if err != nil && !errors.Is(err, errStopWalk) {
			*walkErr = err
		}
	}
}

// cleanupCorrupted removes corrupted manifests and their objects.
// Caller must hold the global write lock (c.mu).
func (c *Cache) cleanupCorrupted(keyHashes []string) {
	for _, keyHash := range keyHashes {
		c.keyLocks.lockKey(keyHash)
		_ = c.removeByHash(keyHash)
		c.keyLocks.unlockKey(keyHash)
	}
}

// manifestEntrySize computes the size of a cache entry by statting the files
// referenced in the manifest. This avoids a full directory walk per entry.
func (c *Cache) manifestEntrySize(m *manifest) int64 {
	var size int64
	for path := range maps.Values(m.OutputFiles) {
		if info, err := c.fs.Stat(path); err == nil {
			size += info.Size()
		}
	}
	for path := range maps.Values(m.OutputData) {
		if info, err := c.fs.Stat(path); err == nil {
			size += info.Size()
		}
	}
	return size
}

// dirSize calculates the total size of all files in a directory.
func (c *Cache) dirSize(dir string) (int64, error) {
	var size int64

	err := afero.Walk(c.fs, dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// removeByHash removes a cache entry by its key hash.
func (c *Cache) removeByHash(keyHash string) error {
	// Remove manifest
	mPath, err := c.manifestPath(keyHash)
	if err != nil {
		return err
	}
	if exists, _ := afero.Exists(c.fs, mPath); exists {
		if err := c.fs.Remove(mPath); err != nil {
			return fmt.Errorf("failed to remove manifest: %w", err)
		}
	}

	// Remove object directory
	objectDir, err := c.objectPath(keyHash)
	if err != nil {
		return err
	}
	if exists, _ := afero.Exists(c.fs, objectDir); exists {
		if err := c.fs.RemoveAll(objectDir); err != nil {
			return fmt.Errorf("failed to remove objects: %w", err)
		}
	}

	return nil
}

// GC performs garbage collection on the cache, removing orphaned object directories
// that have no corresponding manifest. This can happen if Put() succeeds writing
// objects but fails writing the manifest (crash, disk full, etc.).
// Returns the number of orphaned directories removed and total bytes reclaimed.
func (c *Cache) GC() (int, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Step 1: Collect all valid object directory hashes from manifests
	validHashes := make(map[string]bool)
	var walkErr error
	var corruptedKeys []string
	for keyHash := range c.manifests(&walkErr, &corruptedKeys) {
		validHashes[keyHash] = true
	}
	if walkErr != nil {
		return 0, 0, fmt.Errorf("failed to walk manifests: %w", walkErr)
	}

	c.cleanupCorrupted(corruptedKeys)

	// Step 2: Walk the objects directory and find orphans
	objectsDir := c.objectsDir()
	var dirsRemoved int
	var bytesReclaimed int64

	// Objects are stored as: objects/{first2chars}/{fullhash}/files
	// Walk the sharded directories
	err := afero.Walk(c.fs, objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip errors (e.g., permission denied)
			return nil
		}

		// We're looking for hash directories (the ones containing actual files)
		// Path structure: objects/ab/abcd1234.../
		if !info.IsDir() {
			return nil
		}

		// Extract hash from path
		hash := extractHashFromPath(path, objectsDir)
		if hash == "" {
			return nil // Not a hash directory (might be shard directory or root)
		}

		// Check if this hash has a corresponding manifest
		if !validHashes[hash] {
			// Orphan! Remove it
			size, _ := c.dirSize(path)
			if removeErr := c.fs.RemoveAll(path); removeErr == nil {
				dirsRemoved++
				bytesReclaimed += size
			}
			return filepath.SkipDir // Don't descend into removed directory
		}

		return filepath.SkipDir // Don't descend into valid directories either
	})
	if err != nil {
		return dirsRemoved, bytesReclaimed, fmt.Errorf("failed to walk objects directory: %w", err)
	}

	return dirsRemoved, bytesReclaimed, nil
}

// extractHashFromPath extracts the key hash from an object directory path.
// Path format: .cache/objects/ab/abcdef123456...
// Returns empty string if the path is not at the correct depth (shard/hash).
func extractHashFromPath(path, objectsDir string) string {
	rel, err := filepath.Rel(objectsDir, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 {
		return "" // Not at the right depth (shard directory or too deep)
	}
	// parts[0] is the shard (e.g., "ab"), parts[1] is the full hash
	return parts[1]
}
