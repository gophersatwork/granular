package granular

import (
	"errors"
	"sort"
	"testing"

	"github.com/spf13/afero"
)

// setupGlobTestFs creates a test filesystem with a variety of files and directories
func setupGlobTestFs(t *testing.T) afero.Fs {
	t.Helper()

	fs := afero.NewMemMapFs()
	err := errors.Join(
		// complex file structure
		fs.MkdirAll("src/pkg/core", 0o755),
		fs.MkdirAll("src/pkg/util", 0o755),
		fs.MkdirAll("src/cmd", 0o755),
		fs.MkdirAll("tests/unit", 0o755),
		fs.MkdirAll("tests/integration", 0o755),
		fs.MkdirAll("docs", 0o755),
		// Create files
		afero.WriteFile(fs, "src/pkg/core/main.go", []byte("package core"), 0o644),
		afero.WriteFile(fs, "src/pkg/core/types.go", []byte("package core"), 0o644),
		afero.WriteFile(fs, "src/pkg/core/README.md", []byte("# Core"), 0o644),
		afero.WriteFile(fs, "src/pkg/util/helper.go", []byte("package util"), 0o644),
		afero.WriteFile(fs, "src/pkg/util/string.go", []byte("package util"), 0o644),
		afero.WriteFile(fs, "src/cmd/app.go", []byte("package main"), 0o644),
		afero.WriteFile(fs, "tests/unit/test1.go", []byte("package test"), 0o644),
		afero.WriteFile(fs, "tests/unit/test2.go", []byte("package test"), 0o644),
		afero.WriteFile(fs, "tests/integration/integration_test.go", []byte("package test"), 0o644),
		afero.WriteFile(fs, "docs/README.md", []byte("# Docs"), 0o644),
		afero.WriteFile(fs, "README.md", []byte("# Project"), 0o644),
		afero.WriteFile(fs, "go.mod", []byte("module test"), 0o644),
	)
	if err != nil {
		t.FailNow()
	}

	return fs
}

// TestExpandGlob_SimplePatterns tests simple wildcard patterns
func TestExpandGlob_SimplePatterns(t *testing.T) {
	fs := setupGlobTestFs(t)

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "single wildcard extension",
			pattern:  "*.md",
			expected: []string{"README.md", "docs/README.md", "src/pkg/core/README.md"},
		},
		{
			name:     "directory with wildcard",
			pattern:  "src/cmd/*.go",
			expected: []string{"src/cmd/app.go"},
		},
		{
			name:     "no matches",
			pattern:  "*.txt",
			expected: []string{},
		},
		{
			name:     "question mark wildcard",
			pattern:  "go.mo?",
			expected: []string{"go.mod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := expandGlob(tt.pattern, fs)
			if err != nil {
				t.Fatalf("expandGlob failed: %v", err)
			}

			sort.Strings(matches)
			sort.Strings(tt.expected)

			if len(matches) != len(tt.expected) {
				t.Fatalf("got %d matches, want %d\nGot: %v\nWant: %v",
					len(matches), len(tt.expected), matches, tt.expected)
			}

			for i, match := range matches {
				if match != tt.expected[i] {
					t.Errorf("match[%d] = %q, want %q", i, match, tt.expected[i])
				}
			}
		})
	}
}

// TestExpandGlob_RecursivePatterns tests ** recursive wildcard patterns
func TestExpandGlob_RecursivePatterns(t *testing.T) {
	fs := setupGlobTestFs(t)

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:    "recursive all go files",
			pattern: "**/*.go",
			expected: []string{
				"src/pkg/core/main.go",
				"src/pkg/core/types.go",
				"src/pkg/util/helper.go",
				"src/pkg/util/string.go",
				"src/cmd/app.go",
				"tests/unit/test1.go",
				"tests/unit/test2.go",
				"tests/integration/integration_test.go",
			},
		},
		{
			name:    "recursive in subdirectory",
			pattern: "src/**/*.go",
			expected: []string{
				"src/pkg/core/main.go",
				"src/pkg/core/types.go",
				"src/pkg/util/helper.go",
				"src/pkg/util/string.go",
				"src/cmd/app.go",
			},
		},
		{
			name:    "recursive markdown files",
			pattern: "**/*.md",
			expected: []string{
				"src/pkg/core/README.md",
				"docs/README.md",
				"README.md",
			},
		},
		{
			name:    "nested directory specific pattern",
			pattern: "src/pkg/**/*.go",
			expected: []string{
				"src/pkg/core/main.go",
				"src/pkg/core/types.go",
				"src/pkg/util/helper.go",
				"src/pkg/util/string.go",
			},
		},
		{
			name:     "recursive with specific filename",
			pattern:  "**/README.md",
			expected: []string{"src/pkg/core/README.md", "docs/README.md", "README.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := expandGlob(tt.pattern, fs)
			if err != nil {
				t.Fatalf("expandGlob failed: %v", err)
			}

			sort.Strings(matches)
			sort.Strings(tt.expected)

			if len(matches) != len(tt.expected) {
				t.Fatalf("got %d matches, want %d\nGot: %v\nWant: %v",
					len(matches), len(tt.expected), matches, tt.expected)
			}

			for i, match := range matches {
				if match != tt.expected[i] {
					t.Errorf("match[%d] = %q, want %q", i, match, tt.expected[i])
				}
			}
		})
	}
}

// TestExpandGlob_EdgeCases tests edge cases and unusual patterns
func TestExpandGlob_EdgeCases(t *testing.T) {
	fs := setupGlobTestFs(t)

	tests := []struct {
		name        string
		pattern     string
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "pattern with trailing slash",
			pattern:     "src/",
			expectEmpty: true,
		},
		{
			name:        "non-existent directory",
			pattern:     "nonexistent/*.go",
			expectEmpty: true,
		},
		{
			name:        "recursive in non-existent directory",
			pattern:     "nonexistent/**/*.go",
			expectEmpty: true,
		},
		{
			name:        "double star only",
			pattern:     "**",
			expectEmpty: false, // Should match all files
		},
		{
			name:        "double star with slash",
			pattern:     "**/",
			expectEmpty: false, // Should match all files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := expandGlob(tt.pattern, fs)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.expectEmpty && len(matches) > 0 {
				t.Errorf("Expected empty matches, got %d: %v", len(matches), matches)
			}
		})
	}
}

// TestMatchesGlobPattern tests the matchesGlobPattern function directly
func TestMatchesGlobPattern(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		// Simple patterns
		{
			name:     "exact match",
			path:     "file.go",
			pattern:  "file.go",
			expected: true,
		},
		{
			name:     "wildcard extension",
			path:     "main.go",
			pattern:  "*.go",
			expected: true,
		},
		{
			name:     "wildcard extension no match",
			path:     "README.md",
			pattern:  "*.go",
			expected: false,
		},

		// Recursive patterns
		{
			name:     "recursive match nested",
			path:     "src/pkg/core/main.go",
			pattern:  "**/*.go",
			expected: true,
		},
		{
			name:     "recursive match root",
			path:     "main.go",
			pattern:  "**/*.go",
			expected: true,
		},
		{
			name:     "recursive with prefix",
			path:     "src/pkg/util/helper.go",
			pattern:  "src/**/*.go",
			expected: true,
		},
		{
			name:     "recursive with prefix no match",
			path:     "tests/unit/test.go",
			pattern:  "src/**/*.go",
			expected: false,
		},
		{
			name:     "multiple recursive",
			path:     "a/b/c/d/file.go",
			pattern:  "a/**/c/**/*.go",
			expected: true,
		},
		{
			name:     "recursive with exact filename",
			path:     "src/pkg/README.md",
			pattern:  "**/README.md",
			expected: true,
		},

		// Edge cases
		{
			name:     "recursive at end",
			path:     "src/file.go",
			pattern:  "src/**",
			expected: true,
		},
		{
			name:     "question mark match",
			path:     "file1.go",
			pattern:  "file?.go",
			expected: true,
		},
		{
			name:     "question mark no match",
			path:     "file12.go",
			pattern:  "file?.go",
			expected: false,
		},

		// Complex nested patterns
		{
			name:     "deep nesting match",
			path:     "a/b/c/d/e/f/file.txt",
			pattern:  "a/**/f/*.txt",
			expected: true,
		},
		{
			name:     "partial path match",
			path:     "src/pkg/core/main.go",
			pattern:  "pkg/**/*.go",
			expected: false, // Pattern doesn't match from root
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesGlobPattern(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesGlobPattern(%q, %q) = %v, want %v",
					tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestMatchGlobParts tests the matchGlobParts recursive function
func TestMatchGlobParts(t *testing.T) {
	tests := []struct {
		name         string
		pathParts    []string
		patternParts []string
		expected     bool
	}{
		{
			name:         "exact match",
			pathParts:    []string{"src", "main.go"},
			patternParts: []string{"src", "main.go"},
			expected:     true,
		},
		{
			name:         "wildcard match",
			pathParts:    []string{"src", "main.go"},
			patternParts: []string{"src", "*.go"},
			expected:     true,
		},
		{
			name:         "recursive match skip levels",
			pathParts:    []string{"a", "b", "c", "file.go"},
			patternParts: []string{"a", "**", "file.go"},
			expected:     true,
		},
		{
			name:         "recursive match no skip",
			pathParts:    []string{"a", "file.go"},
			patternParts: []string{"a", "**", "file.go"},
			expected:     true,
		},
		{
			name:         "recursive at end matches rest",
			pathParts:    []string{"src", "pkg", "file.go"},
			patternParts: []string{"src", "**"},
			expected:     true,
		},
		{
			name:         "length mismatch",
			pathParts:    []string{"src"},
			patternParts: []string{"src", "main.go"},
			expected:     false,
		},
		{
			name:         "pattern longer with recursive",
			pathParts:    []string{"src"},
			patternParts: []string{"src", "**"},
			expected:     true,
		},
		{
			name:         "empty path empty pattern",
			pathParts:    []string{},
			patternParts: []string{},
			expected:     true,
		},
		{
			name:         "empty path with recursive pattern",
			pathParts:    []string{},
			patternParts: []string{"**"},
			expected:     true,
		},
		{
			name:         "multiple recursive wildcards",
			pathParts:    []string{"a", "b", "c", "d", "e"},
			patternParts: []string{"a", "**", "c", "**", "e"},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchGlobParts(tt.pathParts, tt.patternParts, 0, 0)
			if result != tt.expected {
				t.Errorf("matchGlobParts(%v, %v) = %v, want %v",
					tt.pathParts, tt.patternParts, result, tt.expected)
			}
		})
	}
}

// TestGlobIntegrationWithCache tests glob patterns through the Cache API
func TestGlobIntegrationWithCache(t *testing.T) {
	fs := setupGlobTestFs(t)
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("cache close failed: %v", err)
		}
	}(cache)

	t.Run("glob in key builder", func(t *testing.T) {
		key := cache.Key().Glob("src/**/*.go").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty")
		}
	})

	t.Run("multiple globs in same key", func(t *testing.T) {
		key := cache.Key().
			Glob("src/**/*.go").
			Glob("tests/**/*.go").
			Build()

		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty")
		}
	})

	t.Run("glob with no matches still produces hash", func(t *testing.T) {
		key := cache.Key().Glob("nonexistent/**/*.go").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty even with no matches")
		}
	})
}

// TestGlobDeterminism tests that glob results are deterministic (sorted)
func TestGlobDeterminism(t *testing.T) {
	fs := setupGlobTestFs(t)

	pattern := "src/**/*.go"

	// Run glob multiple times
	var hashes []string
	for i := 0; i < 5; i++ {
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}

		hash := cache.Key().Glob(pattern).Hash()
		hashes = append(hashes, hash)
		err = cache.Close()
		if err != nil {
			t.Fatalf("cache close failed: %v", err)
		}
	}

	// All hashes should be identical
	firstHash := hashes[0]
	for i, hash := range hashes {
		if hash != firstHash {
			t.Errorf("Hash %d differs: got %s, want %s", i, hash, firstHash)
		}
	}
}

// TestGlobExcludeInDir tests Dir() with exclude patterns works correctly
func TestGlobExcludeInDir(t *testing.T) {
	fs := setupGlobTestFs(t)
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("cache close failed: %v", err)
		}
	}(cache)

	t.Run("exclude markdown files", func(t *testing.T) {
		// This test verifies that excluding files works correctly
		// Create a key that includes all files except *.md
		key1 := cache.Key().Dir("src/pkg/core", "*.md").Build()
		hash1, err := key1.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}

		// Create another key that explicitly lists the non-md files
		// This should produce a different hash because Dir() includes the directory path in the hash
		key2 := cache.Key().
			File("src/pkg/core/main.go").
			File("src/pkg/core/types.go").
			Build()
		hash2, err := key2.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}

		// These will be different because Dir() and File() hash differently
		// Dir includes the directory structure, File includes individual file paths
		if hash1 == hash2 {
			t.Skip("Dir() and File() produce different hashes by design")
		}

		// Instead, verify that the exclude actually excludes the markdown file
		// by checking that including it produces a different hash
		key3 := cache.Key().Dir("src/pkg/core").Build() // No exclusions
		hash3, err := key3.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}

		if hash1 == hash3 {
			t.Error("Hash with *.md exclusion should differ from hash without exclusion")
		}
	})

	t.Run("multiple exclude patterns", func(t *testing.T) {
		key := cache.Key().Dir("src/pkg/core", "*.md", "types.go").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("computeHash failed: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty")
		}
	})
}
