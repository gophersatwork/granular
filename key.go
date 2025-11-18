package granular

import (
	"bytes"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
)

// KeyBuilder provides a fluent API for building cache keys.
// It validates inputs eagerly and accumulates errors instead of panicking.
// Errors are only surfaced when Get() or Commit() is called.
type KeyBuilder struct {
	cache            *Cache
	inputs           []input
	extras           map[string]string
	errors           []error // Accumulated validation errors
	accumulateErrors bool    // If true, accumulate all errors; if false, fail-fast
}

// Key represents an opaque cache key.
// Users should not construct this directly, use Cache.Key() instead.
type Key struct {
	inputs []input
	extras map[string]string
	cache  *Cache
	errors []error // Validation errors from key building
}

// input is the internal interface for cache inputs.
// This is not exported - users interact via KeyBuilder methods.
type input interface {
	hash(h hash.Hash, fs afero.Fs) error
	String() string
}

// fileInput represents a single file input.
type fileInput struct {
	path string
}

func (f fileInput) hash(h hash.Hash, fs afero.Fs) error {
	data, err := afero.ReadFile(fs, f.path)
	if err != nil {
		return fmt.Errorf("file %s: %w", f.path, err)
	}
	return hashFile(bytes.NewReader(data), h)
}

func (f fileInput) String() string {
	return fmt.Sprintf("file:%s", f.path)
}

// globInput represents a glob pattern input.
type globInput struct {
	pattern string
}

func (g globInput) hash(h hash.Hash, fs afero.Fs) error {
	matches, err := expandGlob(g.pattern, fs)
	if err != nil {
		return fmt.Errorf("glob %s: %w", g.pattern, err)
	}

	// Sort for deterministic ordering
	sort.Strings(matches)

	// Hash count of matches
	fmt.Fprintf(h, "%d", len(matches))

	// Hash each matched file
	for _, match := range matches {
		h.Write([]byte(match))
		data, err := afero.ReadFile(fs, match)
		if err != nil {
			return fmt.Errorf("glob match %s: %w", match, err)
		}
		if err := hashFile(bytes.NewReader(data), h); err != nil {
			return err
		}
	}

	return nil
}

func (g globInput) String() string {
	return fmt.Sprintf("glob:%s", g.pattern)
}

// dirInput represents a directory input.
type dirInput struct {
	path    string
	exclude []string
}

func (d dirInput) hash(h hash.Hash, fs afero.Fs) error {
	var files []string
	err := afero.Walk(fs, d.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check exclusions (basename only)
		for _, pattern := range d.exclude {
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return fmt.Errorf("invalid exclude pattern %s: %w", pattern, err)
			}
			if matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("dir %s: %w", d.path, err)
	}

	// Sort for deterministic ordering
	sort.Strings(files)

	// Hash count of files
	fmt.Fprintf(h, "%d", len(files))

	// Hash each file
	for _, file := range files {
		h.Write([]byte(file))
		data, err := afero.ReadFile(fs, file)
		if err != nil {
			return fmt.Errorf("dir file %s: %w", file, err)
		}
		if err := hashFile(bytes.NewReader(data), h); err != nil {
			return err
		}
	}

	return nil
}

func (d dirInput) String() string {
	if len(d.exclude) == 0 {
		return fmt.Sprintf("dir:%s", d.path)
	}
	return fmt.Sprintf("dir:%s(exclude:%s)", d.path, strings.Join(d.exclude, ","))
}

// bytesInput represents raw byte data input.
type bytesInput struct {
	data []byte
	name string
}

func (b bytesInput) hash(h hash.Hash, fs afero.Fs) error {
	return hashFile(bytes.NewReader(b.data), h)
}

func (b bytesInput) String() string {
	if b.name != "" {
		return fmt.Sprintf("bytes:%s", b.name)
	}
	return fmt.Sprintf("bytes:%d", len(b.data))
}

// stringInput represents a key-value string input.
type stringInput struct {
	key   string
	value string
}

func (s stringInput) hash(h hash.Hash, fs afero.Fs) error {
	h.Write([]byte(s.key))
	h.Write([]byte(s.value))
	return nil
}

func (s stringInput) String() string {
	return fmt.Sprintf("%s=%s", s.key, s.value)
}

// File adds a file input to the cache key.
// Validates that the file exists and accumulates any errors.
// Errors are only surfaced when Get() or Commit() is called.
func (kb *KeyBuilder) File(path string) *KeyBuilder {
	// If fail-fast and already have errors, skip validation
	if !kb.accumulateErrors && len(kb.errors) > 0 {
		kb.inputs = append(kb.inputs, fileInput{path: path})
		return kb
	}

	// Validate file exists
	exists, err := afero.Exists(kb.cache.fs, path)
	if err != nil {
		kb.errors = append(kb.errors, fmt.Errorf("failed to check file %s: %w", path, err))
	} else if !exists {
		kb.errors = append(kb.errors, fmt.Errorf("file does not exist: %s", path))
	}

	kb.inputs = append(kb.inputs, fileInput{path: path})
	return kb
}

// Glob adds a glob pattern input to the cache key.
// Patterns support ** for recursive matching.
// Validates the pattern and accumulates any errors.
// Errors are only surfaced when Get() or Commit() is called.
func (kb *KeyBuilder) Glob(pattern string) *KeyBuilder {
	// If fail-fast and already have errors, skip validation
	if !kb.accumulateErrors && len(kb.errors) > 0 {
		kb.inputs = append(kb.inputs, globInput{pattern: pattern})
		return kb
	}

	// Validate pattern by attempting to expand it
	_, err := expandGlob(pattern, kb.cache.fs)
	if err != nil {
		kb.errors = append(kb.errors, fmt.Errorf("invalid glob pattern %s: %w", pattern, err))
	}

	kb.inputs = append(kb.inputs, globInput{pattern: pattern})
	return kb
}

// Dir adds a directory input to the cache key.
// All files in the directory are included recursively.
// exclude patterns match against basenames only.
// Validates the directory and patterns, accumulating any errors.
// Errors are only surfaced when Get() or Commit() is called.
func (kb *KeyBuilder) Dir(path string, exclude ...string) *KeyBuilder {
	// If fail-fast and already have errors, skip validation
	if !kb.accumulateErrors && len(kb.errors) > 0 {
		kb.inputs = append(kb.inputs, dirInput{path: path, exclude: exclude})
		return kb
	}

	// Validate directory exists
	exists, err := afero.DirExists(kb.cache.fs, path)
	if err != nil {
		kb.errors = append(kb.errors, fmt.Errorf("failed to check directory %s: %w", path, err))
	} else if !exists {
		kb.errors = append(kb.errors, fmt.Errorf("directory does not exist: %s", path))
	}

	// Validate exclude patterns
	for _, pattern := range exclude {
		_, err := filepath.Match(pattern, "test")
		if err != nil {
			kb.errors = append(kb.errors, fmt.Errorf("invalid exclude pattern %s: %w", pattern, err))
			// If fail-fast, stop validating exclude patterns after first error
			if !kb.accumulateErrors {
				break
			}
		}
	}

	kb.inputs = append(kb.inputs, dirInput{path: path, exclude: exclude})
	return kb
}

// Bytes adds raw byte data as an input to the cache key.
// name is optional and used for debugging/logging.
func (kb *KeyBuilder) Bytes(data []byte) *KeyBuilder {
	kb.inputs = append(kb.inputs, bytesInput{data: data, name: ""})
	return kb
}

// String adds a key-value pair to the cache key.
// This is useful for versioning, configuration, or other metadata.
func (kb *KeyBuilder) String(key, value string) *KeyBuilder {
	if kb.extras == nil {
		kb.extras = make(map[string]string)
	}
	kb.extras[key] = value
	return kb
}

// Version is sugar for String("version", v).
func (kb *KeyBuilder) Version(v string) *KeyBuilder {
	return kb.String("version", v)
}

// Env adds an environment variable to the cache key.
// If the variable is not set, it uses an empty string.
func (kb *KeyBuilder) Env(key string) *KeyBuilder {
	return kb.String("env:"+key, os.Getenv(key))
}

// Build finalizes the key builder and returns an opaque Key.
// Validation errors are not returned here but will be surfaced
// when the key is used in Get() or Commit().
func (kb *KeyBuilder) Build() Key {
	return Key{
		inputs: kb.inputs,
		extras: kb.extras,
		cache:  kb.cache,
		errors: kb.errors,
	}
}

// Hash computes and returns the hash of this key as a hex string.
// This is useful for debugging and logging.
// Returns empty string if there are validation errors.
func (kb *KeyBuilder) Hash() string {
	key := kb.Build()
	hash, err := key.computeHash()
	if err != nil {
		return ""
	}
	return hash
}

// Hash returns the hash of this key as a hex string.
// This is useful for debugging and logging.
// Returns empty string if there are validation errors.
func (k Key) Hash() string {
	hash, err := k.computeHash()
	if err != nil {
		return ""
	}
	return hash
}

// computeHash calculates the hash for this key.
// Returns an error if there are validation errors from key building.
func (k Key) computeHash() (string, error) {
	// Check for validation errors first
	if len(k.errors) > 0 {
		return "", newValidationError(k.errors)
	}

	h := k.cache.newHash()

	// Hash all inputs
	for _, input := range k.inputs {
		// Write input string representation for better determinism
		h.Write([]byte(input.String()))
		if err := input.hash(h, k.cache.fs); err != nil {
			return "", err
		}
	}

	// Hash extras in sorted order for determinism
	if len(k.extras) > 0 {
		keys := make([]string, 0, len(k.extras))
		for k := range k.extras {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte(k.extras[key]))
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// expandGlob expands a glob pattern (supporting **) and returns matching file paths.
func expandGlob(pattern string, fs afero.Fs) ([]string, error) {
	hasRecursive := strings.Contains(pattern, "**")

	// Determine base directory
	baseDir := "."
	if hasRecursive {
		parts := strings.Split(pattern, "**")
		baseDir = filepath.Dir(parts[0])
		if baseDir == "." && parts[0] != "" && !strings.HasSuffix(parts[0], "/") && !strings.HasSuffix(parts[0], string(filepath.Separator)) {
			baseDir = parts[0]
		}
	} else {
		baseDir = filepath.Dir(pattern)
	}

	if baseDir == "." {
		baseDir = ""
	}

	// Check if base directory exists
	if baseDir != "" {
		exists, err := afero.DirExists(fs, baseDir)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil // No matches, not an error
		}
	}

	// Walk and match files
	var matches []string
	err := afero.Walk(fs, baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if hasRecursive {
			if matchesGlobPattern(path, pattern) {
				matches = append(matches, path)
			}
		} else {
			filePattern := filepath.Base(pattern)
			matched, err := filepath.Match(filePattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if matched {
				matches = append(matches, path)
			}
		}

		return nil
	})

	return matches, err
}

// matchesGlobPattern checks if a path matches a pattern with ** support.
func matchesGlobPattern(path, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchGlobParts(pathParts, patternParts, 0, 0)
}

// matchGlobParts recursively matches path parts against pattern parts.
func matchGlobParts(pathParts, patternParts []string, pathIdx, patternIdx int) bool {
	if patternIdx >= len(patternParts) {
		return pathIdx >= len(pathParts)
	}

	if pathIdx >= len(pathParts) {
		for i := patternIdx; i < len(patternParts); i++ {
			if patternParts[i] != "**" {
				return false
			}
		}
		return true
	}

	patternPart := patternParts[patternIdx]
	pathPart := pathParts[pathIdx]

	if patternPart == "**" {
		if matchGlobParts(pathParts, patternParts, pathIdx, patternIdx+1) {
			return true
		}
		return matchGlobParts(pathParts, patternParts, pathIdx+1, patternIdx)
	}

	matched, err := filepath.Match(patternPart, pathPart)
	if err != nil || !matched {
		return false
	}

	return matchGlobParts(pathParts, patternParts, pathIdx+1, patternIdx+1)
}
