package granular

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestValidationError_Error tests the Error() method formatting
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected string
	}{
		{
			name:     "empty errors",
			errors:   []error{},
			expected: "validation failed",
		},
		{
			name:     "single error",
			errors:   []error{fmt.Errorf("file not found")},
			expected: "validation failed: file not found",
		},
		{
			name: "multiple errors",
			errors: []error{
				fmt.Errorf("file not found: foo.txt"),
				fmt.Errorf("invalid pattern: **.go"),
			},
			expected: "validation failed with 2 errors:\n  1. file not found: foo.txt\n  2. invalid pattern: **.go\n",
		},
		{
			name: "three errors",
			errors: []error{
				fmt.Errorf("error one"),
				fmt.Errorf("error two"),
				fmt.Errorf("error three"),
			},
			expected: "validation failed with 3 errors:\n  1. error one\n  2. error two\n  3. error three\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := &ValidationError{Errors: tt.errors}
			got := ve.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestValidationError_Unwrap tests the Unwrap() method for Go 1.20+ multi-error support
func TestValidationError_Unwrap(t *testing.T) {
	err1 := fmt.Errorf("error 1")
	err2 := fmt.Errorf("error 2")
	err3 := fmt.Errorf("error 3")

	ve := &ValidationError{
		Errors: []error{err1, err2, err3},
	}

	unwrapped := ve.Unwrap()
	if len(unwrapped) != 3 {
		t.Fatalf("Unwrap() returned %d errors, want 3", len(unwrapped))
	}

	if !errors.Is(err1, unwrapped[0]) {
		t.Errorf("Unwrap()[0] = %v, want %v", unwrapped[0], err1)
	}
	if !errors.Is(err2, unwrapped[1]) {
		t.Errorf("Unwrap()[1] = %v, want %v", unwrapped[1], err2)
	}
	if !errors.Is(err3, unwrapped[2]) {
		t.Errorf("Unwrap()[2] = %v, want %v", unwrapped[2], err3)
	}
}

// TestValidationError_ErrorsIs tests errors.Is() compatibility
func TestValidationError_ErrorsIs(t *testing.T) {
	sentinelErr := errors.New("sentinel error")
	otherErr := errors.New("other error")

	ve := &ValidationError{
		Errors: []error{
			fmt.Errorf("wrapped: %w", sentinelErr),
			otherErr,
		},
	}

	// Should find sentinel error
	if !errors.Is(ve, sentinelErr) {
		t.Error("errors.Is() should find wrapped sentinel error")
	}

	// Should find other error
	if !errors.Is(ve, otherErr) {
		t.Error("errors.Is() should find direct error")
	}

	// Should not find unrelated error
	unrelatedErr := errors.New("unrelated")
	if errors.Is(ve, unrelatedErr) {
		t.Error("errors.Is() should not find unrelated error")
	}
}

// CustomError is a custom error type for testing errors.AsType()
type CustomError struct {
	Code int
	Msg  string
}

func (ce *CustomError) Error() string {
	return fmt.Sprintf("code %d: %s", ce.Code, ce.Msg)
}

// TestValidationError_ErrorsAs tests errors.AsType() compatibility
func TestValidationError_ErrorsAs(t *testing.T) {
	customErr := &CustomError{Code: 404, Msg: "not found"}

	ve := &ValidationError{
		Errors: []error{
			fmt.Errorf("wrapped: %w", customErr),
			errors.New("other error"),
		},
	}

	target, ok := errors.AsType[*CustomError](ve)
	if !ok {
		t.Fatal("errors.AsType should find CustomError")
	}

	if target.Code != 404 {
		t.Errorf("CustomError.Code = %d, want 404", target.Code)
	}
	if target.Msg != "not found" {
		t.Errorf("CustomError.Msg = %q, want %q", target.Msg, "not found")
	}
}

// TestNewValidationError tests the newValidationError constructor
func TestNewValidationError(t *testing.T) {
	t.Run("nil for empty slice", func(t *testing.T) {
		err := newValidationError([]error{})
		if err != nil {
			t.Errorf("newValidationError([]) = %v, want nil", err)
		}
	})

	t.Run("nil for nil slice", func(t *testing.T) {
		err := newValidationError(nil)
		if err != nil {
			t.Errorf("newValidationError(nil) = %v, want nil", err)
		}
	})

	t.Run("returns ValidationError for non-empty slice", func(t *testing.T) {
		errs := []error{fmt.Errorf("test error")}
		err := newValidationError(errs)
		if err == nil {
			t.Fatal("newValidationError should not return nil for non-empty slice")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatal("newValidationError should return *ValidationError")
		}

		if len(ve.Errors) != 1 {
			t.Errorf("ValidationError.Errors length = %d, want 1", len(ve.Errors))
		}
	})
}

// TestKeyBuilder_FileValidation tests File() validation errors
func TestKeyBuilder_FileValidation(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Create a valid file
	err = afero.WriteFile(fs, "exists.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	t.Run("non-existent file error", func(t *testing.T) {
		key := cache.Key().File("missing.txt").Build()
		_, err := key.computeHash()

		if err == nil {
			t.Fatal("Expected validation error for non-existent file")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		if len(ve.Errors) != 1 {
			t.Fatalf("Expected 1 error, got %d", len(ve.Errors))
		}

		errMsg := ve.Errors[0].Error()
		if !strings.Contains(errMsg, "missing.txt") {
			t.Errorf("Error should mention missing.txt, got: %s", errMsg)
		}
	})

	t.Run("valid file no error", func(t *testing.T) {
		key := cache.Key().File("exists.txt").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("Unexpected error for valid file: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty for valid file")
		}
	})
}

// TestKeyBuilder_GlobValidation tests Glob() validation errors
func TestKeyBuilder_GlobValidation(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Create test files
	if err := errors.Join(
		fs.MkdirAll("src", 0o755),
		afero.WriteFile(fs, "src/file1.go", []byte("package main"), 0o644),
		afero.WriteFile(fs, "src/file2.go", []byte("package main"), 0o644),
	); err != nil {
		t.FailNow()
	}
	t.Run("valid glob pattern", func(t *testing.T) {
		key := cache.Key().Glob("src/*.go").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("Unexpected error for valid glob: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty for valid glob")
		}
	})

	t.Run("glob with non-existent directory", func(t *testing.T) {
		// Non-existent directories return empty matches, not an error
		key := cache.Key().Glob("nonexistent/*.go").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("Unexpected error for glob with non-existent dir: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty even for zero matches")
		}
	})
}

// TestKeyBuilder_DirValidation tests Dir() validation errors
func TestKeyBuilder_DirValidation(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func(cache *Cache) {
		err := cache.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}(cache)

	// Create test directory
	if err := errors.Join(
		fs.MkdirAll("src", 0o755),
		afero.WriteFile(fs, "src/file1.go", []byte("package main"), 0o644),
		afero.WriteFile(fs, "src/file2.txt", []byte("text"), 0o644),
	); err != nil {
		t.FailNow()
	}

	t.Run("non-existent directory error", func(t *testing.T) {
		key := cache.Key().Dir("missing").Build()
		_, err := key.computeHash()

		if err == nil {
			t.Fatal("Expected validation error for non-existent directory")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		if len(ve.Errors) != 1 {
			t.Fatalf("Expected 1 error, got %d", len(ve.Errors))
		}

		errMsg := ve.Errors[0].Error()
		if !strings.Contains(errMsg, "missing") {
			t.Errorf("Error should mention missing directory, got: %s", errMsg)
		}
	})

	t.Run("invalid exclude pattern error", func(t *testing.T) {
		// Invalid pattern: unclosed bracket
		key := cache.Key().Dir("src", "[invalid").Build()
		_, err := key.computeHash()

		if err == nil {
			t.Fatal("Expected validation error for invalid exclude pattern")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		if len(ve.Errors) != 1 {
			t.Fatalf("Expected 1 error, got %d", len(ve.Errors))
		}

		errMsg := ve.Errors[0].Error()
		if !strings.Contains(errMsg, "invalid exclude pattern") {
			t.Errorf("Error should mention invalid exclude pattern, got: %s", errMsg)
		}
	})

	t.Run("valid directory with exclude", func(t *testing.T) {
		key := cache.Key().Dir("src", "*.txt").Build()
		hash, err := key.computeHash()
		if err != nil {
			t.Fatalf("Unexpected error for valid directory: %v", err)
		}
		if hash == "" {
			t.Error("Hash should not be empty for valid directory")
		}
	})
}

// TestKeyBuilder_AccumulateErrors tests WithAccumulateErrors option
func TestKeyBuilder_AccumulateErrors(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("fail-fast mode (default)", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer cache.Close()

		// Add multiple invalid inputs
		key := cache.Key().
			File("missing1.txt").
			File("missing2.txt").
			File("missing3.txt").
			Build()

		_, err = key.computeHash()
		if err == nil {
			t.Fatal("Expected validation error")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		// In fail-fast mode, only first error should be present
		// Note: All inputs are still added, but validation stops after first error
		if len(ve.Errors) < 1 {
			t.Errorf("Expected at least 1 error in fail-fast mode, got %d", len(ve.Errors))
		}
	})

	t.Run("accumulate errors mode", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs), WithAccumulateErrors())
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer cache.Close()

		// Add multiple invalid inputs
		key := cache.Key().
			File("missing1.txt").
			File("missing2.txt").
			File("missing3.txt").
			Build()

		_, err = key.computeHash()
		if err == nil {
			t.Fatal("Expected validation error")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		// All errors should be accumulated
		if len(ve.Errors) != 3 {
			t.Errorf("Expected 3 errors in accumulate mode, got %d", len(ve.Errors))
		}

		// Verify error messages
		errStr := ve.Error()
		if !strings.Contains(errStr, "missing1.txt") {
			t.Error("Error should contain missing1.txt")
		}
		if !strings.Contains(errStr, "missing2.txt") {
			t.Error("Error should contain missing2.txt")
		}
		if !strings.Contains(errStr, "missing3.txt") {
			t.Error("Error should contain missing3.txt")
		}
	})

	t.Run("accumulate mixed validation errors", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs), WithAccumulateErrors())
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer cache.Close()

		// Mix different types of validation errors
		key := cache.Key().
			File("missing.txt").             // File error
			Dir("nonexistent").              // Dir error
			Dir("src", "[invalid").          // Exclude pattern error
			Glob("baddir/nonexistent/*.go"). // Glob with non-existent dir (not an error)
			Build()

		_, err = key.computeHash()
		if err == nil {
			t.Fatal("Expected validation errors")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		// Should have errors for: missing file, missing dir, invalid exclude pattern
		if len(ve.Errors) < 3 {
			t.Errorf("Expected at least 3 errors, got %d: %v", len(ve.Errors), ve)
		}
	})
}

// TestKeyBuilder_HashErrorPropagation tests that Hash() returns empty string on validation errors
func TestKeyBuilder_HashErrorPropagation(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	t.Run("KeyBuilder.Hash() returns empty on error", func(t *testing.T) {
		hash := cache.Key().File("missing.txt").Hash()
		if hash != "" {
			t.Errorf("Hash() should return empty string on validation error, got: %s", hash)
		}
	})

	t.Run("Key.Hash() returns empty on error", func(t *testing.T) {
		key := cache.Key().File("missing.txt").Build()
		hash := key.Hash()
		if hash != "" {
			t.Errorf("Hash() should return empty string on validation error, got: %s", hash)
		}
	})

	t.Run("Hash() returns value for valid input", func(t *testing.T) {
		afero.WriteFile(fs, "valid.txt", []byte("content"), 0o644)
		hash := cache.Key().File("valid.txt").Hash()
		if hash == "" {
			t.Error("Hash() should return non-empty string for valid input")
		}
	})
}

// TestKeyBuilder_ValidateMultipleExcludePatterns tests multiple exclude patterns in Dir()
func TestKeyBuilder_ValidateMultipleExcludePatterns(t *testing.T) {
	fs := afero.NewMemMapFs()

	t.Run("accumulate multiple invalid exclude patterns", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs), WithAccumulateErrors())
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer cache.Close()

		fs.MkdirAll("src", 0o755)

		key := cache.Key().Dir("src", "[invalid1", "[invalid2", "*.txt", "[invalid3").Build()
		_, err = key.computeHash()

		if err == nil {
			t.Fatal("Expected validation errors for invalid patterns")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		// Should have 3 errors for the 3 invalid patterns
		if len(ve.Errors) != 3 {
			t.Errorf("Expected 3 errors for invalid patterns, got %d", len(ve.Errors))
		}
	})

	t.Run("fail-fast stops at first invalid exclude pattern", func(t *testing.T) {
		cache, err := Open(".cache", WithFs(fs))
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer cache.Close()

		fs.MkdirAll("src", 0o755)

		key := cache.Key().Dir("src", "[invalid1", "[invalid2", "[invalid3").Build()
		_, err = key.computeHash()

		if err == nil {
			t.Fatal("Expected validation error")
		}

		ve, ok := errors.AsType[*ValidationError](err)
		if !ok {
			t.Fatalf("Expected *ValidationError, got %T", err)
		}

		// In fail-fast mode, should stop at first invalid pattern
		if len(ve.Errors) != 1 {
			t.Errorf("Expected 1 error in fail-fast mode, got %d", len(ve.Errors))
		}
	})
}

// TestWriteBuilder_DoubleCommit tests that calling Commit() twice returns an error.
func TestWriteBuilder_DoubleCommit(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "double").Build()

	wb := cache.Put(key).Bytes("data", []byte("hello"))

	// First commit should succeed
	if err := wb.Commit(); err != nil {
		t.Fatalf("first Commit should succeed: %v", err)
	}

	// Second commit should fail
	err = wb.Commit()
	if err == nil {
		t.Fatal("second Commit should return error")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected 'already used' error, got: %v", err)
	}
}

// TestWriteBuilder_RetryAfterFailure tests that a failed Commit() prevents retry.
// A failed Commit may have performed side effects (eviction, partial writes) that
// make a retry unsafe. The attempted flag ensures one-shot semantics.
func TestWriteBuilder_RetryAfterFailure(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "retry").Build()

	// Reference a source file that doesn't exist — Commit will fail during file copy
	wb := cache.Put(key).File("missing", "/nonexistent/file.txt")

	// First Commit fails (validation error: file doesn't exist)
	if err := wb.Commit(); err == nil {
		t.Fatal("Commit should fail for missing source file")
	}

	// Retry should be rejected even though the first attempt failed
	err = wb.Commit()
	if err == nil {
		t.Fatal("retry Commit should return error")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected 'already used' error, got: %v", err)
	}
}

// TestMaxDataSize tests that decompressed data exceeding maxDataSize is rejected.
func TestMaxDataSize(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs), WithMaxDataSize(50))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "maxdata").Build()

	// Store data larger than the limit (no compression, so decompressed = raw)
	bigData := make([]byte, 100)
	for i := range bigData {
		bigData[i] = byte(i)
	}

	err = cache.Put(key).Bytes("big", bigData).Commit()
	if err != nil {
		t.Fatalf("Put should succeed (limit is on read, not write): %v", err)
	}

	// Reading should fail
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get should succeed: %v", err)
	}

	_, err = result.BytesErr("big")
	if err == nil {
		t.Fatal("expected error for oversized data read")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Fatalf("expected 'exceeds max size' error, got: %v", err)
	}

	// Data within the limit should work
	smallData := []byte("small")
	key2 := cache.Key().String("test", "small").Build()
	err = cache.Put(key2).Bytes("ok", smallData).Commit()
	if err != nil {
		t.Fatalf("Put small data failed: %v", err)
	}

	result2, err := cache.Get(key2)
	if err != nil {
		t.Fatalf("Get small data failed: %v", err)
	}

	got, err := result2.BytesErr("ok")
	if err != nil {
		t.Fatalf("BytesErr should succeed for small data: %v", err)
	}
	if string(got) != "small" {
		t.Fatalf("expected 'small', got %q", got)
	}
}

// TestEmptyKey tests that a key with no inputs produces a validation error.
func TestEmptyKey(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	key := cache.Key().Build()

	// Get should fail with validation error
	_, err = cache.Get(key)
	if err == nil {
		t.Fatal("expected error for empty key")
	}

	_, ok := errors.AsType[*ValidationError](err)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}

	if !strings.Contains(err.Error(), "key has no inputs") {
		t.Fatalf("expected 'key has no inputs' error, got: %v", err)
	}

	// Commit should also fail
	err = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	if err == nil {
		t.Fatal("expected error for empty key on Put")
	}

	if !strings.Contains(err.Error(), "key has no inputs") {
		t.Fatalf("expected 'key has no inputs' error, got: %v", err)
	}
}

// TestWriteBuilder_NameInjection tests that path traversal in File/Bytes names is rejected.
func TestWriteBuilder_NameInjection(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	// Create a valid source file for File() tests
	afero.WriteFile(fs, "/src/test.txt", []byte("hello"), 0o644)

	key := cache.Key().String("test", "value").Build()

	badNames := []string{
		"../escape",
		"foo/bar",
		"foo\\bar",
		"..\\escape",
		"foo\x00bar",
	}

	for _, name := range badNames {
		t.Run("File_"+fmt.Sprintf("%q", name), func(t *testing.T) {
			err := cache.Put(key).File(name, "/src/test.txt").Commit()
			if err == nil {
				t.Fatalf("expected error for name %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "invalid name") {
				t.Fatalf("expected 'invalid name' error, got: %v", err)
			}
		})

		t.Run("Bytes_"+fmt.Sprintf("%q", name), func(t *testing.T) {
			err := cache.Put(key).Bytes(name, []byte("data")).Commit()
			if err == nil {
				t.Fatalf("expected error for name %q, got nil", name)
			}
			if !strings.Contains(err.Error(), "invalid name") {
				t.Fatalf("expected 'invalid name' error, got: %v", err)
			}
		})
	}

	// Empty name should be rejected
	t.Run("File_empty", func(t *testing.T) {
		err := cache.Put(key).File("", "/src/test.txt").Commit()
		if err == nil {
			t.Fatal("expected error for empty name")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("expected 'must not be empty' error, got: %v", err)
		}
	})

	t.Run("Bytes_empty", func(t *testing.T) {
		err := cache.Put(key).Bytes("", []byte("data")).Commit()
		if err == nil {
			t.Fatal("expected error for empty name")
		}
		if !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("expected 'must not be empty' error, got: %v", err)
		}
	})

	// Valid names should still work (including names containing ".." as a substring)
	t.Run("valid names", func(t *testing.T) {
		err := cache.Put(key).
			File("output", "/src/test.txt").
			Bytes("config", []byte("data")).
			Commit()
		if err != nil {
			t.Fatalf("valid names should succeed: %v", err)
		}
	})

	t.Run("valid name with dots", func(t *testing.T) {
		err := cache.Put(key).
			File("version..2", "/src/test.txt").
			Bytes("report..final", []byte("data")).
			Commit()
		if err != nil {
			t.Fatalf("names containing '..' as substring should succeed: %v", err)
		}
	})
}

// TestWriteBuilder_RejectsInvalidUTF8 verifies that Put rejects strings
// containing invalid UTF-8 byte sequences. Manifests are persisted as JSON
// which silently substitutes U+FFFD for invalid bytes; rejecting at the
// API boundary keeps the Put/Get round-trip lossless.
func TestWriteBuilder_RejectsInvalidUTF8(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer cache.Close()

	afero.WriteFile(fs, "/src/test.txt", []byte("hello"), 0o644)

	const invalid = "\xc9" // standalone continuation byte, invalid UTF-8

	t.Run("Meta_value", func(t *testing.T) {
		key := cache.Key().String("v", "1").Build()
		err := cache.Put(key).Bytes("out", []byte("x")).Meta("k", invalid).Commit()
		if err == nil || !strings.Contains(err.Error(), "non-UTF-8") {
			t.Fatalf("expected non-UTF-8 rejection, got: %v", err)
		}
	})

	t.Run("Meta_key", func(t *testing.T) {
		key := cache.Key().String("v", "1").Build()
		err := cache.Put(key).Bytes("out", []byte("x")).Meta(invalid, "v").Commit()
		if err == nil || !strings.Contains(err.Error(), "non-UTF-8") {
			t.Fatalf("expected non-UTF-8 rejection, got: %v", err)
		}
	})

	t.Run("Bytes_name", func(t *testing.T) {
		key := cache.Key().String("v", "1").Build()
		err := cache.Put(key).Bytes(invalid, []byte("x")).Commit()
		if err == nil || !strings.Contains(err.Error(), "non-UTF-8") {
			t.Fatalf("expected non-UTF-8 rejection, got: %v", err)
		}
	})

	t.Run("Key_String_value", func(t *testing.T) {
		key := cache.Key().String("v", invalid).Build()
		err := cache.Put(key).Bytes("out", []byte("x")).Commit()
		if err == nil || !strings.Contains(err.Error(), "non-UTF-8") {
			t.Fatalf("expected non-UTF-8 rejection, got: %v", err)
		}
	})
}
