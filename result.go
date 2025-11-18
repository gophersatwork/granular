package granular

import (
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// Result represents a cached result with support for multiple files and data.
// Users should not construct this directly - it's returned by Cache.Get().
type Result struct {
	keyHash    string
	cache      *Cache
	files      map[string]string // name -> cached file path
	data       map[string][]byte // name -> bytes
	metadata   map[string]string // metadata key-value pairs
	createdAt  time.Time
	accessedAt time.Time
}

// File returns the path to a cached file by name.
// Returns empty string if the file doesn't exist.
func (r *Result) File(name string) string {
	return r.files[name]
}

// Files returns all cached files as a map of name -> path.
func (r *Result) Files() map[string]string {
	result := make(map[string]string, len(r.files))
	for k, v := range r.files {
		result[k] = v
	}
	return result
}

// HasFile returns true if a file with the given name exists in the cache.
func (r *Result) HasFile(name string) bool {
	_, ok := r.files[name]
	return ok
}

// CopyFile copies a cached file to the destination path.
// Returns an error if the file doesn't exist or the copy fails.
func (r *Result) CopyFile(name, dst string) error {
	src := r.files[name]
	if src == "" {
		return fmt.Errorf("file %s not found in cache", name)
	}

	// Create destination directory if needed
	dstDir := filepath.Dir(dst)
	if dstDir != "." && dstDir != "" {
		if err := r.cache.fs.MkdirAll(dstDir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dstDir, err)
		}
	}

	// Copy the file
	srcFile, err := r.cache.fs.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open cached file %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := r.cache.fs.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Bytes returns byte data by name.
// Returns nil if the data doesn't exist.
func (r *Result) Bytes(name string) []byte {
	return r.data[name]
}

// Data returns all byte data as a map of name -> bytes.
func (r *Result) Data() map[string][]byte {
	result := make(map[string][]byte, len(r.data))
	for k, v := range r.data {
		// Return copy to prevent mutation
		result[k] = append([]byte(nil), v...)
	}
	return result
}

// HasData returns true if data with the given name exists in the cache.
func (r *Result) HasData(name string) bool {
	_, ok := r.data[name]
	return ok
}

// Meta returns metadata by key.
// Returns empty string if the key doesn't exist.
func (r *Result) Meta(key string) string {
	return r.metadata[key]
}

// Metadata returns all metadata as a map.
func (r *Result) Metadata() map[string]string {
	result := make(map[string]string, len(r.metadata))
	for k, v := range r.metadata {
		result[k] = v
	}
	return result
}

// HasMeta returns true if metadata with the given key exists.
func (r *Result) HasMeta(key string) bool {
	_, ok := r.metadata[key]
	return ok
}

// Age returns how long ago this result was created.
func (r *Result) Age() time.Duration {
	return r.cache.now().Sub(r.createdAt)
}

// CreatedAt returns when this result was originally cached.
func (r *Result) CreatedAt() time.Time {
	return r.createdAt
}

// AccessedAt returns when this result was last accessed.
func (r *Result) AccessedAt() time.Time {
	return r.accessedAt
}

// Size returns the total size of all cached files in bytes.
// Returns 0 if unable to determine size.
func (r *Result) Size() int64 {
	var total int64
	for _, path := range r.files {
		info, err := r.cache.fs.Stat(path)
		if err == nil {
			total += info.Size()
		}
	}
	return total
}

// KeyHash returns the hash of the cache key for this result.
// Useful for debugging and logging.
func (r *Result) KeyHash() string {
	return r.keyHash
}

// fileNames returns a sorted list of all file names in this result.
func (r *Result) fileNames() []string {
	names := make([]string, 0, len(r.files))
	for name := range r.files {
		names = append(names, name)
	}
	return names
}

// dataNames returns a sorted list of all data names in this result.
func (r *Result) dataNames() []string {
	names := make([]string, 0, len(r.data))
	for name := range r.data {
		names = append(names, name)
	}
	return names
}
