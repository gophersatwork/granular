package granular

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
)

// WriteBuilder provides a fluent API for storing cache results.
// Users should not construct this directly, use Cache.Put() instead.
type WriteBuilder struct {
	cache            *Cache
	key              Key
	files            map[string]string // name -> source path
	data             map[string][]byte // name -> bytes
	metadata         map[string]string // metadata key-value pairs
	errors           []error           // Accumulated validation errors (from key + write operations)
	accumulateErrors bool              // If true, accumulate all errors; if false, fail-fast
}

// File adds a file to be stored in the cache.
// name is the logical name for this file (used to retrieve it later).
// srcPath is the path to the file to cache.
// Validates the file and accumulates any errors.
// Errors are only surfaced when Commit() is called.
func (wb *WriteBuilder) File(name, srcPath string) *WriteBuilder {
	// If fail-fast and already have errors, skip validation
	if !wb.accumulateErrors && len(wb.errors) > 0 {
		if wb.files == nil {
			wb.files = make(map[string]string)
		}
		wb.files[name] = srcPath
		return wb
	}

	// Validate source file exists
	exists, err := afero.Exists(wb.cache.fs, srcPath)
	if err != nil {
		wb.errors = append(wb.errors, fmt.Errorf("failed to check file %s: %w", srcPath, err))
	} else if !exists {
		wb.errors = append(wb.errors, fmt.Errorf("source file does not exist: %s", srcPath))
	} else {
		// Validate it's a file, not a directory (only if it exists)
		info, err := wb.cache.fs.Stat(srcPath)
		if err != nil {
			wb.errors = append(wb.errors, fmt.Errorf("failed to stat file %s: %w", srcPath, err))
		} else if info.IsDir() {
			wb.errors = append(wb.errors, fmt.Errorf("source path is a directory, not a file: %s", srcPath))
		}
	}

	if wb.files == nil {
		wb.files = make(map[string]string)
	}
	wb.files[name] = srcPath
	return wb
}

// Bytes adds byte data to be stored in the cache.
// name is the logical name for this data (used to retrieve it later).
func (wb *WriteBuilder) Bytes(name string, data []byte) *WriteBuilder {
	if wb.data == nil {
		wb.data = make(map[string][]byte)
	}
	// Store a copy to prevent mutations
	wb.data[name] = append([]byte(nil), data...)
	return wb
}

// Meta adds metadata to the cache entry.
// Metadata is stored as string key-value pairs.
func (wb *WriteBuilder) Meta(key, value string) *WriteBuilder {
	if wb.metadata == nil {
		wb.metadata = make(map[string]string)
	}
	wb.metadata[key] = value
	return wb
}

// Commit finalizes and stores the cache entry.
// Returns a ValidationError if there are accumulated errors from key building or write operations.
// Returns an error if the storage operation fails.
func (wb *WriteBuilder) Commit() error {
	// Check for accumulated validation errors first
	if len(wb.errors) > 0 {
		return newValidationError(wb.errors)
	}

	wb.cache.mu.Lock()
	defer wb.cache.mu.Unlock()

	// Compute key hash (this will check for key validation errors)
	keyHash, err := wb.key.computeHash()
	if err != nil {
		return fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Create object directory
	objectDir := wb.cache.objectPath(keyHash)
	if err := wb.cache.fs.MkdirAll(objectDir, 0o755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	// Copy all files to cache
	cachedFiles := make(map[string]string)
	for name, srcPath := range wb.files {
		// Generate destination filename (preserve basename)
		dstName := filepath.Base(srcPath)
		dstPath := filepath.Join(objectDir, dstName)

		// Copy the file
		if err := wb.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", name, err)
		}

		cachedFiles[name] = dstPath
	}

	// Write byte data to cache as files (but don't add to cachedFiles - keep separate)
	for name, data := range wb.data {
		// Store data as a file with .dat extension
		dstPath := filepath.Join(objectDir, name+".dat")
		if err := afero.WriteFile(wb.cache.fs, dstPath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write data %s: %w", name, err)
		}
		// Note: Not adding to cachedFiles - data is kept separate from files
	}

	// Build input descriptions for manifest
	inputDescs := make([]string, len(wb.key.inputs))
	for i, input := range wb.key.inputs {
		inputDescs[i] = input.String()
	}

	// Create output file list (for hash computation)
	outputFiles := make([]string, 0, len(wb.files))
	for _, srcPath := range wb.files {
		outputFiles = append(outputFiles, srcPath)
	}

	// Compute output hash
	outputHash, err := wb.cache.computeOutputHash(outputFiles, wb.data, wb.metadata)
	if err != nil {
		return fmt.Errorf("failed to compute output hash: %w", err)
	}

	// Create and save manifest
	manifest := &manifest{
		KeyHash:     keyHash,
		InputDescs:  inputDescs,
		ExtraData:   wb.key.extras,
		OutputFiles: cachedFiles,
		OutputData:  wb.data,
		OutputMeta:  wb.metadata,
		OutputHash:  outputHash,
		CreatedAt:   wb.cache.now(),
		AccessedAt:  wb.cache.now(),
	}

	if err := wb.cache.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst.
func (wb *WriteBuilder) copyFile(src, dst string) error {
	srcFile, err := wb.cache.fs.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := wb.cache.fs.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}
