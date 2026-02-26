package granular

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
)

// Result represents a cached result with support for multiple files and data.
// Users should not construct this directly - it's returned by Cache.Get().
//
// A Result is not safe for concurrent use by multiple goroutines.
type Result struct {
	keyHash     string
	cache       *Cache
	files       map[string]string // name -> cached file path
	dataPaths   map[string]string // name -> path to .dat file (lazy loading)
	dataCache   map[string][]byte // lazy-loaded cache for data bytes
	metadata    map[string]string // metadata key-value pairs
	compression CompressionType   // compression used for stored data
	createdAt   time.Time
	accessedAt  time.Time
}

// File returns the path to a cached file by name.
// Returns empty string if the file doesn't exist.
func (r *Result) File(name string) string {
	return r.files[name]
}

// Files returns all cached files as a map of name -> path.
func (r *Result) Files() map[string]string {
	return maps.Clone(r.files)
}

// HasFile returns true if a file with the given name exists in the cache.
func (r *Result) HasFile(name string) bool {
	_, ok := r.files[name]
	return ok
}

// CopyFile copies a cached file to the destination path, decompressing if needed.
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

	// Open source file
	srcFile, err := r.cache.fs.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open cached file %s: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	// Wrap with decompression if needed
	reader, err := decompressReader(srcFile, r.compression)
	if err != nil {
		return fmt.Errorf("failed to create decompressor: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Write to temp file first, then atomic rename to prevent partial files on error
	tmpPath := dst + ".tmp." + randomSuffix()
	dstFile, err := r.cache.fs.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	_, copyErr := io.CopyBuffer(dstFile, reader, buffer)
	closeErr := dstFile.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = r.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := r.cache.fs.Rename(tmpPath, dst); err != nil {
		_ = r.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Bytes returns byte data by name.
// Returns nil if the data doesn't exist or if there's a read/decompression error.
// Data is lazy-loaded from disk on first access and decompressed if needed.
// Use BytesErr for explicit error handling.
func (r *Result) Bytes(name string) []byte {
	data, _ := r.BytesErr(name)
	return data
}

// BytesErr returns byte data by name, with explicit error reporting.
// Returns (nil, nil) if the data name doesn't exist in the cache entry.
// Returns (nil, error) if the data exists but failed to read or decompress.
// Data is lazy-loaded from disk on first access and decompressed if needed.
func (r *Result) BytesErr(name string) ([]byte, error) {
	// Check if already cached
	if r.dataCache != nil {
		if data, ok := r.dataCache[name]; ok {
			return data, nil
		}
	}

	// Check if path exists
	path, ok := r.dataPaths[name]
	if !ok {
		return nil, nil
	}

	// Lazy load from disk with decompression
	data, err := r.readCompressedFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached data %s: %w", name, err)
	}

	// Cache for future access
	if r.dataCache == nil {
		r.dataCache = make(map[string][]byte)
	}
	r.dataCache[name] = data

	return data, nil
}

// readCompressedFile reads a file and decompresses it if needed.
func (r *Result) readCompressedFile(path string) ([]byte, error) {
	file, err := r.cache.fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	reader, err := decompressReader(file, r.compression)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()

	return io.ReadAll(reader)
}

// Data returns all byte data as a map of name -> bytes.
// Data is lazy-loaded from disk. Entries that fail to read are silently skipped.
// Use DataErr for explicit error handling.
func (r *Result) Data() map[string][]byte {
	result := make(map[string][]byte, len(r.dataPaths))
	for name := range r.dataPaths {
		data := r.Bytes(name)
		if data != nil {
			// Return copy to prevent mutation
			result[name] = bytes.Clone(data)
		}
	}
	return result
}

// DataErr returns all byte data as a map of name -> bytes, with explicit error reporting.
// Unlike Data(), it returns an error if any entry fails to read or decompress.
// Data is lazy-loaded from disk.
func (r *Result) DataErr() (map[string][]byte, error) {
	result := make(map[string][]byte, len(r.dataPaths))
	for name := range r.dataPaths {
		data, err := r.BytesErr(name)
		if err != nil {
			return nil, fmt.Errorf("failed to load data %s: %w", name, err)
		}
		if data != nil {
			result[name] = bytes.Clone(data)
		}
	}
	return result, nil
}

// DataIter returns an iterator over all data entries as name-bytes pairs.
// Data is lazy-loaded from disk on each iteration. Entries that fail to read
// are silently skipped. Use DataIterErr for explicit error handling.
// Unlike Data(), this avoids materializing all entries into a map at once.
func (r *Result) DataIter() iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {
		for name := range r.dataPaths {
			data := r.Bytes(name)
			if data != nil {
				if !yield(name, bytes.Clone(data)) {
					return
				}
			}
		}
	}
}

// DataIterErr returns an iterator over all data entries as name-bytes pairs,
// with explicit error handling via errPtr. If a read or decompression fails,
// the error is written to errPtr and iteration stops.
// Unlike DataErr(), this avoids materializing all entries into a map at once.
func (r *Result) DataIterErr(errPtr *error) iter.Seq2[string, []byte] {
	return func(yield func(string, []byte) bool) {
		for name := range r.dataPaths {
			data, err := r.BytesErr(name)
			if err != nil {
				*errPtr = fmt.Errorf("failed to load data %s: %w", name, err)
				return
			}
			if data != nil {
				if !yield(name, bytes.Clone(data)) {
					return
				}
			}
		}
	}
}

// HasData returns true if data with the given name exists in the cache.
func (r *Result) HasData(name string) bool {
	_, ok := r.dataPaths[name]
	return ok
}

// Meta returns metadata by key.
// Returns empty string if the key doesn't exist.
func (r *Result) Meta(key string) string {
	return r.metadata[key]
}

// Metadata returns all metadata as a map.
func (r *Result) Metadata() map[string]string {
	return maps.Clone(r.metadata)
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

// Size returns the total size of all cached files and data in bytes.
// Returns 0 if unable to determine size.
func (r *Result) Size() int64 {
	var total int64
	for path := range maps.Values(r.files) {
		info, err := r.cache.fs.Stat(path)
		if err == nil {
			total += info.Size()
		}
	}
	for path := range maps.Values(r.dataPaths) {
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

// Valid reports whether this Result's underlying cache entry still exists on disk.
// Returns false after the entry has been removed by Delete, Prune, Clear, or GC.
// This is a point-in-time check — the entry could be deleted immediately after
// Valid returns true.
func (r *Result) Valid() bool {
	mPath, err := r.cache.manifestPath(r.keyHash)
	if err != nil {
		return false
	}
	exists, err := afero.Exists(r.cache.fs, mPath)
	return err == nil && exists
}

// DataNames returns an iterator over the names of all data entries in the result.
// Use BytesErr to load the actual data for a given name.
func (r *Result) DataNames() iter.Seq[string] {
	return maps.Keys(r.dataPaths)
}

// FileNames returns an iterator over the names of all file entries in the result.
// Use File to get the cached path for a given name.
func (r *Result) FileNames() iter.Seq[string] {
	return maps.Keys(r.files)
}
