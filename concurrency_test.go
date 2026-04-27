package granular

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/spf13/afero"
)

// setupConcurrentCache creates a test cache with some initial data
func setupConcurrentCache(t *testing.T) (*Cache, afero.Fs) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Create test files
	afero.WriteFile(fs, "file1.txt", []byte("content1"), 0o644)
	afero.WriteFile(fs, "file2.txt", []byte("content2"), 0o644)
	afero.WriteFile(fs, "file3.txt", []byte("content3"), 0o644)

	return cache, fs
}

// TestConcurrentReads tests multiple goroutines reading the same key simultaneously
func TestConcurrentReads(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	// Write a cache entry
	key := cache.Key().File("file1.txt").Build()
	err := cache.Put(key).Bytes("output", []byte("test data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Launch multiple concurrent readers
	const numReaders = 10
	var wg sync.WaitGroup

	results := make([]*Result, numReaders)
	errs := make([]error, numReaders)

	for i := range numReaders {
		wg.Go(func() {
			results[i], errs[i] = cache.Get(key)
		})
	}

	wg.Wait()

	// All reads should succeed
	for i := range numReaders {
		if errs[i] != nil {
			t.Errorf("Reader %d got error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("Reader %d got nil result", i)
		}
	}

	// All results should be identical
	for i := 1; i < numReaders; i++ {
		if results[i].keyHash != results[0].keyHash {
			t.Errorf("Reader %d keyHash differs from reader 0", i)
		}
	}
}

// TestConcurrentWrites tests multiple goroutines writing different keys simultaneously
func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	const numWriters = 10
	var wg sync.WaitGroup

	errs := make([]error, numWriters)

	for i := range numWriters {
		wg.Go(func() {
			// Each writer creates a unique key
			key := cache.Key().
				File("file1.txt").
				String("writer", fmt.Sprintf("writer-%d", i)).
				Build()
			errs[i] = cache.Put(key).
				Bytes("output", []byte(fmt.Sprintf("data-%d", i))).
				Commit()
		})
	}

	wg.Wait()

	// All writes should succeed
	for i := range numWriters {
		if errs[i] != nil {
			t.Errorf("Writer %d got error: %v", i, errs[i])
		}
	}

	// Verify all entries exist
	for i := range numWriters {
		key := cache.Key().
			File("file1.txt").
			String("writer", fmt.Sprintf("writer-%d", i)).
			Build()
		if !cache.Has(key) {
			t.Errorf("Entry for writer %d not found", i)
		}
	}
}

// TestReadDuringWrite tests reading a key while another goroutine is writing it
func TestReadDuringWrite(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	key := cache.Key().File("file1.txt").Build()

	// First, populate the cache with an entry
	err := cache.Put(key).Bytes("output", []byte("v1")).Commit()
	if err != nil {
		t.Fatalf("Initial commit failed: %v", err)
	}

	var wg sync.WaitGroup

	var readResult *Result
	var readErr error

	// Reader goroutine
	wg.Go(func() {
		readResult, readErr = cache.Get(key)
	})

	// Writer goroutine (overwrites the same key)
	wg.Go(func() {
		cache.Put(key).Bytes("output", []byte("v2")).Commit()
	})

	wg.Wait()

	// Reader should either get old value or new value, never corruption
	if readErr != nil && !errors.Is(readErr, ErrCacheMiss) {
		t.Errorf("Read error: %v", readErr)
	}

	if readResult != nil {
		data := readResult.Bytes("output")
		if string(data) != "v1" && string(data) != "v2" {
			t.Errorf("Got corrupted data: %q, expected v1 or v2", string(data))
		}
	}
}

// TestDeleteDuringRead tests deleting a key while another goroutine is reading it
func TestDeleteDuringRead(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	key := cache.Key().File("file1.txt").Build()

	// Populate cache
	err := cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var wg sync.WaitGroup

	var readErr error
	var deleteErr error

	// Reader goroutine
	wg.Go(func() {
		_, readErr = cache.Get(key)
	})

	// Deleter goroutine
	wg.Go(func() {
		deleteErr = cache.Delete(key)
	})

	wg.Wait()

	// Delete should succeed
	if deleteErr != nil {
		t.Errorf("Delete error: %v", deleteErr)
	}

	// Read may succeed (got old data) or fail with cache miss, but not other errors
	if readErr != nil && !errors.Is(readErr, ErrCacheMiss) {
		t.Errorf("Unexpected read error: %v", readErr)
	}
}

// TestClearDuringOperations tests clearing cache while other operations are in progress
func TestClearDuringOperations(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	// Populate cache with multiple entries
	for i := range 5 {
		key := cache.Key().
			File("file1.txt").
			String("id", fmt.Sprintf("%d", i)).
			Build()
		err := cache.Put(key).Bytes("output", []byte(fmt.Sprintf("data%d", i))).Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	}

	var wg sync.WaitGroup

	var readErr error
	var writeErr error
	var clearErr error

	// Reader goroutine
	wg.Go(func() {
		key := cache.Key().File("file1.txt").String("id", "0").Build()
		_, readErr = cache.Get(key)
	})

	// Writer goroutine
	wg.Go(func() {
		key := cache.Key().File("file1.txt").String("id", "999").Build()
		writeErr = cache.Put(key).Bytes("output", []byte("new")).Commit()
	})

	// Clear goroutine
	wg.Go(func() {
		clearErr = cache.Clear()
	})

	wg.Wait()

	// Clear should succeed
	if clearErr != nil {
		t.Errorf("Clear error: %v", clearErr)
	}

	// Read and write may or may not succeed depending on timing
	// Important: no corrupted state or panics
	_ = readErr
	_ = writeErr
}

// TestRaceDetectorCoverage runs operations specifically to catch race conditions
func TestRaceDetectorCoverage(t *testing.T) {
	// This test is designed to be run with -race flag
	// It exercises all major code paths concurrently
	t.Parallel()

	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	const numGoroutines = 20
	const iterations = 10

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Go(func() {
			for j := range iterations {
				key := cache.Key().
					File("file1.txt").
					String("g", fmt.Sprintf("%d", i)).
					String("iter", fmt.Sprintf("%d", j)).
					Build()

				// Mix of operations
				switch (i + j) % 4 {
				case 0:
					// Write
					cache.Put(key).Bytes("output", []byte("data")).Commit()
				case 1:
					// Read
					cache.Get(key)
				case 2:
					// Has check
					cache.Has(key)
				case 3:
					// Delete
					cache.Delete(key)
				}
			}
		})
	}

	wg.Wait()
}

// TestConcurrentHashComputation tests concurrent hash computation
func TestConcurrentHashComputation(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	const numGoroutines = 10
	var wg sync.WaitGroup

	hashes := make([]string, numGoroutines)

	// Multiple goroutines computing hash for the same key
	for i := range numGoroutines {
		wg.Go(func() {
			key := cache.Key().File("file1.txt").Build()
			hashes[i], _ = key.computeHash()
		})
	}

	wg.Wait()

	// All hashes should be identical
	for i := 1; i < numGoroutines; i++ {
		if hashes[i] != hashes[0] {
			t.Errorf("Hash %d differs: got %s, want %s", i, hashes[i], hashes[0])
		}
	}
}

// TestConcurrentMultipleKeys tests concurrent operations on different keys
func TestConcurrentMultipleKeys(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	const numKeys = 5
	const numOpsPerKey = 4

	var wg sync.WaitGroup

	errCount := atomic.Int32{}

	for keyID := range numKeys {
		key := cache.Key().
			File("file1.txt").
			String("key", fmt.Sprintf("key-%d", keyID)).
			Build()

		// For each key, launch: write, read, has, delete
		wg.Go(func() {
			if err := cache.Put(key).Bytes("output", []byte("data")).Commit(); err != nil {
				errCount.Add(1)
			}
		})

		wg.Go(func() {
			if _, err := cache.Get(key); err != nil && !errors.Is(err, ErrCacheMiss) {
				errCount.Add(1)
			}
		})

		wg.Go(func() {
			cache.Has(key) // Has doesn't return errors
		})

		wg.Go(func() {
			if err := cache.Delete(key); err != nil {
				errCount.Add(1)
			}
		})
	}

	wg.Wait()

	// Some operations may fail due to timing, but there should be no panics
	// or data corruption
	if count := errCount.Load(); count > int32(numKeys*numOpsPerKey/2) {
		t.Logf("Warning: %d operations failed (may indicate timing issues)", count)
	}
}

// TestConcurrentSameKeyWrites tests multiple goroutines writing to the same key
func TestConcurrentSameKeyWrites(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	key := cache.Key().File("file1.txt").Build()

	const numWriters = 5
	var wg sync.WaitGroup

	for i := range numWriters {
		wg.Go(func() {
			cache.Put(key).
				Bytes("output", []byte(fmt.Sprintf("writer-%d", i))).
				Meta("writer", fmt.Sprintf("%d", i)).
				Commit()
		})
	}

	wg.Wait()

	// Key should exist and have data from one of the writers
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	data := result.Bytes("output")

	// Data should be from one of the writers (not corrupted)
	valid := false
	for i := range numWriters {
		if string(data) == fmt.Sprintf("writer-%d", i) {
			valid = true
			break
		}
	}
	if !valid {
		t.Errorf("Got unexpected data: %s", string(data))
	}
}

// TestConcurrentReadWriteSameKey tests alternating reads and writes
func TestConcurrentReadWriteSameKey(t *testing.T) {
	t.Parallel()
	cache, _ := setupConcurrentCache(t)
	defer cache.Close()

	key := cache.Key().File("file1.txt").Build()

	// Initial value
	cache.Put(key).Bytes("output", []byte("v0")).Commit()

	const numOps = 10
	var wg sync.WaitGroup

	for i := range numOps {
		if i%2 == 0 {
			// Write
			wg.Go(func() {
				cache.Put(key).
					Bytes("output", []byte(fmt.Sprintf("v%d", i))).
					Commit()
			})
		} else {
			// Read
			wg.Go(func() {
				cache.Get(key)
			})
		}
	}

	wg.Wait()

	// Final state should be consistent
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Final Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

// TestConcurrentCommitsRespectMaxSize verifies that concurrent Commits under a tight
// maxSize do not cause the total cache size to exceed the limit by more than one entry.
func TestConcurrentCommitsRespectMaxSize(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	// Each entry is ~100 bytes of data. Allow room for only 5 entries.
	entrySize := 100
	maxSize := int64(entrySize * 5)

	cache, err := Open(".cache", WithFs(fs), WithMaxSize(maxSize))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	// Create a shared source file
	afero.WriteFile(fs, "input.txt", []byte("input"), 0o644)

	const numWriters = 20
	var wg sync.WaitGroup

	for i := range numWriters {
		wg.Go(func() {
			key := cache.Key().
				String("id", fmt.Sprintf("%d", i)).
				Build()
			data := make([]byte, entrySize)
			for j := range data {
				data[j] = byte(i)
			}
			cache.Put(key).Bytes("payload", data).Commit()
		})
	}

	wg.Wait()

	// Measure total cache size
	stats, err := cache.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	// Allow one entry of slack since eviction is estimated pre-compression.
	limit := maxSize + int64(entrySize)
	if stats.TotalSize > limit {
		t.Fatalf("Cache size %d exceeds maxSize %d + one entry slack %d (limit %d)",
			stats.TotalSize, maxSize, entrySize, limit)
	}
}
