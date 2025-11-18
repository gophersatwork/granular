package granular

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestBasicCacheOperations(t *testing.T) {
	// Setup test cache and filesystem
	cache, memFs, tempDir := setupTestCache(t, "granular-test")

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	createTestFile(t, memFs, testFilePath, testContent)

	// Create a key with the test file as input
	key := cache.Key().
		File(testFilePath).
		Bytes(testContent).
		String("test", "value").
		Build()

	// First get should be a miss
	result, err := cache.Get(key)
	assertCacheMiss(t, result, err, "first Get")

	// Create an output file
	outputFilePath := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("output content")
	createTestFile(t, memFs, outputFilePath, outputContent)

	// Store in cache
	err = cache.Put(key).
		File("output", outputFilePath).
		Meta("data", "test data").
		Commit()
	assertNoError(t, err, "initial Put")

	// Second get should be a hit
	resultGet, err := cache.Get(key)
	assertCacheHit(t, resultGet, err, "second Get")
	assertResultHasFile(t, resultGet, "output", "result from second Get")

	// Get and verify the cached file
	cachedFilePath := resultGet.File("output")
	assertFileContent(t, memFs, cachedFilePath, outputContent)

	// Verify the cached metadata
	cachedData := resultGet.Meta("data")
	assertEqual(t, cachedData, "test data", "Cached metadata")

	// Modify the input file
	newContent := []byte("modified content")
	createTestFile(t, memFs, testFilePath, newContent)

	key2 := cache.Key().
		File(testFilePath).
		String("test", "value").
		Build()

	// Get should be a miss after modification
	result, err = cache.Get(key2)
	assertCacheMiss(t, result, err, "Get after modification")

	// Test cache clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Get should be a miss after clear
	result, err = cache.Get(key)
	assertCacheMiss(t, result, err, "Get after clear")
}

func TestGlobInput(t *testing.T) {
	// Setup test cache and filesystem
	cache, memFs, tempDir := setupTestCache(t, "granular-glob-test")

	// Create test files
	testDir := filepath.Join(tempDir, "testfiles")
	createTestDir(t, memFs, testDir)

	// Create multiple files
	for i, content := range []string{"file1", "file2", "file3"} {
		filePath := filepath.Join(testDir, fmt.Sprintf("file%d.txt", i+1))
		createTestFile(t, memFs, filePath, []byte(content))
	}

	// Create a key with glob pattern
	key := cache.Key().
		Glob(filepath.Join(testDir, "*.txt")).
		Build()

	// First get should be a miss
	result, err := cache.Get(key)
	assertCacheMiss(t, result, err, "first Get with glob pattern")

	// Store in cache
	err = cache.Put(key).
		Meta("count", "3").
		Commit()
	assertNoError(t, err, "glob pattern Put")

	// Second get should be a hit
	resultGet, err := cache.Get(key)
	assertCacheHit(t, resultGet, err, "second Get with glob pattern")

	// Verify the cached metadata
	assertMetadataValue(t, resultGet, "count", "3")

	// Add a new file
	newFilePath := filepath.Join(testDir, "file4.txt")
	createTestFile(t, memFs, newFilePath, []byte("file4"))

	// Get should be a miss after adding a file
	result, err = cache.Get(key)
	assertCacheMiss(t, result, err, "Get after adding file")
}

func TestBytesInput(t *testing.T) {
	// Setup test cache and filesystem
	cache, _, _ := setupTestCache(t, "granular-bytes-test")

	// Create a key with bytes input
	key := cache.Key().
		Bytes([]byte("test data")).
		Version("1.0").
		Build()

	// First get should be a miss
	result, err := cache.Get(key)
	assertCacheMiss(t, result, err, "first Get with bytes input")

	// Store in cache
	err = cache.Put(key).
		Meta("result", "computed from raw data").
		Commit()
	assertNoError(t, err, "bytes input Put")

	// Second get should be a hit
	resultGet, err := cache.Get(key)
	assertCacheHit(t, resultGet, err, "second Get with bytes input")

	// Verify the cached metadata
	assertMetadataValue(t, resultGet, "result", "computed from raw data")

	// Modify the bytes data
	key2 := cache.Key().
		Bytes([]byte("modified data")).
		Version("1.0").
		Build()

	// Get should be a miss after modification
	result, err = cache.Get(key2)
	assertCacheMiss(t, result, err, "Get after bytes modification")
}

func TestDirectoryInput(t *testing.T) {
	// Setup test cache and filesystem
	cache, memFs, tempDir := setupTestCache(t, "granular-dir-test")

	// Create test directory structure
	testDir := filepath.Join(tempDir, "testdir")
	subDir := filepath.Join(testDir, "subdir")
	createTestDir(t, memFs, subDir)

	// Create files in the directory
	files := map[string]string{
		filepath.Join(testDir, "file1.txt"):    "content1",
		filepath.Join(testDir, "file2.log"):    "log content",
		filepath.Join(subDir, "subfile.txt"):   "subcontent",
		filepath.Join(subDir, "another.log"):   "another log",
		filepath.Join(subDir, "important.txt"): "important",
	}

	for path, content := range files {
		createTestFile(t, memFs, path, []byte(content))
	}

	// Create a key with directory input, excluding log files
	key := cache.Key().
		Dir(testDir, "*.log").
		Build()

	// First get should be a miss
	result, err := cache.Get(key)
	assertCacheMiss(t, result, err, "first Get with directory input")

	// Store in cache
	err = cache.Put(key).
		Meta("fileCount", "3"). // 3 non-log files
		Commit()
	assertNoError(t, err, "directory input Put")

	// Second get should be a hit
	resultGet, err := cache.Get(key)
	assertCacheHit(t, resultGet, err, "second Get with directory input")

	// Modify a file that should be included
	createTestFile(t, memFs, filepath.Join(subDir, "important.txt"), []byte("modified"))

	// Get should be a miss after modification
	result, err = cache.Get(key)
	assertCacheMiss(t, result, err, "Get after modifying included file")

	// Modify a file that should be excluded
	createTestFile(t, memFs, filepath.Join(testDir, "file2.log"), []byte("new log content"))

	// Store in cache again with the modified excluded file
	err = cache.Put(key).
		Meta("fileCount", "3").
		Commit()
	assertNoError(t, err, "after log modification Put")

	// Get should be a hit since we only modified an excluded file
	resultGet, err = cache.Get(key)
	assertCacheHit(t, resultGet, err, "Get after modifying excluded file")
}

func TestMultiFileStorage(t *testing.T) {
	// Setup test cache and filesystem
	cache, memFs, tempDir := setupTestCache(t, "granular-multifile-test")

	// Create test files
	file1Path := filepath.Join(tempDir, "output1.txt")
	file2Path := filepath.Join(tempDir, "output2.json")
	createTestFile(t, memFs, file1Path, []byte("output 1"))
	createTestFile(t, memFs, file2Path, []byte(`{"key": "value"}`))

	// Create a key
	key := cache.Key().String("version", "1.0").Build()

	// Store multiple files
	err := cache.Put(key).
		File("text", file1Path).
		File("json", file2Path).
		Bytes("summary", []byte("test summary")).
		Meta("count", "2").
		Commit()
	assertNoError(t, err, "multi-file Put")

	// Retrieve and verify
	result, err := cache.Get(key)
	assertCacheHit(t, result, err, "multi-file Get")

	// Check both files exist
	if !result.HasFile("text") {
		t.Fatal("Expected 'text' file to exist")
	}
	if !result.HasFile("json") {
		t.Fatal("Expected 'json' file to exist")
	}

	// Check bytes data
	if !result.HasData("summary") {
		t.Fatal("Expected 'summary' data to exist")
	}
	summaryData := result.Bytes("summary")
	assertBytesEqual(t, summaryData, []byte("test summary"), "summary data")

	// Verify file count
	files := result.Files()
	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}
}

func TestHasAndDelete(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "granular-has-delete-test")

	// Create a test file and key
	testFile := filepath.Join(tempDir, "test.txt")
	createTestFile(t, memFs, testFile, []byte("content"))

	key := cache.Key().File(testFile).Build()

	// Has should return false initially
	if cache.Has(key) {
		t.Fatal("Expected Has to return false for non-existent key")
	}

	// Store something
	err := cache.Put(key).Meta("test", "value").Commit()
	assertNoError(t, err, "Put for Has test")

	// Has should return true now
	if !cache.Has(key) {
		t.Fatal("Expected Has to return true for existing key")
	}

	// Delete
	err = cache.Delete(key)
	assertNoError(t, err, "Delete")

	// Has should return false after delete
	if cache.Has(key) {
		t.Fatal("Expected Has to return false after Delete")
	}
}

// setupTestCache creates a new in-memory filesystem and cache for testing.
func setupTestCache(t *testing.T, tempDirName string) (*Cache, afero.Fs, string) {
	t.Helper()

	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/" + tempDirName
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := Open(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	return cache, memFs, tempDir
}

// createTestFile creates a file with the given path and content.
func createTestFile(t *testing.T, fs afero.Fs, path string, content []byte) {
	t.Helper()

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		createTestDir(t, fs, dir)
	}

	if err := afero.WriteFile(fs, path, content, 0o644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// createTestDir creates a directory.
func createTestDir(t *testing.T, fs afero.Fs, path string) {
	t.Helper()

	if err := fs.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

// assertCacheMiss asserts that the result is nil (cache miss).
func assertCacheMiss(t *testing.T, result *Result, err error, context string) {
	t.Helper()

	if err != ErrCacheMiss {
		t.Fatalf("Expected ErrCacheMiss on %s, got error: %v", context, err)
	}
	if result != nil {
		t.Fatalf("Expected nil result on cache miss for %s, got non-nil", context)
	}
}

// assertCacheHit asserts that the result is not nil (cache hit) and there's no error.
func assertCacheHit(t *testing.T, result *Result, err error, context string) {
	t.Helper()

	if err != nil {
		t.Fatalf("Expected no error on cache hit for %s, got error: %v", context, err)
	}
	if result == nil {
		t.Fatalf("Expected cache hit on %s, got nil result", context)
	}
}

// assertFileContent asserts that a file has the expected content.
func assertFileContent(t *testing.T, fs afero.Fs, path string, expected []byte) {
	t.Helper()

	actual, err := afero.ReadFile(fs, path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	assertBytesEqual(t, actual, expected, fmt.Sprintf("File content for %s", path))
}

// assertMetadataValue asserts that a metadata value matches expected.
func assertMetadataValue(t *testing.T, result *Result, key, expected string) {
	t.Helper()

	actual := result.Meta(key)
	if actual != expected {
		t.Fatalf("Metadata value mismatch for key %s:\nExpected: %s\nActual: %s",
			key, expected, actual)
	}
}

// assertBytesEqual asserts that two byte slices are equal.
func assertBytesEqual(t *testing.T, actual, expected []byte, context string) {
	t.Helper()

	if !bytes.Equal(actual, expected) {
		t.Fatalf("%s mismatch:\nExpected: %s\nActual: %s",
			context, string(expected), string(actual))
	}
}

// assertResultHasFile asserts that a result has the specified file.
func assertResultHasFile(t *testing.T, result *Result, name, context string) {
	t.Helper()

	if !result.HasFile(name) {
		t.Fatalf("Expected %s to have file '%s'", context, name)
	}
}

// assertNoError asserts that err is nil.
func assertNoError(t *testing.T, err error, context string) {
	t.Helper()

	if err != nil {
		t.Fatalf("Unexpected error on %s: %v", context, err)
	}
}

// assertEqual asserts that two strings are equal.
func assertEqual(t *testing.T, actual, expected, context string) {
	t.Helper()

	if actual != expected {
		t.Fatalf("%s mismatch:\nExpected: %s\nActual: %s",
			context, expected, actual)
	}
}

// TestCacheStats tests the Stats() method.
func TestCacheStats(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "granular-stats-test")

	// Initially, stats should show 0 entries
	stats, err := cache.Stats()
	assertNoError(t, err, "initial Stats")
	if stats.Entries != 0 {
		t.Fatalf("Expected 0 entries initially, got %d", stats.Entries)
	}
	if stats.TotalSize != 0 {
		t.Fatalf("Expected 0 total size initially, got %d", stats.TotalSize)
	}

	// Create and cache a file
	testFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, testFile, []byte("test data"))

	key1 := cache.Key().File(testFile).String("version", "1").Build()
	outputFile := filepath.Join(tempDir, "output1.txt")
	createTestFile(t, memFs, outputFile, []byte("output data 1"))

	err = cache.Put(key1).
		File("out", outputFile).
		Meta("key", "value").
		Commit()
	assertNoError(t, err, "Put 1")

	// Stats should now show 1 entry
	stats, err = cache.Stats()
	assertNoError(t, err, "Stats after Put 1")
	if stats.Entries != 1 {
		t.Fatalf("Expected 1 entry, got %d", stats.Entries)
	}
	if stats.TotalSize == 0 {
		t.Fatal("Expected non-zero total size")
	}

	// Add another entry
	key2 := cache.Key().File(testFile).String("version", "2").Build()
	outputFile2 := filepath.Join(tempDir, "output2.txt")
	createTestFile(t, memFs, outputFile2, []byte("output data 2"))

	err = cache.Put(key2).
		File("out", outputFile2).
		Commit()
	assertNoError(t, err, "Put 2")

	// Stats should now show 2 entries
	stats, err = cache.Stats()
	assertNoError(t, err, "Stats after Put 2")
	if stats.Entries != 2 {
		t.Fatalf("Expected 2 entries, got %d", stats.Entries)
	}
}

// TestCachePrune tests the Prune() method.
func TestCachePrune(t *testing.T) {
	// Create cache with custom time function
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	timeFunc := func() time.Time { return now }

	cache, memFs, tempDir := setupTestCache(t, "granular-prune-test")
	cache.nowFunc = timeFunc

	// Create test file
	testFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, testFile, []byte("test"))

	// Create old entry
	key1 := cache.Key().File(testFile).String("v", "1").Build()
	outputFile1 := filepath.Join(tempDir, "out1.txt")
	createTestFile(t, memFs, outputFile1, []byte("old"))

	err := cache.Put(key1).File("out", outputFile1).Commit()
	assertNoError(t, err, "Put old entry")

	// Advance time by 8 days
	now = now.Add(8 * 24 * time.Hour)
	cache.nowFunc = func() time.Time { return now }

	// Create recent entry
	key2 := cache.Key().File(testFile).String("v", "2").Build()
	outputFile2 := filepath.Join(tempDir, "out2.txt")
	createTestFile(t, memFs, outputFile2, []byte("recent"))

	err = cache.Put(key2).File("out", outputFile2).Commit()
	assertNoError(t, err, "Put recent entry")

	// Prune entries older than 7 days
	removed, err := cache.Prune(7 * 24 * time.Hour)
	assertNoError(t, err, "Prune")

	// Should have removed 1 entry (the old one)
	if removed != 1 {
		t.Fatalf("Expected to prune 1 entry, got %d", removed)
	}

	// Old entry should be gone
	result1, err := cache.Get(key1)
	assertCacheMiss(t, result1, err, "Get old entry after prune")

	// Recent entry should still exist
	result2, err := cache.Get(key2)
	assertCacheHit(t, result2, err, "Get recent entry after prune")
}

// TestCachePruneUnused tests the PruneUnused() method.
func TestCachePruneUnused(t *testing.T) {
	// Create cache with custom time function
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	timeFunc := func() time.Time { return now }

	cache, memFs, tempDir := setupTestCache(t, "granular-prune-unused-test")
	cache.nowFunc = timeFunc

	// Create test file
	testFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, testFile, []byte("test"))

	// Create old entry
	key1 := cache.Key().File(testFile).String("v", "1").Build()
	outputFile1 := filepath.Join(tempDir, "out1.txt")
	createTestFile(t, memFs, outputFile1, []byte("data1"))

	err := cache.Put(key1).File("out", outputFile1).Commit()
	assertNoError(t, err, "Put old entry")

	// Advance time by 8 days
	now = now.Add(8 * 24 * time.Hour)
	cache.nowFunc = func() time.Time { return now }

	// Create recent entry (AccessedAt will be 8 days after entry 1)
	key2 := cache.Key().File(testFile).String("v", "2").Build()
	outputFile2 := filepath.Join(tempDir, "out2.txt")
	createTestFile(t, memFs, outputFile2, []byte("data2"))

	err = cache.Put(key2).File("out", outputFile2).Commit()
	assertNoError(t, err, "Put recent entry")

	// Prune entries not accessed in last 7 days
	removed, err := cache.PruneUnused(7 * 24 * time.Hour)
	assertNoError(t, err, "PruneUnused")

	// Should have removed 1 entry (entry 1, created 8 days ago)
	if removed != 1 {
		t.Fatalf("Expected to prune 1 unused entry, got %d", removed)
	}

	// Entry 1 should be gone (created more than 7 days ago)
	result1, err := cache.Get(key1)
	assertCacheMiss(t, result1, err, "Get old entry after prune")

	// Entry 2 should still exist (created recently)
	result2, err := cache.Get(key2)
	assertCacheHit(t, result2, err, "Get recent entry after prune")
}

// TestCacheEntries tests the Entries() method.
func TestCacheEntries(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "granular-entries-test")

	// Initially, no entries
	entries, err := cache.Entries()
	assertNoError(t, err, "initial Entries")
	if len(entries) != 0 {
		t.Fatalf("Expected 0 entries initially, got %d", len(entries))
	}

	// Create test file
	testFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, testFile, []byte("test"))

	// Add 3 entries
	for i := 1; i <= 3; i++ {
		key := cache.Key().File(testFile).String("v", string(rune('0'+i))).Build()
		outputFile := filepath.Join(tempDir, "out"+string(rune('0'+i))+".txt")
		createTestFile(t, memFs, outputFile, []byte("data"))

		err := cache.Put(key).File("out", outputFile).Commit()
		assertNoError(t, err, "Put entry "+string(rune('0'+i)))
	}

	// List entries
	entries, err = cache.Entries()
	assertNoError(t, err, "Entries after adding")
	if len(entries) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(entries))
	}

	// Verify each entry has required fields
	for _, entry := range entries {
		if entry.KeyHash == "" {
			t.Fatal("Entry missing KeyHash")
		}
		if entry.CreatedAt.IsZero() {
			t.Fatal("Entry missing CreatedAt")
		}
		if entry.AccessedAt.IsZero() {
			t.Fatal("Entry missing AccessedAt")
		}
	}
}

// TestResultCopyFile tests the Result.CopyFile() method.
func TestResultCopyFile(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "granular-copyfile-test")

	// Create and cache a file
	inputFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, inputFile, []byte("input"))

	key := cache.Key().File(inputFile).Build()

	outputFile := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("cached output content")
	createTestFile(t, memFs, outputFile, outputContent)

	err := cache.Put(key).File("myfile", outputFile).Commit()
	assertNoError(t, err, "Put")

	// Get the cached result
	result, err := cache.Get(key)
	assertCacheHit(t, result, err, "Get")

	// Copy the cached file to a new location
	destPath := filepath.Join(tempDir, "restored.txt")
	err = result.CopyFile("myfile", destPath)
	assertNoError(t, err, "CopyFile")

	// Verify the copied file has correct content
	assertFileContent(t, memFs, destPath, outputContent)

	// Test error case: file doesn't exist
	err = result.CopyFile("nonexistent", destPath)
	if err == nil {
		t.Fatal("Expected error when copying nonexistent file")
	}
}

// TestResultTiming tests Result timing methods.
func TestResultTiming(t *testing.T) {
	// Create cache with custom time function
	createdTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	timeFunc := func() time.Time { return createdTime }

	cache, memFs, tempDir := setupTestCache(t, "granular-timing-test")
	cache.nowFunc = timeFunc

	// Create and cache a file
	inputFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, inputFile, []byte("test"))

	key := cache.Key().File(inputFile).Build()

	outputFile := filepath.Join(tempDir, "output.txt")
	createTestFile(t, memFs, outputFile, []byte("output"))

	err := cache.Put(key).File("out", outputFile).Commit()
	assertNoError(t, err, "Put")

	// Advance time by 5 minutes (to test Age() calculation)
	now := createdTime.Add(5 * time.Minute)
	cache.nowFunc = func() time.Time { return now }

	// Get the result
	result, err := cache.Get(key)
	assertCacheHit(t, result, err, "Get")

	// Verify CreatedAt
	if !result.CreatedAt().Equal(createdTime) {
		t.Fatalf("CreatedAt mismatch:\nExpected: %v\nActual: %v",
			createdTime, result.CreatedAt())
	}

	// Verify AccessedAt (equals CreatedAt since Get() doesn't update it)
	if !result.AccessedAt().Equal(createdTime) {
		t.Fatalf("AccessedAt mismatch:\nExpected: %v\nActual: %v",
			createdTime, result.AccessedAt())
	}

	// Verify Age() (time since creation)
	expectedAge := 5 * time.Minute
	actualAge := result.Age()
	if actualAge != expectedAge {
		t.Fatalf("Age mismatch:\nExpected: %v\nActual: %v",
			expectedAge, actualAge)
	}

	// Verify Size() returns non-zero
	if result.Size() == 0 {
		t.Fatal("Expected non-zero Size()")
	}
}

// TestKeyBuilderEnv tests the KeyBuilder.Env() method.
func TestKeyBuilderEnv(t *testing.T) {
	cache, _, _ := setupTestCache(t, "granular-env-test")

	// Set an environment variable
	os.Setenv("TEST_ENV_VAR", "value1")
	defer os.Unsetenv("TEST_ENV_VAR")

	// Build key including env var
	key1 := cache.Key().
		String("test", "data").
		Env("TEST_ENV_VAR").
		Build()

	// Build same key with same env value
	key2 := cache.Key().
		String("test", "data").
		Env("TEST_ENV_VAR").
		Build()

	// Keys should match
	hash1, _ := key1.computeHash()
	hash2, _ := key2.computeHash()
	if hash1 != hash2 {
		t.Fatal("Expected matching keys with same env value")
	}

	// Change env var
	os.Setenv("TEST_ENV_VAR", "value2")

	// Build key with different env value
	key3 := cache.Key().
		String("test", "data").
		Env("TEST_ENV_VAR").
		Build()

	// Key should be different
	hash3, _ := key3.computeHash()
	if hash1 == hash3 {
		t.Fatal("Expected different keys with different env value")
	}
}

// TestKeyBuilderHash tests the Hash() methods.
func TestKeyBuilderHash(t *testing.T) {
	cache, _, _ := setupTestCache(t, "granular-hash-test")

	// Test KeyBuilder.Hash()
	builder := cache.Key().
		String("test", "value").
		String("foo", "bar")

	hash1 := builder.Hash()
	if hash1 == "" {
		t.Fatal("Expected non-empty hash from KeyBuilder.Hash()")
	}

	// Build the key
	key := builder.Build()

	// Test Key.Hash()
	hash2 := key.Hash()
	if hash2 == "" {
		t.Fatal("Expected non-empty hash from Key.Hash()")
	}

	// Hashes should match
	if hash1 != hash2 {
		t.Fatalf("Hash mismatch:\nKeyBuilder.Hash(): %s\nKey.Hash(): %s",
			hash1, hash2)
	}

	// Different builder should produce different hash
	builder2 := cache.Key().
		String("test", "different").
		String("foo", "bar")

	hash3 := builder2.Hash()
	if hash1 == hash3 {
		t.Fatal("Expected different hashes for different builders")
	}
}
