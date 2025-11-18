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

	var toRemove []string

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		if m.CreatedAt.Before(cutoff) {
			toRemove = append(toRemove, keyHash)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Remove entries
	for _, keyHash := range toRemove {
		if err := c.removeByHash(keyHash); err != nil {
			return count, fmt.Errorf("failed to remove entry %s: %w", keyHash, err)
		}
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

	var toRemove []string

	err := c.walkManifests(func(keyHash string, m *manifest) error {
		if m.AccessedAt.Before(cutoff) {
			toRemove = append(toRemove, keyHash)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Remove entries
	for _, keyHash := range toRemove {
		if err := c.removeByHash(keyHash); err != nil {
			return count, fmt.Errorf("failed to remove entry %s: %w", keyHash, err)
		}
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
			// Skip corrupted manifests
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
