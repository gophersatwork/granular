package granular

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/spf13/afero"
)

// TestHashFile tests the standalone hashFile function
// The main idea is to test if the hashing interacting with the abstractions preserve the results compared to using the hash directly
func TestHashFile(t *testing.T) {
	// Create a temporary directory for the test
	memFs := afero.NewMemMapFs()
	tmpDir, err := afero.TempDir(memFs, "", "hash-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Test cases
	testCases := []struct {
		name     string
		content  []byte
		fileFunc func(string) string
	}{
		{
			name:    "Normal file",
			content: []byte("test content"),
			fileFunc: func(dir string) string {
				path := filepath.Join(dir, "normal.txt")
				if err := afero.WriteFile(memFs, path, []byte("test content"), 0o644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				return path
			},
		},
		{
			name:    "Empty file",
			content: []byte{},
			fileFunc: func(dir string) string {
				path := filepath.Join(dir, "empty.txt")
				if err := afero.WriteFile(memFs, path, []byte{}, 0o644); err != nil {
					t.Fatalf("Failed to write empty file: %v", err)
				}
				return path
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := tc.fileFunc(tmpDir)

			// Create two hash instances to compare results
			h1 := xxhash.New()
			h2 := xxhash.New()

			file, err := memFs.Open(filePath)
			if err != nil {
				t.Fatalf("Failed to open file: %v", err)
			}
			defer file.Close()

			// Hash the file using our hashFile
			err = hashFile(file, h1)
			// Check error expectation
			if err != nil {
				t.Errorf("hashFile() error = %v", err)
				return
			}

			// Hash the content directly
			h2.Write(tc.content)

			// Compare the hashes
			if !bytes.Equal(h1.Sum(nil), h2.Sum(nil)) {
				t.Errorf("hashFile() produced different hash than direct hashing")
			}
		})
	}
}

// TestHashFile tests failures for the standalone hashFile function
func TestHashFile_Fail(t *testing.T) {
	// Create a temporary directory for the test
	memFs := afero.NewMemMapFs()
	tmpDir, err := afero.TempDir(memFs, "", "hash-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	t.Run("Non-existent file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "nonexistent.txt")

		// For non-existent file test, try to open it to generate the expected error
		_, err := memFs.Open(filePath)
		if err == nil {
			t.Errorf("Expected error opening non-existent file, but got none")
		}
	})
}

// TestCacheHashFile tests the Cache.hashInput method
// The main idea is to test if the hashing interacting with the abstractions preserve the results compared to using the hash directly
func TestCacheHashFile(t *testing.T) {
	// Create a cache with memory filesystem
	memFs := afero.NewMemMapFs()
	cache, err := New("", WithNowFunc(fixedNowFunc), WithFs(memFs), WithHashFunc(defaultHashFunc))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test cases
	testCases := []struct {
		name     string
		content  []byte
		size     int64
		fileFunc func(afero.Fs) string
	}{
		{
			name:    "Small file",
			content: []byte("small file content"),
			size:    int64(len([]byte("small file content"))),
			fileFunc: func(fs afero.Fs) string {
				path := "/small.txt"
				if err := afero.WriteFile(fs, path, []byte("small file content"), 0o644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				return path
			},
		},
		{
			name:    "Empty file",
			content: []byte{},
			size:    0,
			fileFunc: func(fs afero.Fs) string {
				path := "/empty.txt"
				if err := afero.WriteFile(fs, path, []byte{}, 0o644); err != nil {
					t.Fatalf("Failed to write empty file: %v", err)
				}
				return path
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache.hash.Reset()
			filePath := tc.fileFunc(memFs)

			// Create two hash instances to compare results
			h1 := cache.hash
			h2 := xxhash.New()

			input := FileInput{
				Path: filePath,
				Fs:   memFs,
			}

			// Hash the input
			err := cache.hashInput(input)
			if err != nil {
				t.Errorf("Cache.hashInput() error = %v, but expected none", err)
				return
			}

			// Hash the content directly
			h2.Write(tc.content)

			// Compare the hashes
			if !bytes.Equal(h1.Sum(nil), h2.Sum(nil)) {
				t.Errorf("Cache.hashFile() produced different hash than direct hashing")
			}
		})
	}
}

func TestCacheHashFile_Fail(t *testing.T) {
	// Create a cache with memory filesystem
	memFs := afero.NewMemMapFs()
	cache, err := New("", WithNowFunc(fixedNowFunc), WithFs(memFs))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test cases
	testCases := []struct {
		name     string
		fileFunc func(afero.Fs) string
	}{
		{
			name: "Non-existent file",
			fileFunc: func(fs afero.Fs) string {
				return "/nonexistent.txt"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := tc.fileFunc(memFs)

			input := FileInput{
				Path: filePath,
				Fs:   memFs,
			}
			err := cache.hashInput(input)
			if err == nil {
				t.Error("Cache.hashInput() should fail, but there is no error")
				return
			}
		})
	}
}

// TestSpecialCharacters tests hashing files with special characters in their names
func TestSpecialCharacters(t *testing.T) {
	memFs := afero.NewMemMapFs()
	cache, err := New("", WithNowFunc(fixedNowFunc), WithFs(memFs), WithHashFunc(defaultHashFunc))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test content
	content := []byte("content for special character test")

	// Test cases with special characters in filenames
	specialNames := []string{
		"/special-!@#$%^&*().txt",
		"/space file.txt",
		"/unicode-文件.txt",
		"/emoji-😀.txt",
	}

	for _, name := range specialNames {
		t.Run(name, func(t *testing.T) {
			cache.hash.Reset()
			// Write file
			if err := afero.WriteFile(memFs, name, content, 0o644); err != nil {
				t.Fatalf("Failed to write file %s: %v", name, err)
			}

			// Create hash instances
			h1 := cache.hash   // From cache
			h2 := xxhash.New() // For direct hashing

			input := FileInput{
				Path: name,
				Fs:   memFs,
			}
			// Hash using Cache.hashFile
			if err := cache.hashInput(input); err != nil {
				t.Fatalf("Cache.hashFile failed for %s: %v", name, err)
			}

			// Hash directly
			h2.Write(content)

			// Compare hashes
			if !bytes.Equal(h1.Sum(nil), h2.Sum(nil)) {
				t.Errorf("Cache.hashFile produced different hash than direct hashing for %s", name)
			}
		})
	}
}

// TestBufferPoolReuse tests that the buffer pool is properly reused
func TestBufferPoolReuse(t *testing.T) {
	// Create a memory filesystem
	memFs := afero.NewMemMapFs()

	// Create a test file
	filePath := "/test.txt"
	content := []byte("test content for buffer pool test")
	if err := afero.WriteFile(memFs, filePath, content, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Get a buffer from the pool
	bufPtr1 := bufferPool.Get().(*[]byte)
	buffer1 := *bufPtr1

	// Use the buffer
	h := xxhash.New()
	file, err := memFs.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	_, err = io.CopyBuffer(h, file, buffer1)
	if err != nil {
		t.Fatalf("Failed to copy: %v", err)
	}

	// Put the buffer back
	bufferPool.Put(bufPtr1)

	// Get another buffer
	bufPtr2 := bufferPool.Get().(*[]byte)
	buffer2 := *bufPtr2
	defer bufferPool.Put(bufPtr2)

	// Check if it's the same buffer (by capacity and length)
	if cap(buffer1) != cap(buffer2) || len(buffer1) != len(buffer2) {
		t.Errorf("Buffer pool not reusing buffers: cap1=%d, len1=%d, cap2=%d, len2=%d",
			cap(buffer1), len(buffer1), cap(buffer2), len(buffer2))
	}
}
