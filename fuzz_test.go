package granular

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// FuzzManifestJSON fuzzes manifest JSON deserialization
func FuzzManifestJSON(f *testing.F) {
	// Seed corpus with valid JSON examples
	f.Add([]byte(`{
		"keyHash": "abc123",
		"inputs": ["file:test.txt"],
		"extra": {"version": "1.0"},
		"outputs": {"result": "/path/to/file"},
		"outputData": {},
		"outputMeta": {},
		"outputHash": "def456",
		"createdAt": "2024-01-01T12:00:00Z",
		"accessedAt": "2024-01-01T12:00:00Z"
	}`))

	f.Add([]byte(`{}`))
	f.Add([]byte(`{"keyHash": ""}`))
	f.Add([]byte(`{"keyHash": "x"}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`""`))
	f.Add([]byte(``))

	// Fuzz
	f.Fuzz(func(t *testing.T, data []byte) {
		// Create filesystem and cache
		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Skip("Cache creation failed")
		}
		defer cache.Close()

		// Write fuzzed data as manifest
		manifestPath := ".cache/manifests/ab/abc123.json"
		fs.MkdirAll(".cache/manifests/ab", 0o755)
		afero.WriteFile(fs, manifestPath, data, 0o644)

		// Try to load manifest - should never panic
		_, err = cache.loadManifest("abc123")

		// We accept any error (invalid JSON is expected)
		// The important thing is no panic
		_ = err
	})
}

// FuzzManifestFields fuzzes individual manifest fields
func FuzzManifestFields(f *testing.F) {
	// Seed with various field values
	f.Add("keyhash123", "input1", "key1", "value1", "file.txt", "path/to/file", "meta1", "metaval1")

	f.Fuzz(func(t *testing.T, keyHash, input, extraKey, extraVal, outputName, outputPath, metaKey, metaVal string) {
		// Create manifest structure
		m := manifest{
			KeyHash:     keyHash,
			InputDescs:  []string{input},
			ExtraData:   map[string]string{extraKey: extraVal},
			OutputFiles: map[string]string{outputName: outputPath},
			OutputData:  map[string]string{},
			OutputMeta:  map[string]string{metaKey: metaVal},
			OutputHash:  "hash123",
			CreatedAt:   time.Now(),
			AccessedAt:  time.Now(),
		}

		// Try to marshal - should never panic
		data, err := json.Marshal(m)
		if err != nil {
			// JSON marshaling failed, that's ok
			return
		}

		// Try to unmarshal back - should never panic
		var m2 manifest
		_ = json.Unmarshal(data, &m2)

		// Round-trip should preserve non-nil maps
		if len(m2.ExtraData) > 0 {
			if m2.ExtraData[extraKey] != extraVal {
				t.Errorf("Extra data mismatch after round-trip")
			}
		}
	})
}

// FuzzGlobPattern fuzzes glob pattern parsing
func FuzzGlobPattern(f *testing.F) {
	// Seed corpus with valid patterns
	f.Add("*.go")
	f.Add("**/*.go")
	f.Add("src/**/*.txt")
	f.Add("**")
	f.Add("**/")
	f.Add("file?.txt")
	f.Add("src/*/test.go")
	f.Add("")
	f.Add("***")
	f.Add("?")
	f.Add("[")
	f.Add("]")
	f.Add("[!]")
	f.Add("\\")

	// Fuzz
	f.Fuzz(func(t *testing.T, pattern string) {
		// Create test filesystem
		fs := afero.NewMemMapFs()
		fs.MkdirAll("src/pkg", 0o755)
		afero.WriteFile(fs, "src/pkg/file.go", []byte("code"), 0o644)
		afero.WriteFile(fs, "test.txt", []byte("test"), 0o644)

		// Try to expand glob - should never panic
		matches, err := expandGlob(pattern, fs)

		// Any error is acceptable (invalid patterns are expected)
		_ = err
		_ = matches

		// If no error, matches should be valid file paths
		if err == nil {
			for _, match := range matches {
				// Each match should be a valid string (not corrupt)
				if match != "" {
					// Verify it's a reasonable path (no null bytes, etc)
					if strings.Contains(match, "\x00") {
						t.Errorf("Match contains null byte: %q", match)
					}
				}
			}
		}
	})
}

// FuzzGlobMatching fuzzes the matchesGlobPattern function
func FuzzGlobMatching(f *testing.F) {
	// Seed with various path/pattern combinations
	f.Add("src/file.go", "*.go")
	f.Add("src/pkg/file.go", "**/*.go")
	f.Add("a/b/c/file.txt", "a/**/file.txt")
	f.Add("file.go", "*")
	f.Add("", "")
	f.Add("/", "/")

	f.Fuzz(func(t *testing.T, path, pattern string) {
		// Should never panic
		result := matchesGlobPattern(path, pattern)
		_ = result

		// If it matches, basic sanity check
		// (we can't verify correctness without reimplementing the logic,
		// but we can check it doesn't crash)
	})
}

// FuzzGlobParts fuzzes the matchGlobParts recursive function
func FuzzGlobParts(f *testing.F) {
	// Seed with split paths and patterns
	f.Add("src", "pkg", "file.go", "**", "*.go")
	f.Add("a", "b", "c", "**", "")
	f.Add("", "", "", "**", "")

	f.Fuzz(func(t *testing.T, path1, path2, path3, pattern1, pattern2 string) {
		pathParts := []string{}
		if path1 != "" {
			pathParts = append(pathParts, path1)
		}
		if path2 != "" {
			pathParts = append(pathParts, path2)
		}
		if path3 != "" {
			pathParts = append(pathParts, path3)
		}

		patternParts := []string{}
		if pattern1 != "" {
			patternParts = append(patternParts, pattern1)
		}
		if pattern2 != "" {
			patternParts = append(patternParts, pattern2)
		}

		// Should never panic or infinite loop
		result := matchGlobParts(pathParts, patternParts, 0, 0)
		_ = result
	})
}

// FuzzKeyHash fuzzes key hash computation
func FuzzKeyHash(f *testing.F) {
	// Seed with various inputs
	f.Add("file.txt", "content data", "key1", "value1")

	f.Fuzz(func(t *testing.T, filename, content, extraKey, extraValue string) {
		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Skip("Cache creation failed")
		}
		defer cache.Close()

		// Create file if filename is valid
		if filename != "" && !strings.Contains(filename, "\x00") {
			afero.WriteFile(fs, filename, []byte(content), 0o644)

			// Try to build key and compute hash - should never panic
			key := cache.Key().File(filename).String(extraKey, extraValue).Build()
			hash, _ := key.computeHash()
			_ = hash

			// Hash should be consistent
			hash2, _ := key.computeHash()
			if hash != "" && hash2 != "" && hash != hash2 {
				t.Errorf("Hash not deterministic: %s vs %s", hash, hash2)
			}
		}
	})
}

// FuzzCachePutGet fuzzes the Put/Get cycle
func FuzzCachePutGet(f *testing.F) {
	// Seed with various data
	f.Add("test.txt", []byte("input"), []byte("output"), "meta", "value")

	f.Fuzz(func(t *testing.T, filename string, inputData, outputData []byte, metaKey, metaValue string) {
		// Skip invalid filenames
		if filename == "" || strings.Contains(filename, "\x00") || strings.Contains(filename, string([]byte{0xFF})) {
			return
		}

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Skip("Cache creation failed")
		}
		defer cache.Close()

		// Create input file
		if err := afero.WriteFile(fs, filename, inputData, 0o644); err != nil {
			return
		}

		// Build key
		key := cache.Key().File(filename).Build()

		// Put data - should never panic
		err = cache.Put(key).
			Bytes("output", outputData).
			Meta(metaKey, metaValue).
			Commit()
		if err != nil {
			// Errors are acceptable
			return
		}

		// Get data - should never panic
		result, err := cache.Get(key)
		if err != nil {
			t.Errorf("Get failed after successful Put: %v", err)
			return
		}

		// Verify data integrity
		retrievedData := result.Bytes("output")
		if retrievedData == nil {
			t.Error("Output data not found")
			return
		}

		// Data should match what we put
		if len(retrievedData) != len(outputData) {
			t.Errorf("Data length mismatch: got %d, want %d", len(retrievedData), len(outputData))
			return
		}

		for i := range outputData {
			if retrievedData[i] != outputData[i] {
				t.Errorf("Data corruption at byte %d: got %d, want %d", i, retrievedData[i], outputData[i])
				break
			}
		}

		// Metadata should match
		if metaKey != "" {
			retrievedMeta := result.Meta(metaKey)
			if retrievedMeta != metaValue && retrievedMeta != "" {
				t.Errorf("Meta mismatch: got %q, want %q", retrievedMeta, metaValue)
			}
		}
	})
}

// FuzzManifestOutputHash fuzzes output hash computation
func FuzzManifestOutputHash(f *testing.F) {
	// Seed with various outputs
	f.Add("file1.txt", "file2.txt", []byte("data1"), []byte("data2"))

	f.Fuzz(func(t *testing.T, file1, file2 string, data1, data2 []byte) {
		// Skip invalid filenames
		if file1 == "" || file2 == "" || strings.Contains(file1, "\x00") || strings.Contains(file2, "\x00") {
			return
		}

		fs := afero.NewMemMapFs()
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Skip()
		}
		defer cache.Close()

		// Create output files
		afero.WriteFile(fs, file1, data1, 0o644)
		afero.WriteFile(fs, file2, data2, 0o644)

		outputs := []string{file1, file2}
		outputData := map[string][]byte{
			"data1": data1,
			"data2": data2,
		}
		outputMeta := map[string]string{
			"meta1": "value1",
		}

		// Compute hash - should never panic
		hash1, err := cache.computeOutputHash(outputs, outputData, outputMeta)
		if err != nil {
			return
		}

		// Hash should be deterministic
		hash2, err := cache.computeOutputHash(outputs, outputData, outputMeta)
		if err != nil {
			return
		}

		if hash1 != hash2 {
			t.Errorf("Output hash not deterministic: %s vs %s", hash1, hash2)
		}

		// Hash should be non-empty
		if hash1 == "" {
			t.Error("Output hash is empty")
		}
	})
}

// FuzzValidateArchivePath fuzzes the path validation used in Import to find
// path traversal bypasses. validateArchivePath handles attacker-controlled tar paths
// with filepath.IsAbs, filepath.Clean, strings.Contains, filepath.Abs, and
// strings.HasPrefix — all of which have subtle platform behavior.
func FuzzValidateArchivePath(f *testing.F) {
	// Seed corpus: known attacks and edge cases
	f.Add("../../../etc/passwd")
	f.Add("manifests/../../../tmp/evil")
	f.Add("/absolute/path")
	f.Add("normal/path/file.json")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("..\\..\\etc\\passwd")
	f.Add("manifests/ab/abc123.json")
	f.Add("objects/de/deadbeef/file.dat")
	f.Add("a/b/c/d/e/f/g")
	f.Add(strings.Repeat("a", 1000))
	f.Add("valid/../../../escape")
	f.Add("./relative")
	f.Add("manifests/../../outside")

	f.Fuzz(func(t *testing.T, name string) {
		baseDir := "/cache"

		target, err := validateArchivePath(name, baseDir)
		if err != nil {
			// Rejected — this is fine, the function is doing its job
			return
		}

		// If accepted, the resolved path MUST be within baseDir.
		// This is the critical safety invariant.
		if !strings.HasPrefix(target, baseDir+"/") && target != baseDir {
			t.Errorf("validateArchivePath accepted path that escapes base dir:\n  name=%q\n  baseDir=%q\n  target=%q",
				name, baseDir, target)
		}

		// The accepted path must not contain ".."
		if strings.Contains(target, "..") {
			t.Errorf("validateArchivePath accepted path containing '..':\n  name=%q\n  target=%q",
				name, target)
		}
	})
}

// FuzzValidationErrors fuzzes error accumulation
func FuzzValidationErrors(f *testing.F) {
	// Seed with various error messages
	f.Add("error 1", "error 2", "error 3")

	f.Fuzz(func(t *testing.T, err1, err2, err3 string) {
		var errs []error
		if err1 != "" {
			errs = append(errs, errors.New(err1))
		}
		if err2 != "" {
			errs = append(errs, errors.New(err2))
		}
		if err3 != "" {
			errs = append(errs, errors.New(err3))
		}

		// Create validation error - should never panic
		ve := newValidationError(errs)
		if ve == nil && len(errs) > 0 {
			t.Error("newValidationError returned nil for non-empty errors")
			return
		}

		if ve != nil {
			// Should be able to call Error() without panic
			errMsg := ve.Error()
			if errMsg == "" {
				t.Error("Error message is empty")
			}

			// Should be able to unwrap without panic
			validationErr := ve.(*ValidationError)
			unwrapped := validationErr.Unwrap()
			if len(unwrapped) != len(errs) {
				t.Errorf("Unwrap returned %d errors, want %d", len(unwrapped), len(errs))
			}
		}
	})
}
