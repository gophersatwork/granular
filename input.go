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

// FileInput represents a single file input for the cache.
type FileInput struct {
	Path     string // Path to the file
	afero.Fs        // The filesystem to use
}

// Hash implements the Input interface for FileInput.
func (f FileInput) Hash(h hash.Hash) error {
	// If no filesystem is provided, use the OS filesystem
	fs := f.Fs
	if fs == nil {
		fs = afero.NewOsFs()
	}
	b, err := afero.ReadFile(fs, f.Path)
	if err != nil {
		return err
	}
	err = hashFile(bytes.NewReader(b), h)
	if err != nil {
		return err
	}
	return nil
}

// String implements the Input interface for FileInput.
func (f FileInput) String() string {
	return fmt.Sprintf("file:%s", f.Path)
}

// GlobInput represents a glob pattern input for the cache.
type GlobInput struct {
	Pattern string   // Glob pattern
	Fs      afero.Fs // The filesystem to use
}

// Hash implements the Input interface for GlobInput.
// It finds all files matching the pattern, sorts them for determinism.
func (g GlobInput) Hash(h hash.Hash) error {
	// If no filesystem is provided, use the OS filesystem
	fs := g.Fs
	if fs == nil {
		fs = afero.NewOsFs()
	}

	// Handle recursive glob patterns with "**"
	pattern := g.Pattern
	hasRecursiveGlob := strings.Contains(pattern, "**")

	// Get the base directory to start the walk
	baseDir := "."
	if hasRecursiveGlob {
		// For patterns with "**", find the directory part before the first "**"
		parts := strings.Split(pattern, "**")
		baseDir = filepath.Dir(parts[0])
		if baseDir == "." && parts[0] != "" && !strings.HasSuffix(parts[0], "/") && !strings.HasSuffix(parts[0], string(filepath.Separator)) {
			baseDir = parts[0]
		}
	} else {
		// For simple patterns, use the directory part
		baseDir = filepath.Dir(pattern)
	}

	// If baseDir is ".", use empty string to search in current directory
	if baseDir == "." {
		baseDir = ""
	}

	// Check if the base directory exists
	exists, err := afero.DirExists(fs, baseDir)
	if err != nil {
		return err
	}
	if !exists && baseDir != "" {
		// If the directory doesn't exist, return an empty list of matches
		// This is not an error, just no matches
		return nil
	}

	// Find all files matching the pattern
	var matches []string
	err = afero.Walk(fs, baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// For recursive glob patterns, use a custom matching logic
		if hasRecursiveGlob {
			if matchesGlobPattern(path, pattern) {
				matches = append(matches, path)
			}
		} else {
			// For simple patterns, use filepath.Match on the base name
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
	if err != nil {
		return err
	}

	// Sort matches for deterministic ordering
	sort.Strings(matches)

	// Hash the number of matches first
	countStr := fmt.Sprintf("%d", len(matches))
	h.Write([]byte(countStr))

	// Hash each file
	for _, match := range matches {
		// Hash the filename first
		h.Write([]byte(match))

		// Then hash the file content
		b, err := afero.ReadFile(fs, match)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(b)
		if err != nil {
			return err
		}

		if err = hashFile(reader, h); err != nil {
			return err
		}
	}

	return nil
}

// String implements the Input interface for GlobInput.
func (g GlobInput) String() string {
	return fmt.Sprintf("glob:%s", g.Pattern)
}

// DirectoryInput represents a directory input for the cache.
type DirectoryInput struct {
	Path    string   // Path to the directory
	Exclude []string // Patterns to exclude
	Fs      afero.Fs // The filesystem to use
}

// Hash implements the Input interface for DirectoryInput.
func (d DirectoryInput) Hash(h hash.Hash) error {
	// If no filesystem is provided, use the OS filesystem
	fs := d.Fs
	if fs == nil {
		fs = afero.NewOsFs()
	}

	// Collect all files in the directory
	var files []string
	err := afero.Walk(fs, d.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if the file should be excluded
		for _, pattern := range d.Exclude {
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}

	// Sort files for deterministic ordering
	sort.Strings(files)

	// Hash the number of files first
	countStr := fmt.Sprintf("%d", len(files))
	h.Write([]byte(countStr))

	// Hash each file
	for _, file := range files {
		// Hash the filename first
		h.Write([]byte(file))

		// Then hash the file content
		b, err := afero.ReadFile(fs, file)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(b)
		if err = hashFile(reader, h); err != nil {
			return err
		}
	}

	return nil
}

// String implements the Input interface for DirectoryInput.
func (d DirectoryInput) String() string {
	if len(d.Exclude) == 0 {
		return fmt.Sprintf("dir:%s", d.Path)
	}
	return fmt.Sprintf("dir:%s(exclude:%s)", d.Path, strings.Join(d.Exclude, ","))
}

// RawInput represents a raw data input for the cache.
type RawInput struct {
	Data []byte // Raw data
	Name string // Optional name for the input
}

// Hash implements the Input interface for RawInput.
func (r RawInput) Hash(h hash.Hash) error {
	err := hashFile(bytes.NewReader(r.Data), h)
	if err != nil {
		return fmt.Errorf("%s: %w", r.Name, err)
	}
	return nil
}

// String implements the Input interface for RawInput.
func (r RawInput) String() string {
	if r.Name != "" {
		return fmt.Sprintf("raw:%s", r.Name)
	}
	return fmt.Sprintf("raw:%d bytes", len(r.Data))
}

// matchesGlobPattern checks if a path matches a glob pattern that may include "**".
// It handles the special case of "**" which matches any number of directories.
func matchesGlobPattern(path, pattern string) bool {
	// Convert pattern to use forward slashes for consistency
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Split pattern into segments
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchGlobPatternParts(pathParts, patternParts, 0, 0)
}

// matchGlobPatternParts is a recursive helper function for matchesGlobPattern.
// It matches path parts against pattern parts using a recursive approach.
func matchGlobPatternParts(pathParts, patternParts []string, pathIndex, patternIndex int) bool {
	// If we've reached the end of the pattern, the match is successful only if
	// we've also reached the end of the path
	if patternIndex >= len(patternParts) {
		return pathIndex >= len(pathParts)
	}

	// If we've reached the end of the path but not the pattern, the match fails
	// unless the remaining pattern parts are all "**"
	if pathIndex >= len(pathParts) {
		// Check if all remaining pattern parts are "**"
		for i := patternIndex; i < len(patternParts); i++ {
			if patternParts[i] != "**" {
				return false
			}
		}
		return true
	}

	// Get the current pattern part
	patternPart := patternParts[patternIndex]
	pathPart := pathParts[pathIndex]

	// Handle "**" pattern
	if patternPart == "**" {
		// "**" can match zero or more directories
		// Try matching the rest of the pattern with the current path position
		if matchGlobPatternParts(pathParts, patternParts, pathIndex, patternIndex+1) {
			return true
		}
		// Or try matching the current pattern with the next path position
		return matchGlobPatternParts(pathParts, patternParts, pathIndex+1, patternIndex)
	}

	// For normal glob patterns, use filepath.Match
	matched, err := filepath.Match(patternPart, pathPart)
	if err != nil || !matched {
		return false
	}

	// If the current parts match, continue with the next parts
	return matchGlobPatternParts(pathParts, patternParts, pathIndex+1, patternIndex+1)
}
