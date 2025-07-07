package granular

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/afero"
)

// Manifest represents a cache manifest file.
// It contains metadata about a cached computation.
type Manifest struct {
	// Key information
	KeyHash    string            `json:"keyHash"` // Hash of the key
	InputDescs []string          `json:"inputs"`  // String descriptions of inputs
	ExtraData  map[string]string `json:"extra"`   // Extra key components

	// Result information
	OutputFiles []string          `json:"outputs"`    // Paths to output files
	OutputData  map[string][]byte `json:"-"`          // Raw output data
	OutputMeta  map[string]string `json:"outputMeta"` // String metadata (stored in JSON)
	OutputHash  string            `json:"outputHash"` // Hash of outputs

	// Metadata
	CreatedAt   time.Time `json:"createdAt"`   // When the cache entry was created
	AccessedAt  time.Time `json:"accessedAt"`  // When the cache entry was last accessed
	Description string    `json:"description"` // Optional description
}

// computeKeyHash calculates the hash for a given key using the Cache's hashing methods.
func (c *Cache) computeKeyHash(key Key) (string, error) {
	// Reset the hash to its initial state
	c.hash.Reset()
	// Hash all inputs
	for _, input := range key.Inputs {
		// Write the input type first
		c.hash.Write([]byte(input.String()))

		// Then hash the input content using the Cache's hashInput method
		if err := c.hashInput(input); err != nil {
			return "", fmt.Errorf("failed to hash input %s: %w", input.String(), err)
		}
	}

	// Hash extra data
	// Sort keys for deterministic ordering
	extraKeys := make([]string, 0, len(key.Extra))
	for k := range key.Extra {
		extraKeys = append(extraKeys, k)
	}
	sortStrings(extraKeys)

	// Hash each key-value pair
	for _, k := range extraKeys {
		c.hash.Write([]byte(k))
		c.hash.Write([]byte(key.Extra[k]))
	}

	// Return the hash as a hex string
	return hex.EncodeToString(c.hash.Sum(nil)), nil
}

// saveManifest saves a manifest to disk using the cache's filesystem.
func (c *Cache) saveManifest(manifest *Manifest) error {
	// Create the manifest directory if it doesn't exist
	manifestDir := filepath.Dir(c.manifestPath(manifest.KeyHash))
	if err := c.fs.MkdirAll(manifestDir, 0o755); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	// Marshal the manifest to JSON
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write the manifest file
	if err := afero.WriteFile(c.fs, c.manifestPath(manifest.KeyHash), data, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// loadManifest loads a manifest from disk using the cache's filesystem.
func (c *Cache) loadManifest(keyHash string) (*Manifest, error) {
	// Read the manifest file
	data, err := afero.ReadFile(c.fs, c.manifestPath(keyHash))
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Unmarshal the manifest
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	// Update access time
	manifest.AccessedAt = c.now()

	// Save the updated manifest
	if err := c.saveManifest(&manifest); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: failed to update manifest access time: %v\n", err)
	}

	return &manifest, nil
}

// computeOutputHash calculates the hash for the outputs using the cache's filesystem.
func (c *Cache) computeOutputHash(outputs []string, outputData map[string][]byte, outputMeta map[string]string) (string, error) {
	// Reset the hash to its initial state
	c.hash.Reset()

	// Hash output files
	// Sort for deterministic ordering
	sortStrings(outputs)

	// Hash the number of outputs first
	c.hash.Write([]byte(fmt.Sprintf("%d", len(outputs))))

	// Hash each output file
	for _, output := range outputs {
		// Hash the filename first
		c.hash.Write([]byte(output))

		// Then hash the file content
		// Open the file
		file, err := c.fs.Open(output)
		if err != nil {
			return "", fmt.Errorf("failed to open output file %s: %w", output, err)
		}

		// Get a buffer from the pool
		bufPtr := bufferPool.Get().(*[]byte)
		buffer := *bufPtr

		// Hash the file content
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				return "", fmt.Errorf("failed to read output file %s: %w", output, err)
			}
			if n > 0 {
				c.hash.Write(buffer[:n])
			}
			if err == io.EOF {
				break
			}
		}

		bufferPool.Put(bufPtr)
		_ = file.Close()
	}

	// Hash output data
	// Sort keys for deterministic ordering
	dataKeys := make([]string, 0, len(outputData))
	for k := range outputData {
		dataKeys = append(dataKeys, k)
	}
	sortStrings(dataKeys)

	// Hash the number of data entries first
	c.hash.Write([]byte(fmt.Sprintf("%d", len(dataKeys))))

	// Hash each data entry
	for _, k := range dataKeys {
		// Hash the key first
		c.hash.Write([]byte(k))

		// Then hash the data
		c.hash.Write(outputData[k])
	}

	// Hash output meta
	// Sort keys for deterministic ordering
	metaKeys := make([]string, 0, len(outputMeta))
	for k := range outputMeta {
		metaKeys = append(metaKeys, k)
	}
	sortStrings(metaKeys)

	// Hash the number of meta entries first
	c.hash.Write([]byte(fmt.Sprintf("%d", len(metaKeys))))

	// Hash each meta entry
	for _, k := range metaKeys {
		// Hash the key first
		c.hash.Write([]byte(k))

		// Then hash the value
		c.hash.Write([]byte(outputMeta[k]))
	}

	// Return the hash as a hex string
	return hex.EncodeToString(c.hash.Sum(nil)), nil
}

// sortStrings sorts a slice of strings in place.
// This is a helper function to avoid importing sort in multiple places.
func sortStrings(s []string) {
	sort.Strings(s)
}
