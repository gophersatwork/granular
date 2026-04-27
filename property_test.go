package granular

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/spf13/afero"
)

// randFill fills b with random bytes from r without using the deprecated (*rand.Rand).Read.
func randFill(r *rand.Rand, b []byte) {
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
}

// TestProperty_HashDeterminism tests that same inputs always produce same hash
func TestProperty_HashDeterminism(t *testing.T) {
	property := func(seed int64) bool {
		// Use seed for reproducibility
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create random test files
		numFiles := r.Intn(5) + 1
		for i := range numFiles {
			filename := fmt.Sprintf("file%d.txt", i)
			content := make([]byte, r.Intn(100)+1)
			randFill(r, content)
			afero.WriteFile(fs, filename, content, 0o644)
		}

		// Build key with random inputs
		kb := cache.Key()
		for i := range numFiles {
			kb = kb.File(fmt.Sprintf("file%d.txt", i))
		}

		// Add random string pairs
		numStrings := r.Intn(3)
		for i := range numStrings {
			kb = kb.String(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", r.Intn(100)))
		}

		key := kb.Build()

		// Compute hash multiple times
		hash1, err1 := key.computeHash()
		hash2, err2 := key.computeHash()
		hash3, err3 := key.computeHash()

		// All should succeed and be identical
		if err1 != nil || err2 != nil || err3 != nil {
			return false
		}
		if hash1 != hash2 || hash2 != hash3 {
			t.Logf("Hashes differ: %s, %s, %s", hash1, hash2, hash3)
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 100}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_HashIndependentOfInputOrder tests that hash is order-independent for sorted inputs
func TestProperty_HashIndependentOfInputOrder(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create test files
		afero.WriteFile(fs, "a.txt", []byte("content a"), 0o644)
		afero.WriteFile(fs, "b.txt", []byte("content b"), 0o644)

		// String() calls should be order-independent (internally sorted)
		numPairs := r.Intn(5) + 1
		pairs := make(map[string]string)
		for i := range numPairs {
			pairs[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", r.Intn(100))
		}

		// Build key with pairs in one order
		kb1 := cache.Key().File("a.txt")
		for k, v := range pairs {
			kb1 = kb1.String(k, v)
		}
		hash1 := kb1.Hash()

		// Build key with pairs in potentially different order (map iteration)
		kb2 := cache.Key().File("a.txt")
		for k, v := range pairs {
			kb2 = kb2.String(k, v)
		}
		hash2 := kb2.Hash()

		// Hashes should be identical (extras are sorted internally)
		if hash1 != hash2 {
			t.Logf("Hashes differ with same string pairs: %s vs %s", hash1, hash2)
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 100}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_CacheIdempotency tests that multiple Put operations are idempotent
func TestProperty_CacheIdempotency(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create test file
		content := make([]byte, r.Intn(100)+10)
		randFill(r, content)
		afero.WriteFile(fs, "input.txt", content, 0o644)

		// Create output data
		outputData := make([]byte, r.Intn(50)+10)
		randFill(r, outputData)

		// Build key
		key := cache.Key().File("input.txt").String("v", "1").Build()

		// Put same entry multiple times
		for range 3 {
			err := cache.Put(key).Bytes("output", outputData).Commit()
			if err != nil {
				return false
			}
		}

		// Get result - should be consistent
		result, err := cache.Get(key)
		if err != nil {
			return false
		}

		data := result.Bytes("output")
		if data == nil {
			return false
		}

		// Data should match what we put
		if !reflect.DeepEqual(data, outputData) {
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 50}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_ManifestRoundTrip tests that manifest save/load preserves data
func TestProperty_ManifestRoundTrip(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create test file
		afero.WriteFile(fs, "input.txt", []byte("test"), 0o644)

		// Create random output data
		numDataEntries := r.Intn(5) + 1
		outputData := make(map[string][]byte)
		for i := range numDataEntries {
			data := make([]byte, r.Intn(50)+1)
			randFill(r, data)
			outputData[fmt.Sprintf("data%d", i)] = data
		}

		// Create random metadata
		numMeta := r.Intn(5) + 1
		metadata := make(map[string]string)
		for i := range numMeta {
			metadata[fmt.Sprintf("meta%d", i)] = fmt.Sprintf("value%d", r.Intn(100))
		}

		// Build and commit
		key := cache.Key().File("input.txt").Build()
		wb := cache.Put(key)
		for name, data := range outputData {
			wb = wb.Bytes(name, data)
		}
		for k, v := range metadata {
			wb = wb.Meta(k, v)
		}

		err = wb.Commit()
		if err != nil {
			return false
		}

		// Retrieve and verify all data preserved
		result, err := cache.Get(key)
		if err != nil {
			return false
		}

		// Verify data
		for name, expectedData := range outputData {
			actualData := result.Bytes(name)
			if actualData == nil {
				t.Logf("Missing data entry: %s", name)
				return false
			}
			if !reflect.DeepEqual(actualData, expectedData) {
				t.Logf("Data mismatch for %s", name)
				return false
			}
		}

		// Verify metadata
		for key, expectedValue := range metadata {
			actualValue := result.Meta(key)
			if actualValue != expectedValue {
				t.Logf("Metadata mismatch for %s: %s vs %s", key, actualValue, expectedValue)
				return false
			}
		}

		return true
	}

	config := &quick.Config{MaxCount: 50}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_BytesCopyImmutability tests that Bytes() copies data (no mutation)
func TestProperty_BytesCopyImmutability(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		afero.WriteFile(fs, "input.txt", []byte("test"), 0o644)

		// Create data
		data := make([]byte, r.Intn(100)+10)
		randFill(r, data)
		originalData := make([]byte, len(data))
		copy(originalData, data)

		// Store in cache
		key := cache.Key().File("input.txt").Build()
		err = cache.Put(key).Bytes("output", data).Commit()
		if err != nil {
			return false
		}

		// Mutate original data
		for i := range data {
			data[i] = 0
		}

		// Retrieve from cache
		result, err := cache.Get(key)
		if err != nil {
			return false
		}

		retrievedData := result.Bytes("output")
		if retrievedData == nil {
			return false
		}

		// Retrieved data should match original, not mutated version
		if !reflect.DeepEqual(retrievedData, originalData) {
			t.Log("Data was not properly copied")
			return false
		}

		// Verify all bytes are NOT zero (would indicate mutation)
		allZero := true
		for _, b := range retrievedData {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero && len(retrievedData) > 0 {
			// This would mean data was mutated
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 50}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_DeleteIsEffective tests that Delete actually removes entries
func TestProperty_DeleteIsEffective(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create test file
		content := make([]byte, r.Intn(100)+10)
		randFill(r, content)
		afero.WriteFile(fs, "test.txt", content, 0o644)

		// Store entry
		key := cache.Key().File("test.txt").Build()
		err = cache.Put(key).Bytes("data", []byte("test")).Commit()
		if err != nil {
			return false
		}

		// Verify exists
		if !cache.Has(key) {
			return false
		}

		// Delete
		err = cache.Delete(key)
		if err != nil {
			return false
		}

		// Verify doesn't exist
		if cache.Has(key) {
			t.Log("Entry still exists after delete")
			return false
		}

		// Get should return cache miss
		_, err = cache.Get(key)
		if !errors.Is(err, ErrCacheMiss) {
			t.Logf("Expected cache miss after delete, got: %v", err)
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 50}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_GlobMatchesAreSubsetOfWalk tests that glob results are valid
func TestProperty_GlobMatchesAreSubsetOfWalk(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()

		// Create random directory structure
		numDirs := r.Intn(3) + 1
		numFiles := r.Intn(10) + 1

		for i := range numDirs {
			dir := fmt.Sprintf("dir%d", i)
			fs.MkdirAll(dir, 0o755)

			// Create some files in each dir
			for j := range numFiles {
				ext := []string{".go", ".txt", ".md"}[r.Intn(3)]
				path := fmt.Sprintf("%s/file%d%s", dir, j, ext)
				afero.WriteFile(fs, path, []byte("content"), 0o644)
			}
		}

		// Test various patterns
		patterns := []string{
			"**/*.go",
			"**/*.txt",
			"dir0/*.go",
			"**/*",
		}

		for _, pattern := range patterns {
			matches, err := expandGlob(pattern, fs)
			if err != nil {
				continue // Skip invalid patterns
			}

			// All matches should be real files
			for _, match := range matches {
				exists, err := afero.Exists(fs, match)
				if err != nil || !exists {
					t.Logf("Glob matched non-existent file: %s", match)
					return false
				}

				// Should be a file, not directory
				info, err := fs.Stat(match)
				if err != nil || info.IsDir() {
					t.Logf("Glob matched directory: %s", match)
					return false
				}
			}
		}

		return true
	}

	config := &quick.Config{MaxCount: 30}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_ClearRemovesAllEntries tests that Clear removes everything
func TestProperty_ClearRemovesAllEntries(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create test file
		afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)

		// Create random number of entries
		numEntries := r.Intn(10) + 1
		keys := make([]Key, numEntries)

		for i := range numEntries {
			keys[i] = cache.Key().
				File("test.txt").
				String("id", fmt.Sprintf("%d", i)).
				Build()

			err := cache.Put(keys[i]).Bytes("data", []byte("test")).Commit()
			if err != nil {
				return false
			}
		}

		// Verify all exist
		for _, key := range keys {
			if !cache.Has(key) {
				return false
			}
		}

		// Clear cache
		err = cache.Clear()
		if err != nil {
			return false
		}

		// Verify none exist
		for _, key := range keys {
			if cache.Has(key) {
				t.Log("Entry still exists after Clear")
				return false
			}
		}

		// Stats should show zero entries
		stats, err := cache.Stats()
		if err != nil {
			return false
		}

		if stats.Entries != 0 {
			t.Logf("Stats shows %d entries after Clear, want 0", stats.Entries)
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 30}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}

// TestProperty_DifferentInputsDifferentHashes tests hash uniqueness
func TestProperty_DifferentInputsDifferentHashes(t *testing.T) {
	property := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed))

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			return false
		}
		defer cache.Close()

		// Create two different files
		content1 := make([]byte, r.Intn(100)+10)
		content2 := make([]byte, r.Intn(100)+10)
		randFill(r, content1)
		randFill(r, content2)

		// Ensure they're actually different
		if reflect.DeepEqual(content1, content2) {
			content2[0] ^= 0xFF // Flip bits to make different
		}

		afero.WriteFile(fs, "file1.txt", content1, 0o644)
		afero.WriteFile(fs, "file2.txt", content2, 0o644)

		// Build keys for different files
		key1 := cache.Key().File("file1.txt").Build()
		key2 := cache.Key().File("file2.txt").Build()

		hash1, err1 := key1.computeHash()
		hash2, err2 := key2.computeHash()

		if err1 != nil || err2 != nil {
			return false
		}

		// Different inputs should (almost certainly) produce different hashes
		if hash1 == hash2 {
			t.Log("Different inputs produced same hash (collision)")
			return false
		}

		return true
	}

	config := &quick.Config{MaxCount: 100}
	if err := quick.Check(property, config); err != nil {
		t.Error(err)
	}
}
