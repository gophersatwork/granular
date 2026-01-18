package granular

import (
	"errors"
	"hash"
	"hash/fnv"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// TestWithHashFunc tests custom hash function option
func TestWithHashFunc(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Create cache with custom hash function (FNV)
	customHashFunc := func() hash.Hash {
		return fnv.New64a()
	}

	cache, err := Open(".cache", WithFs(fs), WithHashFunc(customHashFunc))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Create a key and get its hash
	key := cache.Key().File("test.txt").Build()
	hash1 := key.Hash()

	if hash1 == "" {
		t.Fatal("Hash should not be empty")
	}

	// Create another cache with default hash (xxHash)
	cacheDefault, err := Open(".cache2", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cacheDefault *Cache) {
		err := cacheDefault.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cacheDefault)

	key2 := cacheDefault.Key().File("test.txt").Build()
	hash2 := key2.Hash()

	// Hashes should differ (FNV vs xxHash)
	if hash1 == hash2 {
		t.Error("Custom hash function should produce different hash than default")
	}

	// Verify custom hash function is actually used
	if cache.hashFunc == nil {
		t.Error("hashFunc should be set")
	}
	h := cache.hashFunc()
	if _, ok := h.(hash.Hash64); !ok {
		t.Error("Custom hash function should return a hash.Hash implementation")
	}
}

// TestWithHashFunc_Persistence tests that custom hash affects cache storage
func TestWithHashFunc_Persistence(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	customHashFunc := func() hash.Hash {
		return fnv.New64a()
	}

	cache, err := Open(".cache", WithFs(fs), WithHashFunc(customHashFunc))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Retrieve with same cache
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Create new cache with different hash function - should not find entry
	cacheDefault, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cacheDefault *Cache) {
		err := cacheDefault.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cacheDefault)

	key2 := cacheDefault.Key().File("test.txt").Build()
	_, err = cacheDefault.Get(key2)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Expected cache miss with different hash function, got: %v", err)
	}
}

// TestWithNowFunc tests custom time function option
func TestWithNowFunc(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Fixed time for deterministic testing
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	customNowFunc := func() time.Time {
		return fixedTime
	}

	cache, err := Open(".cache", WithFs(fs), WithNowFunc(customNowFunc))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Store an entry
	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Retrieve and check timestamp
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !result.CreatedAt().Equal(fixedTime) {
		t.Errorf("CreatedAt = %v, want %v", result.CreatedAt(), fixedTime)
	}
	if !result.AccessedAt().Equal(fixedTime) {
		t.Errorf("AccessedAt = %v, want %v", result.AccessedAt(), fixedTime)
	}
}

// TestWithNowFunc_IncrementalTime tests with incrementing time
func TestWithNowFunc_IncrementalTime(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Incrementing time
	callCount := 0
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	customNowFunc := func() time.Time {
		result := baseTime.Add(time.Duration(callCount) * time.Hour)
		callCount++
		return result
	}

	cache, err := Open(".cache", WithFs(fs), WithNowFunc(customNowFunc))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Store first entry
	key1 := cache.Key().File("test.txt").String("v", "1").Build()
	err = cache.Put(key1).Bytes("output", []byte("data1")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	time1CallCount := callCount

	// Store second entry
	key2 := cache.Key().File("test.txt").String("v", "2").Build()
	err = cache.Put(key2).Bytes("output", []byte("data2")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	time2CallCount := callCount

	// Get both results
	result1, _ := cache.Get(key1)
	result2, _ := cache.Get(key2)

	// The times are based on when Commit was called, which calls nowFunc twice (CreatedAt and AccessedAt)
	// So we expect the difference to be based on the number of nowFunc calls between commits
	diff := result2.CreatedAt().Sub(result1.CreatedAt())

	// Since each commit calls nowFunc twice, the difference should be 2 hours
	if diff != 2*time.Hour {
		t.Errorf("Time difference = %v, want 2h (nowFunc called %d times between commits)", diff, time2CallCount-time1CallCount)
	}
}

// TestWithNowFunc_Stats tests custom time function affects statistics
func TestWithNowFunc_Stats(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Track times for each entry
	var entryTimes []time.Time
	callCount := 0
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	customNowFunc := func() time.Time {
		result := baseTime.Add(time.Duration(callCount) * 24 * time.Hour)
		callCount++
		return result
	}

	cache, err := Open(".cache", WithFs(fs), WithNowFunc(customNowFunc))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Create entries - each Commit calls nowFunc twice (CreatedAt and AccessedAt)
	for i := 0; i < 5; i++ {
		key := cache.Key().File("test.txt").String("day", string(rune('0'+i))).Build()
		err := cache.Put(key).Bytes("output", []byte("data")).Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
		// Track when this entry was created (first of the two nowFunc calls in Commit)
		entryTimes = append(entryTimes, baseTime.Add(time.Duration(callCount-2)*24*time.Hour))
	}

	// Get stats - this also calls nowFunc to calculate ages
	currentTimeAtStats := baseTime.Add(time.Duration(callCount) * 24 * time.Hour)
	stats, err := cache.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Entries != 5 {
		t.Errorf("Entries = %d, want 5", stats.Entries)
	}

	// Stats calculates age based on the current time when Stats() is called
	// The oldest entry was created at entryTimes[0], newest at entryTimes[4]
	// Ages are durations from "now" at the time Stats() was called
	expectedOldest := currentTimeAtStats.Sub(entryTimes[0])
	expectedNewest := currentTimeAtStats.Sub(entryTimes[4])

	// Allow some tolerance since exact timing depends on implementation details
	if stats.OldestEntry < expectedOldest-24*time.Hour || stats.OldestEntry > expectedOldest+24*time.Hour {
		t.Errorf("OldestEntry = %v, want approximately %v", stats.OldestEntry, expectedOldest)
	}

	if stats.NewestEntry < expectedNewest-24*time.Hour || stats.NewestEntry > expectedNewest+24*time.Hour {
		t.Errorf("NewestEntry = %v, want approximately %v", stats.NewestEntry, expectedNewest)
	}
}

// TestWithFs tests custom filesystem option
func TestWithFs(t *testing.T) {
	// Already heavily tested in other tests, but let's be explicit

	t.Run("MemMapFs", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer func(cache *Cache) {
			err := cache.Close()
			if err != nil {
				t.Fatalf("Close failed: %v", err)
			}
		}(cache)

		err = afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
		if err != nil {
			t.FailNow()
		}
		key := cache.Key().File("test.txt").Build()
		err = cache.Put(key).Bytes("output", []byte("data")).Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify cache used the in-memory fs
		exists, _ := afero.Exists(fs, ".cache/manifests")
		if !exists {
			t.Error("Cache should create directories in provided filesystem")
		}
	})

	t.Run("ReadOnlyFs error handling", func(t *testing.T) {
		baseFs := afero.NewMemMapFs()
		readOnlyFs := afero.NewReadOnlyFs(baseFs)

		// Should fail to create cache on read-only filesystem
		_, err := Open(".cache", WithFs(readOnlyFs))
		if err == nil {
			t.Error("Expected error when creating cache on read-only filesystem")
		}
	})
}

// TestWithAccumulateErrors tests error accumulation option
func TestWithAccumulateErrors(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("default fail-fast", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer func(cache *Cache) {
			err := cache.Close()
			if err != nil {
				t.Fatalf("Close failed: %v", err)
			}
		}(cache)

		// Add multiple invalid files
		key := cache.Key().
			File("missing1.txt").
			File("missing2.txt").
			File("missing3.txt").
			Build()

		_, err = key.computeHash()
		if err == nil {
			t.Fatal("Expected error")
		}

		// Fail-fast mode: validation stops after first error
		// But all inputs are still added
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		// Should have at least 1 error
		if len(ve.Errors) < 1 {
			t.Errorf("Expected at least 1 error, got %d", len(ve.Errors))
		}
	})

	t.Run("accumulate all errors", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs), WithAccumulateErrors())
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer func(cache *Cache) {
			err := cache.Close()
			if err != nil {
				t.Fatalf("Close failed: %v", err)
			}
		}(cache)

		// Add multiple invalid files
		key := cache.Key().
			File("missing1.txt").
			File("missing2.txt").
			File("missing3.txt").
			Build()

		_, err = key.computeHash()
		if err == nil {
			t.Fatal("Expected error")
		}

		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("Expected ValidationError, got %T", err)
		}

		// All 3 errors should be accumulated
		if len(ve.Errors) != 3 {
			t.Errorf("Expected 3 errors, got %d", len(ve.Errors))
		}
	})
}

// TestMultipleOptions tests combining multiple options
func TestMultipleOptions(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	customHashFunc := func() hash.Hash { return fnv.New64a() }
	customNowFunc := func() time.Time { return fixedTime }

	cache, err := Open(".cache",
		WithFs(fs),
		WithHashFunc(customHashFunc),
		WithNowFunc(customNowFunc),
		WithAccumulateErrors(),
	)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Verify all options applied
	if cache.fs != fs {
		t.Error("fs option not applied")
	}
	if cache.nowFunc == nil {
		t.Error("nowFunc option not applied")
	}
	if cache.hashFunc == nil {
		t.Error("hashFunc option not applied")
	}
	if !cache.accumulateErrors {
		t.Error("accumulateErrors option not applied")
	}

	// Test functionality with combined options
	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Verify time function worked
	if !result.CreatedAt().Equal(fixedTime) {
		t.Errorf("CreatedAt = %v, want %v", result.CreatedAt(), fixedTime)
	}
}

// TestOptionOrderIndependence tests that option order doesn't matter
func TestOptionOrderIndependence(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	customNowFunc := func() time.Time { return fixedTime }

	// Create two caches with options in different order
	cache1, err := Open(".cache1",
		WithFs(fs),
		WithNowFunc(customNowFunc),
		WithAccumulateErrors(),
	)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache1 *Cache) {
		err := cache1.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache1)

	cache2, err := Open(".cache2",
		WithAccumulateErrors(),
		WithNowFunc(customNowFunc),
		WithFs(fs),
	)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache2 *Cache) {
		err := cache2.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache2)

	// Both should behave identically
	key := cache1.Key().File("test.txt").Build()
	hash1 := key.Hash()

	key2 := cache2.Key().File("test.txt").Build()
	hash2 := key2.Hash()

	if hash1 != hash2 {
		t.Error("Option order should not affect hash computation")
	}
}

// TestWithHashFunc_NilPanic tests that nil hash function is handled
func TestWithHashFunc_NilFunction(t *testing.T) {
	fs := afero.NewMemMapFs()

	// This test verifies behavior when hashFunc is called
	cache, err := Open(".cache", WithFs(fs), WithHashFunc(nil))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Attempting to use cache with nil hash function should panic or error
	defer func() {
		if r := recover(); r == nil {
			// If no panic, should at least error
			t.Error("Expected panic or error with nil hash function")
		}
	}()

	err = afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}
	key := cache.Key().File("test.txt").Build()
	_, err = key.computeHash() // Should panic when calling nil hashFunc
	if err != nil {
		t.Fatalf("ComputeHash failed: %v", err)
	}
}

// TestDefaultOptions tests that defaults are reasonable
func TestDefaultOptions(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Verify defaults
	if cache.hashFunc == nil {
		t.Error("Default hashFunc should be set")
	}
	if cache.nowFunc == nil {
		t.Error("Default nowFunc should be set")
	}
	if cache.accumulateErrors {
		t.Error("Default should be fail-fast (accumulateErrors=false)")
	}

	// Verify hash function is xxHash (default)
	h := cache.hashFunc()
	if h == nil {
		t.Fatal("Default hash function returned nil")
	}

	// Verify now function returns reasonable time
	now := cache.nowFunc()
	if now.IsZero() {
		t.Error("Default nowFunc should return non-zero time")
	}
	if time.Since(now) > 1*time.Second {
		t.Error("Default nowFunc should return current time")
	}
}
