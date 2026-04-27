package granular

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
)

// Ensure os.FileMode is used (for atomicWriteFile parameter type)
var _ os.FileMode

// suffixCounter is a process-wide counter used in the randomSuffix fallback
// to prevent collisions when crypto/rand is unavailable.
var suffixCounter atomic.Uint64

// randomSuffix generates a random suffix for temporary files.
// Uses crypto/rand for unpredictable suffixes to avoid collisions.
func randomSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp + monotonic counter to prevent collisions
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), suffixCounter.Add(1))
	}
	return hex.EncodeToString(b)
}

// atomicWriteFile writes data to a file atomically using a temp file and rename.
// This ensures that the file is either fully written or not present at all,
// preventing corruption from crashes during write.
func atomicWriteFile(fs afero.Fs, path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp." + randomSuffix()

	// Write to temp file
	if err := afero.WriteFile(fs, tmpPath, data, perm); err != nil {
		// Attempt cleanup on error
		_ = fs.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename to final path
	if err := fs.Rename(tmpPath, path); err != nil {
		// Cleanup temp file on rename failure
		_ = fs.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// manifest represents a cache manifest file (internal use only).
// It contains metadata about a cached computation.
type manifest struct {
	// Manifest metadata
	Version  int    `json:"version"`  // Manifest format version (0 = legacy, 1 = current)
	HashAlgo string `json:"hashAlgo"` // Hash algorithm identifier (e.g., "xxhash64")

	// Key information
	KeyHash    string            `json:"keyHash"` // Hash of the key
	InputDescs []string          `json:"inputs"`  // String descriptions of inputs
	ExtraData  map[string]string `json:"extra"`   // Extra key components

	// Result information (multi-file support)
	OutputFiles map[string]string `json:"outputs"`    // name -> cached file path
	OutputData  map[string]string `json:"outputData"` // name -> path to .dat file
	OutputMeta  map[string]string `json:"outputMeta"` // metadata key-value pairs
	OutputHash  string            `json:"outputHash"` // Hash of outputs
	Compression CompressionType   `json:"compression,omitzero"`

	// Metadata
	CreatedAt  time.Time `json:"createdAt"`  // When the cache entry was created
	AccessedAt time.Time `json:"accessedAt"` // When the cache entry was last accessed
}

// saveManifest saves a manifest to disk using the cache's filesystem.
// Uses atomic write pattern to prevent corruption from crashes during write.
func (c *Cache) saveManifest(m *manifest) error {
	mPath, err := c.manifestPath(m.KeyHash)
	if err != nil {
		return err
	}

	// Create the manifest directory if it doesn't exist
	manifestDir := filepath.Dir(mPath)
	if err := c.fs.MkdirAll(manifestDir, 0o755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Marshal the manifest to JSON
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write atomically using temp file + rename
	if err := atomicWriteFile(c.fs, mPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// loadManifest loads a manifest from disk using the cache's filesystem.
func (c *Cache) loadManifest(keyHash string) (*manifest, error) {
	mPath, err := c.manifestPath(keyHash)
	if err != nil {
		return nil, err
	}

	// Read the manifest file
	data, err := afero.ReadFile(c.fs, mPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Unmarshal the manifest
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &m, nil
}

// computeOutputHash calculates the hash for the outputs using the cache's filesystem.
func (c *Cache) computeOutputHash(outputs []string, outputData map[string][]byte, outputMeta map[string]string) (string, error) {
	h := c.newHash()

	// Hash output files
	// Sort for deterministic ordering
	slices.Sort(outputs)

	// Hash the number of outputs first
	fmt.Fprintf(h, "%d", len(outputs))

	// Hash each output file with length-prefixed path to prevent collisions
	for _, output := range outputs {
		fmt.Fprintf(h, "%d:", len(output))
		h.Write([]byte(output))

		if err := c.hashOutputFile(h, output); err != nil {
			return "", err
		}
	}

	// Hash output data
	// Sort keys for deterministic ordering
	dataKeys := slices.Sorted(maps.Keys(outputData))

	// Hash the number of data entries first
	fmt.Fprintf(h, "%d", len(dataKeys))

	// Hash each data entry with length-prefixed key to prevent collisions
	for _, k := range dataKeys {
		fmt.Fprintf(h, "%d:", len(k))
		h.Write([]byte(k))
		h.Write(outputData[k])
	}

	// Hash output meta
	// Sort keys for deterministic ordering
	metaKeys := slices.Sorted(maps.Keys(outputMeta))

	// Hash the number of meta entries first
	fmt.Fprintf(h, "%d", len(metaKeys))

	// Hash each meta entry with length-prefixed encoding to prevent collisions
	for _, k := range metaKeys {
		fmt.Fprintf(h, "%d:", len(k))
		h.Write([]byte(k))
		fmt.Fprintf(h, "%d:", len(outputMeta[k]))
		h.Write([]byte(outputMeta[k]))
	}

	// Return the hash as a hex string
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashOutputFile hashes a single output file's content into h.
// Properly defers buffer pool return to avoid leaks on error.
func (c *Cache) hashOutputFile(h io.Writer, path string) error {
	file, err := c.fs.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open output file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	if _, err := io.CopyBuffer(h, file, buffer); err != nil {
		return fmt.Errorf("failed to read output file %s: %w", path, err)
	}

	return nil
}

// verifyOutputHash recomputes the output hash from cached files and data,
// then compares it to the stored hash in the manifest.
// Returns ErrCacheCorrupted if the hashes do not match.
func (c *Cache) verifyOutputHash(m *manifest) error {
	// Extract cached file paths from the manifest
	// m.OutputFiles maps logical names to cached file paths
	cachedPaths := slices.Collect(maps.Values(m.OutputFiles))

	// Load data from .dat files for hash verification
	// Read the raw (possibly compressed) data to match what was stored during commit
	outputData := make(map[string][]byte, len(m.OutputData))
	for name, dataPath := range m.OutputData {
		data, err := afero.ReadFile(c.fs, dataPath)
		if err != nil {
			return fmt.Errorf("failed to read data file %s: %w", dataPath, err)
		}
		outputData[name] = data
	}

	// Compute hash from the cached files (raw, possibly compressed) and loaded data
	computedHash, err := c.computeOutputHash(cachedPaths, outputData, m.OutputMeta)
	if err != nil {
		return fmt.Errorf("failed to compute hash for verification: %w", err)
	}

	if computedHash != m.OutputHash {
		return ErrCacheCorrupted
	}

	return nil
}
