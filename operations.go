package granular

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
)

var (
	// ErrCacheMiss is returned when a cache key is not found.
	ErrCacheMiss = errors.New("cache miss")

	// ErrInvalidKey is returned when a key is invalid.
	ErrInvalidKey = errors.New("invalid key")
)

// Result represents the result of a cache operation.
type Result struct {
	// Path contains the path to the cached file.
	Path string

	// Metadata contains additional information about the cached result.
	Metadata map[string]string
}

// Get retrieves a cached result for the given key.
// It returns the result, a boolean indicating whether the key was found,
// and an error if one occurred.
func (c *Cache) Get(key Key) (*Result, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Validate the key
	if len(key.Inputs) == 0 {
		return nil, false, fmt.Errorf("%w: key has no inputs", ErrInvalidKey)
	}

	// Compute the key hash
	keyHash, err := c.computeKeyHash(key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Check if the manifest exists on disk
	manifestFile := c.manifestPath(keyHash)
	exists, err := afero.Exists(c.fs, manifestFile)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check manifest existence: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	// Load the manifest
	manifest, err := c.loadManifest(keyHash)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create the result
	// Use the first output file as the Path
	path := ""
	if len(manifest.OutputFiles) > 0 {
		objectDir := c.objectPath(keyHash)
		path = filepath.Join(objectDir, filepath.Base(manifest.OutputFiles[0]))
	}

	// Create metadata from OutputMeta (preferred) or OutputData (fallback)
	metadata := make(map[string]string)
	if len(manifest.OutputMeta) > 0 {
		// Use OutputMeta if available
		for k, v := range manifest.OutputMeta {
			metadata[k] = v
		}
	} else {
		// Fallback to OutputData for backward compatibility
		for k, v := range manifest.OutputData {
			metadata[k] = string(v)
		}
	}

	result := &Result{
		Path:     path,
		Metadata: metadata,
	}

	// Load any output data from files
	if len(manifest.OutputFiles) > 0 {
		objectDir := c.objectPath(keyHash)

		// Check if all output files exist
		for _, file := range manifest.OutputFiles {
			outputPath := filepath.Join(objectDir, filepath.Base(file))
			exists, err := afero.Exists(c.fs, outputPath)
			if err != nil {
				return nil, false, fmt.Errorf("failed to check output file existence: %w", err)
			}
			if !exists {
				return nil, false, fmt.Errorf("output file %s not found in cache", file)
			}
		}
	}

	return result, true, nil
}

// Store stores a result in the cache for the given key.
func (c *Cache) Store(key Key, result Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate the key
	if len(key.Inputs) == 0 {
		return fmt.Errorf("%w: key has no inputs", ErrInvalidKey)
	}

	// Compute the key hash
	keyHash, err := c.computeKeyHash(key)
	if err != nil {
		return fmt.Errorf("failed to compute key hash: %w", err)
	}

	// Create input descriptions
	inputDescs := make([]string, len(key.Inputs))
	for i, input := range key.Inputs {
		inputDescs[i] = input.String()
	}

	// Extract file path from result
	var files []string
	if result.Path != "" {
		files = append(files, result.Path)
	}

	// Convert metadata to output data and output meta
	outputData := make(map[string][]byte)
	outputMeta := make(map[string]string)
	for k, v := range result.Metadata {
		outputData[k] = []byte(v)
		outputMeta[k] = v
	}

	// Create the manifest
	manifest := &Manifest{
		KeyHash:     keyHash,
		InputDescs:  inputDescs,
		ExtraData:   key.Extra,
		OutputFiles: files,
		OutputData:  outputData,
		OutputMeta:  outputMeta,
		CreatedAt:   c.now(),
		AccessedAt:  c.now(),
		Description: "",
	}

	// Compute the output hash
	outputHash, err := c.computeOutputHash(files, outputData, outputMeta)
	if err != nil {
		return fmt.Errorf("failed to compute output hash: %w", err)
	}
	manifest.OutputHash = outputHash

	// Create the object directory
	objectDir := c.objectPath(keyHash)
	if err := c.fs.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	// Copy output files to the cache
	for _, file := range files {
		exists, err := afero.Exists(c.fs, file)
		if err != nil {
			return fmt.Errorf("failed to check output file existence: %w", err)
		}
		if !exists {
			return fmt.Errorf("output file %s does not exist", file)
		}

		// Copy the file to the cache
		destPath := filepath.Join(objectDir, filepath.Base(file))
		if err := c.copyFile(file, destPath); err != nil {
			return fmt.Errorf("failed to copy output file %s: %w", file, err)
		}
	}

	// Save the manifest
	if err := c.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst using the cache's filesystem.
func (c *Cache) copyFile(src, dst string) error {
	// Open the source file
	srcFile, err := c.fs.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := c.fs.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Get a buffer from the pool
	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	// Copy the file
	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	return err
}

// GetFile retrieves a cached file for the given key and filename.
// It returns the path to the file, a boolean indicating whether the key was found,
// and an error if one occurred.
func (c *Cache) GetFile(key Key, filename string) (string, bool, error) {
	// Get the cache entry
	result, hit, err := c.Get(key)
	if err != nil {
		return "", false, err
	}
	if !hit {
		return "", false, nil
	}

	if filepath.Base(result.Path) == filepath.Base(filename) {
		return result.Path, true, nil
	}

	return "", false, fmt.Errorf("file %s not found in cache entry", filename)
}

// manifestPath returns the path to the manifest file for the given key hash.
func (c *Cache) manifestPath(keyHash string) string {
	// Use first 2 characters as directory name for better distribution
	prefix := keyHash[:2]
	return filepath.Join(c.manifestDir(), prefix, keyHash+".json")
}

// objectPath returns the path to the object directory for the given key hash.
func (c *Cache) objectPath(keyHash string) string {
	// Use first 2 characters as directory name for better distribution
	prefix := keyHash[:2]
	return filepath.Join(c.objectsDir(), prefix, keyHash)
}

// GetData retrieves cached data for the given key and data key.
// It returns the data, a boolean indicating whether the key was found,
// and an error if one occurred.
func (c *Cache) GetData(key Key, dataKey string) ([]byte, bool, error) {
	// Get the cache entry
	result, hit, err := c.Get(key)
	if err != nil {
		return nil, false, err
	}
	if !hit {
		return nil, false, nil
	}

	// Check if the data is in the metadata
	if value, ok := result.Metadata[dataKey]; ok {
		return []byte(value), true, nil
	}

	return nil, false, fmt.Errorf("data key %s not found in cache entry", dataKey)
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove the cache directories
	if err := c.fs.RemoveAll(c.manifestDir()); err != nil {
		return fmt.Errorf("failed to remove manifests directory: %w", err)
	}
	if err := c.fs.RemoveAll(c.objectsDir()); err != nil {
		return fmt.Errorf("failed to remove objects directory: %w", err)
	}

	// Recreate the directories
	if err := c.fs.MkdirAll(c.manifestDir(), 0755); err != nil {
		return fmt.Errorf("failed to create manifests directory: %w", err)
	}
	if err := c.fs.MkdirAll(c.objectsDir(), 0755); err != nil {
		return fmt.Errorf("failed to create objects directory: %w", err)
	}

	return nil
}

// Remove removes a specific entry from the cache.
func (c *Cache) Remove(keyHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove the manifest file
	manifestFile := c.manifestPath(keyHash)
	exists, err := afero.Exists(c.fs, manifestFile)
	if err != nil {
		return fmt.Errorf("failed to check manifest file existence: %w", err)
	}
	if exists {
		if err := c.fs.Remove(manifestFile); err != nil {
			return fmt.Errorf("failed to remove manifest file: %w", err)
		}
	}

	// Remove the object directory
	objectDir := c.objectPath(keyHash)
	exists, err = afero.Exists(c.fs, objectDir)
	if err != nil {
		return fmt.Errorf("failed to check object directory existence: %w", err)
	}
	if exists {
		if err := c.fs.RemoveAll(objectDir); err != nil {
			return fmt.Errorf("failed to remove object directory: %w", err)
		}
	}

	return nil
}