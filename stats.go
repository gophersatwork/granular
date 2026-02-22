package granular

import (
	"fmt"
	"os"
	"path/filepath"
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

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		stats.Entries++

		// Track oldest and newest
		if oldest.IsZero() || m.CreatedAt.Before(oldest) {
			oldest = m.CreatedAt
		}
		if newest.IsZero() || m.CreatedAt.After(newest) {
			newest = m.CreatedAt
		}

		// Calculate size
		objectDir := c.objectPath(keyHash)
		size, _ := c.dirSize(objectDir)
		stats.TotalSize += size

		return nil
	})
	if err != nil {
		return Stats{}, err
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

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		if m.CreatedAt.Before(cutoff) {
			objectDir := c.objectPath(keyHash)
			size, _ := c.dirSize(objectDir)
			toRemove = append(toRemove, entryToRemove{keyHash: keyHash, size: size})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

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

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		if m.AccessedAt.Before(cutoff) {
			objectDir := c.objectPath(keyHash)
			size, _ := c.dirSize(objectDir)
			toRemove = append(toRemove, entryToRemove{keyHash: keyHash, size: size})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

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

// Entries returns an iterator over all cache entries.
// Note: This holds a read lock during iteration, so process entries quickly.
func (c *Cache) Entries() ([]Entry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var entries []Entry

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		objectDir := c.objectPath(keyHash)
		size, _ := c.dirSize(objectDir)

		entry := Entry{
			KeyHash:    keyHash,
			CreatedAt:  m.CreatedAt,
			AccessedAt: m.AccessedAt,
			Size:       size,
			FileCount:  len(m.OutputFiles) + len(m.OutputData),
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// walkManifests walks all manifest files and calls the function for each.
func (c *Cache) walkManifests(fn func(keyHash string, m *manifest) error) error {
	manifestDir := c.manifestDir()

	return afero.Walk(c.fs, manifestDir, func(path string, info os.FileInfo, err error) error {
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
			// Skip corrupted manifests but report via metrics
			c.metrics.error("walkManifests", fmt.Errorf("corrupted manifest %s: %w", keyHash, err))
			return nil
		}

		return fn(keyHash, m)
	})
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

// GC performs garbage collection on the cache, removing orphaned object directories
// that have no corresponding manifest. This can happen if Put() succeeds writing
// objects but fails writing the manifest (crash, disk full, etc.).
// Returns the number of orphaned directories removed and total bytes reclaimed.
func (c *Cache) GC() (int, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Step 1: Collect all valid object directory hashes from manifests
	validHashes := make(map[string]bool)
	err := c.walkManifests(func(keyHash string, m *manifest) error {
		validHashes[keyHash] = true
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to walk manifests: %w", err)
	}

	// Step 2: Walk the objects directory and find orphans
	objectsDir := c.objectsDir()
	var filesRemoved int
	var bytesReclaimed int64

	// Objects are stored as: objects/{first2chars}/{fullhash}/files
	// Walk the sharded directories
	err = afero.Walk(c.fs, objectsDir, func(path string, info os.FileInfo, err error) error {
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
				filesRemoved++
				bytesReclaimed += size
			}
			return filepath.SkipDir // Don't descend into removed directory
		}

		return filepath.SkipDir // Don't descend into valid directories either
	})
	if err != nil {
		return filesRemoved, bytesReclaimed, fmt.Errorf("failed to walk objects directory: %w", err)
	}

	return filesRemoved, bytesReclaimed, nil
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
