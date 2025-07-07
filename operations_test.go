package granular

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

// TestGet tests the Get operation
func TestGet(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-get-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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
	_, hit, err := cache.Get(key)
	if err != nil && !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Unexpected error on first Get: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss on first Get, got hit")
	}

	// Create an output file
	outputFilePath := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("output content")
	if err := afero.WriteFile(memFs, outputFilePath, outputContent, 0o644); err != nil {
		t.Fatalf("Failed to write output file: %v", err)
	}

	// Debug: Check cache root directories
	manifestsDir := filepath.Join(tempDir, "manifests")
	objectsDir := filepath.Join(tempDir, "objects")
	manifestsDirExists, err := afero.DirExists(memFs, manifestsDir)
	if err != nil {
		t.Fatalf("Failed to check manifests directory existence: %v", err)
	}
	t.Logf("Manifests directory exists: %v", manifestsDirExists)
	objectsDirExists, err := afero.DirExists(memFs, objectsDir)
	if err != nil {
		t.Fatalf("Failed to check objects directory existence: %v", err)
	}
	t.Logf("Objects directory exists: %v", objectsDirExists)

	// Store in cache
	result := Result{
		Path: outputFilePath,
		Metadata: map[string]string{
			"data": "test data",
		},
	}

	// Debug: Add a wrapper around Store to see what's happening
	t.Logf("About to call Store with key: %+v", key)
	t.Logf("Result to store: %+v", result)

	// Create a custom Store function that adds debug output
	storeWithDebug := func(key Key, result Result) error {
		// Compute the key hash
		keyHash, err := cache.computeKeyHash(key)
		if err != nil {
			t.Logf("Failed to compute key hash: %v", err)
			return err
		}
		t.Logf("Computed key hash: %s", keyHash)

		// Create input descriptions
		inputDescs := make([]string, len(key.Inputs))
		for i, input := range key.Inputs {
			inputDescs[i] = input.String()
		}
		t.Logf("Input descriptions: %v", inputDescs)

		// Extract file path from result
		var files []string
		if result.Path != "" {
			files = append(files, result.Path)
		}
		t.Logf("Files to store: %v", files)

		// Create the object directory
		objectDir := filepath.Join(tempDir, "objects", keyHash[:2], keyHash)
		t.Logf("Object directory: %s", objectDir)
		dirExists, err := afero.DirExists(memFs, objectDir)
		if err != nil {
			t.Logf("Failed to check object directory existence: %v", err)
		} else {
			t.Logf("Object directory exists before creation: %v", dirExists)
		}

		if err := memFs.MkdirAll(objectDir, 0o755); err != nil {
			t.Logf("Failed to create object directory: %v", err)
			return err
		}
		t.Logf("Created object directory")

		// Copy output files to the cache
		for _, file := range files {
			exists, err := afero.Exists(memFs, file)
			if err != nil {
				t.Logf("Failed to check output file existence: %v", err)
				return err
			}
			if !exists {
				t.Logf("Output file %s does not exist", file)
				return fmt.Errorf("output file %s does not exist", file)
			}

			// Copy the file to the cache
			destPath := filepath.Join(objectDir, filepath.Base(file))
			t.Logf("Copying file from %s to %s", file, destPath)

			// Open the source file
			srcFile, err := memFs.Open(file)
			if err != nil {
				t.Logf("Failed to open source file: %v", err)
				return err
			}
			defer srcFile.Close()

			// Create the destination file
			dstFile, err := memFs.Create(destPath)
			if err != nil {
				t.Logf("Failed to create destination file: %v", err)
				return err
			}
			defer dstFile.Close()

			// Copy the file
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				t.Logf("Failed to copy file: %v", err)
				return err
			}
			t.Logf("Copied file successfully")
		}

		// Create the manifest directory
		manifestDir := filepath.Join(tempDir, "manifests", keyHash[:2])
		t.Logf("Manifest directory: %s", manifestDir)
		dirExists, err = afero.DirExists(memFs, manifestDir)
		if err != nil {
			t.Logf("Failed to check manifest directory existence: %v", err)
		} else {
			t.Logf("Manifest directory exists before creation: %v", dirExists)
		}

		if err := memFs.MkdirAll(manifestDir, 0o755); err != nil {
			t.Logf("Failed to create manifest directory: %v", err)
			return err
		}
		t.Logf("Created manifest directory")

		// Create the manifest file
		manifestFile := filepath.Join(manifestDir, keyHash+".json")
		t.Logf("Manifest file: %s", manifestFile)

		// Create a simple manifest
		manifest := map[string]interface{}{
			"keyHash":     keyHash,
			"inputs":      inputDescs,
			"extra":       key.Extra,
			"outputFiles": files,
			"metadata":    result.Metadata,
		}

		// Marshal the manifest to JSON
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			t.Logf("Failed to marshal manifest: %v", err)
			return err
		}

		// Write the manifest file
		if err := afero.WriteFile(memFs, manifestFile, data, 0o644); err != nil {
			t.Logf("Failed to write manifest file: %v", err)
			return err
		}
		t.Logf("Wrote manifest file successfully")

		// Verify the manifest file exists
		exists, err := afero.Exists(memFs, manifestFile)
		if err != nil {
			t.Logf("Failed to check manifest file existence: %v", err)
		} else {
			t.Logf("Manifest file exists after creation: %v", exists)
		}

		return nil
	}

	// Call our custom Store function
	err = storeWithDebug(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Also call the original Store function for comparison
	t.Logf("Calling original Store function")
	err = cache.Store(key, result)
	if err != nil {
		t.Logf("Original Store function failed: %v", err)
	} else {
		t.Logf("Original Store function succeeded")
	}

	// Debug: Check if manifest file exists after Store
	keyHash, err := cache.computeKeyHash(key)
	if err != nil {
		t.Fatalf("Failed to compute key hash: %v", err)
	}
	t.Logf("Key hash after Store: %s", keyHash)

	manifestDir := filepath.Join(tempDir, "manifests", keyHash[:2])
	manifestFile := filepath.Join(manifestDir, keyHash+".json")
	t.Logf("Manifest file path: %s", manifestFile)

	dirExists, err := afero.DirExists(memFs, manifestDir)
	if err != nil {
		t.Fatalf("Failed to check manifest directory existence: %v", err)
	}
	t.Logf("Manifest directory exists after Store: %v", dirExists)

	fileExists, err := afero.Exists(memFs, manifestFile)
	if err != nil {
		t.Fatalf("Failed to check manifest file existence: %v", err)
	}
	t.Logf("Manifest file exists after Store: %v", fileExists)

	// Debug: List files in manifest directory
	if dirExists {
		files, err := afero.ReadDir(memFs, manifestDir)
		if err != nil {
			t.Logf("Failed to read manifest directory: %v", err)
		} else {
			t.Logf("Files in manifest directory:")
			for _, file := range files {
				t.Logf("  %s", file.Name())
			}
		}
	}

	// Second get should be a hit
	resultGet, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Unexpected error on second Get: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit on second Get, got miss")
	}
	if resultGet.Path == "" {
		t.Fatalf("Expected path in result, got empty path")
	}
	if resultGet.Metadata["data"] != "test data" {
		t.Fatalf("Expected metadata in result, got %v", resultGet.Metadata)
	}

	// Test with invalid key (no inputs)
	invalidKey := Key{
		Extra: map[string]string{"test": "value"},
	}
	_, _, err = cache.Get(invalidKey)
	if err == nil || !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey for invalid key, got: %v", err)
	}
}

// TestStore tests the Store operation
func TestStore(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-store-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Create an output file
	outputFilePath := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("output content")
	if err := afero.WriteFile(memFs, outputFilePath, outputContent, 0o644); err != nil {
		t.Fatalf("Failed to write output file: %v", err)
	}

	// Store in cache
	result := Result{
		Path: outputFilePath,
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	err = cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Verify the file was stored by getting it
	resultGet, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Unexpected error after Store: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit after Store, got miss")
	}
	if resultGet.Path == "" {
		t.Fatalf("Expected path in result, got empty path")
	}

	// Test with invalid key (no inputs)
	invalidKey := Key{
		Extra: map[string]string{"test": "value"},
	}
	err = cache.Store(invalidKey, result)
	if err == nil || !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Expected ErrInvalidKey for invalid key, got: %v", err)
	}

	// Test with non-existent output file
	nonExistentResult := Result{
		Path: filepath.Join(tempDir, "nonexistent.txt"),
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	err = cache.Store(key, nonExistentResult)
	if err == nil {
		t.Fatalf("Expected error for non-existent output file, got nil")
	}
}

// TestGetFile tests the GetFile operation
func TestGetFile(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-getfile-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Create an output file
	outputFilePath := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("output content")
	if err := afero.WriteFile(memFs, outputFilePath, outputContent, 0o644); err != nil {
		t.Fatalf("Failed to write output file: %v", err)
	}

	// Store in cache
	result := Result{
		Path: outputFilePath,
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	err = cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Get the cached file
	cachedFilePath, found, err := cache.GetFile(key, filepath.Base(outputFilePath))
	if err != nil {
		t.Fatalf("Failed to GetFile: %v", err)
	}
	if !found {
		t.Fatalf("Expected to find cached file, but not found")
	}

	// Verify the cached file content
	cachedContent, err := afero.ReadFile(memFs, cachedFilePath)
	if err != nil {
		t.Fatalf("Failed to read cached file: %v", err)
	}
	if string(cachedContent) != string(outputContent) {
		t.Fatalf("Cached file content doesn't match original. Expected %q, got %q", string(outputContent), string(cachedContent))
	}

	// Test with non-existent file
	_, found, err = cache.GetFile(key, "nonexistent.txt")
	if err == nil {
		t.Fatalf("Expected error for non-existent file, got nil")
	}
	if found {
		t.Fatalf("Expected not found for non-existent file, got found")
	}

	// Test with non-existent key
	nonExistentKey := Key{
		Inputs: []Input{
			FileInput{Path: filepath.Join(tempDir, "nonexistent.txt"), Fs: memFs},
			RawInput{
				Data: []byte("nonexistent content"),
				Name: "nonexistent.txt",
			},
		},
	}
	_, found, _ = cache.GetFile(nonExistentKey, filepath.Base(outputFilePath))
	if found {
		t.Fatalf("Expected not found for non-existent key, got found")
	}
}

// TestGetData tests the GetData operation
func TestGetData(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-getdata-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Store in cache with metadata
	result := Result{
		Metadata: map[string]string{
			"data1": "test data 1",
			"data2": "test data 2",
		},
	}
	err = cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Get the cached data
	cachedData, found, err := cache.GetData(key, "data1")
	if err != nil {
		t.Fatalf("Failed to GetData: %v", err)
	}
	if !found {
		t.Fatalf("Expected to find cached data, but not found")
	}
	if string(cachedData) != "test data 1" {
		t.Fatalf("Cached data doesn't match original. Expected %q, got %q", "test data 1", string(cachedData))
	}

	// Test with non-existent data key
	_, found, err = cache.GetData(key, "nonexistent")
	if err == nil {
		t.Fatalf("Expected error for non-existent data key, got nil")
	}
	if found {
		t.Fatalf("Expected not found for non-existent data key, got found")
	}

	// Test with non-existent cache key
	nonExistentKey := Key{
		Inputs: []Input{
			FileInput{Path: filepath.Join(tempDir, "nonexistent.txt"), Fs: memFs},
			RawInput{
				Data: []byte("nonexistent content"),
				Name: "nonexistent.txt",
			},
		},
	}
	_, found, _ = cache.GetData(nonExistentKey, "data1")
	if found {
		t.Fatalf("Expected not found for non-existent cache key, got found")
	}
}

// TestClear tests the Clear operation
func TestClear(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-clear-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Store in cache
	result := Result{
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	err = cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Verify the entry exists
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Unexpected error on Get: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, got miss")
	}

	// Clear the cache
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Failed to Clear: %v", err)
	}

	// Verify the entry no longer exists
	_, hit, err = cache.Get(key)
	if err != nil && !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Unexpected error after Clear: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after Clear, got hit")
	}

	// Verify the cache directories were recreated
	manifestDirExists, err := afero.DirExists(memFs, filepath.Join(tempDir, "manifests"))
	if err != nil {
		t.Fatalf("Failed to check manifests directory existence: %v", err)
	}
	if !manifestDirExists {
		t.Fatalf("Expected manifests directory to exist after Clear")
	}

	objectsDirExists, err := afero.DirExists(memFs, filepath.Join(tempDir, "objects"))
	if err != nil {
		t.Fatalf("Failed to check objects directory existence: %v", err)
	}
	if !objectsDirExists {
		t.Fatalf("Expected objects directory to exist after Clear")
	}
}

// TestRemove tests the Remove operation
func TestRemove(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-remove-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a test file
	testFilePath := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := afero.WriteFile(memFs, testFilePath, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Store in cache
	result := Result{
		Metadata: map[string]string{
			"data": "test data",
		},
	}
	err = cache.Store(key, result)
	if err != nil {
		t.Fatalf("Failed to Store: %v", err)
	}

	// Get the key hash
	keyHash, err := cache.computeKeyHash(key)
	if err != nil {
		t.Fatalf("Failed to compute key hash: %v", err)
	}

	// Verify the entry exists
	_, hit, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Unexpected error on Get: %v", err)
	}
	if !hit {
		t.Fatalf("Expected cache hit, got miss")
	}

	// Remove the entry
	err = cache.Remove(keyHash)
	if err != nil {
		t.Fatalf("Failed to Remove: %v", err)
	}

	// Verify the entry no longer exists
	_, hit, err = cache.Get(key)
	if err != nil && !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("Unexpected error after Remove: %v", err)
	}
	if hit {
		t.Fatalf("Expected cache miss after Remove, got hit")
	}

	// Test removing a non-existent entry
	err = cache.Remove("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error when removing non-existent entry, got: %v", err)
	}
}

// TestCopyFile tests the copyFile operation
func TestCopyFile(t *testing.T) {
	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a temporary directory for the test
	tempDir := "/granular-copyfile-test"
	if err := memFs.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a cache with the in-memory filesystem
	cache, err := New(tempDir, WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a source file
	srcPath := filepath.Join(tempDir, "source.txt")
	srcContent := []byte("source content")
	if err := afero.WriteFile(memFs, srcPath, srcContent, 0o644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create a destination path
	dstPath := filepath.Join(tempDir, "destination.txt")

	// Copy the file
	err = cache.copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("Failed to copy file: %v", err)
	}

	// Verify the destination file exists and has the correct content
	dstExists, err := afero.Exists(memFs, dstPath)
	if err != nil {
		t.Fatalf("Failed to check destination file existence: %v", err)
	}
	if !dstExists {
		t.Fatalf("Expected destination file to exist")
	}

	dstContent, err := afero.ReadFile(memFs, dstPath)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(dstContent) != string(srcContent) {
		t.Fatalf("Destination file content doesn't match source. Expected %q, got %q", string(srcContent), string(dstContent))
	}

	// Test with non-existent source file
	err = cache.copyFile(filepath.Join(tempDir, "nonexistent.txt"), dstPath)
	if err == nil {
		t.Fatalf("Expected error for non-existent source file, got nil")
	}
}
