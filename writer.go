package granular

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

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
	startTime := time.Now()

	// Check for accumulated validation errors first (no lock needed)
	if len(wb.errors) > 0 {
		return newValidationError(wb.errors)
	}

	// Compute key hash BEFORE locking (pure computation, no lock needed)
	keyHash, err := wb.key.computeHash()
	if err != nil {
		return fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Estimate required space for this entry (before acquiring locks)
	requiredSpace, err := wb.estimateSize()
	if err != nil {
		return fmt.Errorf("failed to estimate entry size: %w", err)
	}

	// If max size is set, perform eviction under exclusive global lock.
	if wb.cache.maxSize > 0 {
		wb.cache.mu.Lock()
		if err := wb.cache.evictIfNeeded(requiredSpace); err != nil {
			wb.cache.mu.Unlock()
			wb.cache.metrics.error("put", err)
			return fmt.Errorf("failed to evict entries: %w", err)
		}
		wb.cache.mu.Unlock()
	}

	// Hold global read lock during the write phase to prevent Clear() from
	// removing directories while files are being written. Multiple Put()
	// calls can proceed concurrently since they all hold RLock.
	wb.cache.mu.RLock()
	defer wb.cache.mu.RUnlock()

	// Use per-key lock for concurrent writes to different keys
	wb.cache.keyLocks.lockKey(keyHash)
	defer wb.cache.keyLocks.unlockKey(keyHash)

	// Create object directory
	objectDir := wb.cache.objectPath(keyHash)
	if err := wb.cache.fs.MkdirAll(objectDir, 0o755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	// Copy all files to cache.
	// Uses "file.<name>.<ext>" as the destination to avoid basename collisions
	// when different source paths share the same filename.
	cachedFiles := make(map[string]string)
	for name, srcPath := range wb.files {
		ext := filepath.Ext(srcPath)
		dstPath := filepath.Join(objectDir, "file."+name+ext)

		if err := wb.copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", name, err)
		}

		cachedFiles[name] = dstPath
	}

	// Write byte data to cache as files atomically and track paths for manifest.
	// Uses "data.<name>.dat" as the destination to namespace separately from files.
	cachedDataPaths := make(map[string]string, len(wb.data))
	for name, data := range wb.data {
		dstPath := filepath.Join(objectDir, "data."+name+".dat")
		if err := wb.writeDataFile(dstPath, data); err != nil {
			return fmt.Errorf("failed to write data %s: %w", name, err)
		}
		// Store the path to the .dat file in the manifest (not the raw bytes)
		cachedDataPaths[name] = dstPath
	}

	// Build input descriptions for manifest
	inputDescs := make([]string, len(wb.key.inputs))
	for i, ki := range wb.key.inputs {
		inputDescs[i] = ki.String()
	}

	// Create output file list for hash computation (use cached paths for consistency with verification)
	cachedFilePaths := make([]string, 0, len(cachedFiles))
	for _, cachedPath := range cachedFiles {
		cachedFilePaths = append(cachedFilePaths, cachedPath)
	}

	// Read back the compressed data from .dat files for hash computation
	// This ensures the hash matches what verification will compute
	cachedDataForHash := make(map[string][]byte, len(cachedDataPaths))
	for name, dataPath := range cachedDataPaths {
		data, err := afero.ReadFile(wb.cache.fs, dataPath)
		if err != nil {
			return fmt.Errorf("failed to read back cached data %s: %w", name, err)
		}
		cachedDataForHash[name] = data
	}

	// Compute output hash from cached files and data (both possibly compressed)
	outputHash, err := wb.cache.computeOutputHash(cachedFilePaths, cachedDataForHash, wb.metadata)
	if err != nil {
		return fmt.Errorf("failed to compute output hash: %w", err)
	}

	// Create and save manifest
	manifest := &manifest{
		Version:     1,                     // Current manifest format version
		HashAlgo:    wb.cache.hashAlgoName, // Hash algorithm for compatibility checking
		KeyHash:     keyHash,
		InputDescs:  inputDescs,
		ExtraData:   wb.key.extras,
		OutputFiles: cachedFiles,
		OutputData:  cachedDataPaths, // Store paths to .dat files
		OutputMeta:  wb.metadata,
		OutputHash:  outputHash,
		Compression: wb.cache.compression,
		CreatedAt:   wb.cache.now(),
		AccessedAt:  wb.cache.now(),
	}

	if err := wb.cache.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Report successful put with duration
	wb.cache.metrics.put(keyHash, requiredSpace, time.Since(startTime))

	return nil
}

// copyFile copies a file from src to dst atomically, applying compression if configured.
// Uses temp file + rename to prevent corruption from crashes during copy.
func (wb *WriteBuilder) copyFile(src, dst string) error {
	srcFile, err := wb.cache.fs.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	// Write to temp file first for atomic operation
	tmpPath := dst + ".tmp." + randomSuffix()
	dstFile, err := wb.cache.fs.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	// Wrap with compression if configured
	compWriter, err := compressWriter(dstFile, wb.cache.compression)
	if err != nil {
		_ = dstFile.Close()
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to create compressor: %w", err)
	}

	_, err = io.CopyBuffer(compWriter, srcFile, buffer)
	if closeErr := compWriter.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if closeErr := dstFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		// Cleanup temp file on copy failure
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to copy: %w", err)
	}

	// Atomic rename to final path
	if err := wb.cache.fs.Rename(tmpPath, dst); err != nil {
		// Cleanup temp file on rename failure
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// writeDataFile writes byte data to a file atomically, applying compression if configured.
func (wb *WriteBuilder) writeDataFile(dst string, data []byte) error {
	tmpPath := dst + ".tmp." + randomSuffix()
	dstFile, err := wb.cache.fs.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Wrap with compression if configured
	compWriter, err := compressWriter(dstFile, wb.cache.compression)
	if err != nil {
		_ = dstFile.Close()
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to create compressor: %w", err)
	}

	_, err = compWriter.Write(data)
	if closeErr := compWriter.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if closeErr := dstFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Atomic rename to final path
	if err := wb.cache.fs.Rename(tmpPath, dst); err != nil {
		_ = wb.cache.fs.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// estimateSize calculates the approximate size of the data to be written.
// This includes all files and byte data that will be stored in the objects directory.
// Note: This is a pre-compression estimate. With compression enabled, actual stored
// size will be smaller than this estimate.
func (wb *WriteBuilder) estimateSize() (int64, error) {
	var totalSize int64

	// Sum up file sizes
	for _, srcPath := range wb.files {
		info, err := wb.cache.fs.Stat(srcPath)
		if err != nil {
			return 0, fmt.Errorf("failed to stat file %s: %w", srcPath, err)
		}
		totalSize += info.Size()
	}

	// Sum up byte data sizes
	for _, data := range wb.data {
		totalSize += int64(len(data))
	}

	return totalSize, nil
}
