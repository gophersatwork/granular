package granular

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

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
	key := Key{
		Inputs: []Input{
			FileInput{Path: testFilePath, Fs: memFs},
			RawInput{
				Data: testContent,
				Name: "test.txt",
			},
		},
		Extra: map[string]string{"test": "value"},
	}

	// First get should be a miss
	result, hit, err := cache.Get(key)
	assertCacheMiss(t, result, hit, err, "first Get")

	// Create an output file
	outputFilePath := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("output content")
	createTestFile(t, memFs, outputFilePath, outputContent)

	// Store in cache
	resultToStore := Result{
		Path: outputFilePath,
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	assertStoreSucceeds(t, cache, key, resultToStore, "initial result")

	// Second get should be a hit
	resultGet, hit, err := cache.Get(key)
	assertCacheHit(t, resultGet, hit, err, "second Get")
	assertResultHasPath(t, resultGet, "result from second Get")

	// Get and verify the cached file
	cachedFilePath := assertFileExists(t, cache, key, filepath.Base(outputFilePath))
	assertFileContent(t, memFs, cachedFilePath, outputContent)

	// Get and verify the cached data
	cachedData := assertDataExists(t, cache, key, "data")
	expectedData := []byte("test data")
	assertBytesEqual(t, cachedData, expectedData, "Cached data")

	// Modify the input file
	newContent := []byte("modified content")
	createTestFile(t, memFs, testFilePath, newContent)

	key2 := Key{
		Inputs: []Input{
			FileInput{Path: testFilePath, Fs: memFs},
		},
		Extra: map[string]string{"test": "value"},
	}

	// Get should be a miss after modification
	result, hit, err = cache.Get(key2)
	assertCacheMiss(t, result, hit, err, "Get after modification")

	// Test cache clear
	if err := cache.Clear(); err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Get should be a miss after clear
	result, hit, err = cache.Get(key)
	assertCacheMiss(t, result, hit, err, "Get after clear")
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
	key := Key{
		Inputs: []Input{
			GlobInput{Pattern: filepath.Join(testDir, "*.txt"), Fs: memFs},
		},
	}

	// First get should be a miss
	result, hit, err := cache.Get(key)
	assertCacheMiss(t, result, hit, err, "first Get with glob pattern")

	// Store in cache
	resultToStore := Result{
		Path: "",
		Metadata: map[string]string{
			"count": "3",
		},
	}
	assertStoreSucceeds(t, cache, key, resultToStore, "glob pattern result")

	// Second get should be a hit
	resultGet, hit, err := cache.Get(key)
	assertCacheHit(t, resultGet, hit, err, "second Get with glob pattern")

	// Verify the cached data
	assertMetadataValue(t, resultGet, "count", "3")

	// Add a new file
	newFilePath := filepath.Join(testDir, "file4.txt")
	createTestFile(t, memFs, newFilePath, []byte("file4"))

	// Get should be a miss after adding a file
	result, hit, err = cache.Get(key)
	assertCacheMiss(t, result, hit, err, "Get after adding file")
}

func TestRawInput(t *testing.T) {
	// Setup test cache and filesystem
	cache, _, _ := setupTestCache(t, "granular-raw-test")

	// Create a key with raw input
	key := Key{
		Inputs: []Input{
			RawInput{
				Data: []byte("test data"),
				Name: "test-input",
			},
		},
		Extra: map[string]string{"version": "1.0"},
	}

	// First get should be a miss
	result, hit, err := cache.Get(key)
	assertCacheMiss(t, result, hit, err, "first Get with raw input")

	// Store in cache
	resultToStore := Result{
		Path: "",
		Metadata: map[string]string{
			"result": "computed from raw data",
		},
	}
	assertStoreSucceeds(t, cache, key, resultToStore, "raw input result")

	// Second get should be a hit
	resultGet, hit, err := cache.Get(key)
	assertCacheHit(t, resultGet, hit, err, "second Get with raw input")

	// Verify the cached data
	assertMetadataValue(t, resultGet, "result", "computed from raw data")

	// Modify the raw data
	key.Inputs = []Input{
		RawInput{
			Data: []byte("modified data"),
			Name: "test-input",
		},
	}

	// Get should be a miss after modification
	result, hit, err = cache.Get(key)
	assertCacheMiss(t, result, hit, err, "Get after raw input modification")
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
	key := Key{
		Inputs: []Input{
			DirectoryInput{
				Path:    testDir,
				Exclude: []string{"*.log"},
				Fs:      memFs,
			},
		},
	}

	// First get should be a miss
	result, hit, err := cache.Get(key)
	assertCacheMiss(t, result, hit, err, "first Get with directory input")

	// Store in cache
	resultToStore := Result{
		Path: "",
		Metadata: map[string]string{
			"fileCount": "3", // 3 non-log files
		},
	}
	assertStoreSucceeds(t, cache, key, resultToStore, "directory input result")

	// Second get should be a hit
	resultGet, hit, err := cache.Get(key)
	assertCacheHit(t, resultGet, hit, err, "second Get with directory input")

	// Modify a file that should be included
	createTestFile(t, memFs, filepath.Join(subDir, "important.txt"), []byte("modified"))

	// Get should be a miss after modification
	result, hit, err = cache.Get(key)
	assertCacheMiss(t, result, hit, err, "Get after modifying included file")

	// Modify a file that should be excluded
	createTestFile(t, memFs, filepath.Join(testDir, "file2.log"), []byte("new log content"))

	// Store in cache again
	resultToStore = Result{
		Path: "",
		Metadata: map[string]string{
			"fileCount": "3", // 3 non-log files
		},
	}
	assertStoreSucceeds(t, cache, key, resultToStore, "after log modification")

	// Get should be a hit since we only modified an excluded file
	resultGet, hit, err = cache.Get(key)
	assertCacheHit(t, resultGet, hit, err, "Get after modifying excluded file")
}

// setupTestCache creates a new in-memory filesystem and cache for testing.
// It returns the cache, filesystem, and temporary directory path.
func setupTestCache(t *testing.T, tempDirName string) (*Cache, afero.Fs, string) {
	t.Helper()

	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/" + tempDirName
	if err := memFs.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	return cache, memFs, tempDir
}

// createTestFile creates a file with the given path and content in the filesystem.
func createTestFile(t *testing.T, fs afero.Fs, path string, content []byte) {
	t.Helper()

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		createTestDir(t, fs, dir)
	}

	if err := afero.WriteFile(fs, path, content, 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// createTestDir creates a directory with the given path in the filesystem.
func createTestDir(t *testing.T, fs afero.Fs, path string) {
	t.Helper()

	if err := fs.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

// assertCacheMiss asserts that a cache operation results in a miss.
func assertCacheMiss(t *testing.T, result *Result, hit bool, err error, context string) {
	t.Helper()

	if err != nil && !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Unexpected error on %s: %v", context, err)
	}
	if hit {
		t.Fatalf("Expected cache miss on %s, got hit", context)
	}
}

// assertCacheHit asserts that a cache operation results in a hit.
func assertCacheHit(t *testing.T, result *Result, hit bool, err error, context string) {
	t.Helper()

	if err != nil {
		t.Fatalf("Unexpected error on %s: %v", context, err)
	}
	if !hit {
		t.Fatalf("Expected cache hit on %s, got miss", context)
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

// assertMetadataValue asserts that a metadata value matches the expected value.
func assertMetadataValue(t *testing.T, result *Result, key, expected string) {
	t.Helper()

	if result.Metadata == nil {
		t.Fatalf("Expected metadata to contain key %s, but metadata is nil", key)
	}

	actual, exists := result.Metadata[key]
	if !exists {
		t.Fatalf("Expected metadata to contain key %s, but key not found", key)
	}

	if actual != expected {
		t.Fatalf("Metadata value mismatch for key %s:\nExpected: %s\nActual: %s",
			key, expected, actual)
	}
}

// assertFileExists asserts that a file exists and returns its path.
func assertFileExists(t *testing.T, cache *Cache, key Key, filename string) string {
	t.Helper()

	path, found, err := cache.GetFile(key, filename)
	if err != nil {
		t.Fatalf("Failed to GetFile %s: %v", filename, err)
	}
	if !found {
		t.Fatalf("Expected to find file %s, but not found", filename)
	}
	return path
}

// assertDataExists asserts that data exists and returns its content.
func assertDataExists(t *testing.T, cache *Cache, key Key, dataKey string) []byte {
	t.Helper()

	data, found, err := cache.GetData(key, dataKey)
	if err != nil {
		t.Fatalf("Failed to GetData %s: %v", dataKey, err)
	}
	if !found {
		t.Fatalf("Expected to find data %s, but not found", dataKey)
	}
	return data
}

// assertBytesEqual asserts that two byte slices are equal.
func assertBytesEqual(t *testing.T, actual, expected []byte, context string) {
	t.Helper()

	if !bytes.Equal(actual, expected) {
		t.Fatalf("%s mismatch:\nExpected: %s\nActual: %s",
			context, string(expected), string(actual))
	}
}

// assertStoreSucceeds asserts that a store operation succeeds.
func assertStoreSucceeds(t *testing.T, cache *Cache, key Key, result Result, context string) {
	t.Helper()

	err := cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store %s: %v", context, err)
	}
}

// assertResultHasPath asserts that a result has a non-empty path.
func assertResultHasPath(t *testing.T, result *Result, context string) {
	t.Helper()

	if result.Path == "" {
		t.Fatalf("Expected %s to have a path, got empty path", context)
	}
}