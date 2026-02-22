package granular

import (
	"archive/tar"
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportCreatesValidTar(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "export-test")

	// Create test file and cache entry
	inputFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, inputFile, []byte("input data"))

	key := cache.Key().File(inputFile).String("version", "1").Build()

	outputFile := filepath.Join(tempDir, "output.txt")
	outputContent := []byte("cached output")
	createTestFile(t, memFs, outputFile, outputContent)

	err := cache.Put(key).
		File("result", outputFile).
		Meta("key", "value").
		Commit()
	assertNoError(t, err, "Put")

	// Export to buffer
	var buf bytes.Buffer
	err = cache.Export(&buf)
	assertNoError(t, err, "Export")

	// Verify tar is valid and contains expected files
	tr := tar.NewReader(&buf)
	foundFiles := make(map[string]bool)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		assertNoError(t, err, "reading tar header")
		foundFiles[header.Name] = true
	}

	// Should have manifests and objects directories
	if len(foundFiles) == 0 {
		t.Fatal("Expected non-empty tar archive")
	}

	// Verify we have manifest and object files
	hasManifest := false
	hasObject := false
	for name := range foundFiles {
		if strings.HasPrefix(filepath.ToSlash(name), "manifests") {
			hasManifest = true
		}
		if strings.HasPrefix(filepath.ToSlash(name), "objects") {
			hasObject = true
		}
	}

	if !hasManifest {
		t.Fatal("Expected tar to contain manifests")
	}
	if !hasObject {
		t.Fatal("Expected tar to contain objects")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "roundtrip-test")

	// Create multiple cache entries
	for i := 1; i <= 3; i++ {
		inputFile := filepath.Join(tempDir, "input.txt")
		createTestFile(t, memFs, inputFile, []byte("shared input"))

		key := cache.Key().
			File(inputFile).
			String("index", string(rune('0'+i))).
			Build()

		outputFile := filepath.Join(tempDir, "out.txt")
		createTestFile(t, memFs, outputFile, []byte("output "+string(rune('0'+i))))

		err := cache.Put(key).
			File("out", outputFile).
			Bytes("data", []byte("binary "+string(rune('0'+i)))).
			Meta("index", string(rune('0'+i))).
			Commit()
		assertNoError(t, err, "Put entry")
	}

	// Get stats before export
	statsBefore, err := cache.Stats()
	assertNoError(t, err, "Stats before export")

	// Export
	var buf bytes.Buffer
	err = cache.Export(&buf)
	assertNoError(t, err, "Export")

	// Clear the cache
	err = cache.Clear()
	assertNoError(t, err, "Clear")

	// Verify cache is empty
	statsAfterClear, err := cache.Stats()
	assertNoError(t, err, "Stats after clear")
	if statsAfterClear.Entries != 0 {
		t.Fatalf("Expected 0 entries after clear, got %d", statsAfterClear.Entries)
	}

	// Import back
	err = cache.Import(&buf)
	assertNoError(t, err, "Import")

	// Get stats after import
	statsAfter, err := cache.Stats()
	assertNoError(t, err, "Stats after import")

	// Verify entry count matches
	if statsBefore.Entries != statsAfter.Entries {
		t.Fatalf("Entry count mismatch: before=%d, after=%d",
			statsBefore.Entries, statsAfter.Entries)
	}

	// Verify all entries are accessible
	for i := 1; i <= 3; i++ {
		inputFile := filepath.Join(tempDir, "input.txt")
		createTestFile(t, memFs, inputFile, []byte("shared input"))

		key := cache.Key().
			File(inputFile).
			String("index", string(rune('0'+i))).
			Build()

		result, err := cache.Get(key)
		assertCacheHit(t, result, err, "Get after round-trip")

		if result.Meta("index") != string(rune('0'+i)) {
			t.Fatalf("Expected index '%s', got '%s'",
				string(rune('0'+i)), result.Meta("index"))
		}

		// Verify file is accessible
		if !result.HasFile("out") {
			t.Fatal("Expected 'out' file after round-trip")
		}

		// Verify bytes data is accessible
		if !result.HasData("data") {
			t.Fatal("Expected 'data' bytes after round-trip")
		}

		expectedData := []byte("binary " + string(rune('0'+i)))
		actualData := result.Bytes("data")
		if !bytes.Equal(actualData, expectedData) {
			t.Fatalf("Data mismatch: expected %q, got %q", expectedData, actualData)
		}
	}
}

func TestImportRejectsPathTraversal(t *testing.T) {
	cache, _, _ := setupTestCache(t, "traversal-test")

	// Create a malicious tar with path traversal
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add a file with path traversal attempt
	maliciousPath := "../../../etc/passwd"
	header := &tar.Header{
		Name: maliciousPath,
		Mode: 0o644,
		Size: int64(len("malicious content")),
	}
	err := tw.WriteHeader(header)
	assertNoError(t, err, "WriteHeader for malicious file")

	_, err = tw.Write([]byte("malicious content"))
	assertNoError(t, err, "Write malicious content")

	err = tw.Close()
	assertNoError(t, err, "Close tar writer")

	// Import should reject the malicious tar
	err = cache.Import(&buf)
	if err == nil {
		t.Fatal("Expected Import to reject path traversal attack")
	}

	// Verify error message mentions the path traversal
	if !bytes.Contains([]byte(err.Error()), []byte("path traversal")) {
		t.Fatalf("Expected error about path traversal, got: %v", err)
	}
}

func TestImportRejectsHiddenPathTraversal(t *testing.T) {
	cache, _, _ := setupTestCache(t, "hidden-traversal-test")

	// Create a tar with a more subtle path traversal
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Path that looks innocent but contains traversal
	maliciousPath := "manifests/../../../tmp/evil"
	header := &tar.Header{
		Name: maliciousPath,
		Mode: 0o644,
		Size: int64(len("evil")),
	}
	err := tw.WriteHeader(header)
	assertNoError(t, err, "WriteHeader")

	_, err = tw.Write([]byte("evil"))
	assertNoError(t, err, "Write")

	err = tw.Close()
	assertNoError(t, err, "Close")

	// Import should reject
	err = cache.Import(&buf)
	if err == nil {
		t.Fatal("Expected Import to reject hidden path traversal")
	}
}

func TestExportEmptyCache(t *testing.T) {
	cache, _, _ := setupTestCache(t, "export-empty-test")

	// Export empty cache
	var buf bytes.Buffer
	err := cache.Export(&buf)
	assertNoError(t, err, "Export empty cache")

	// Verify tar is valid (should have directory entries at minimum)
	tr := tar.NewReader(&buf)
	entryCount := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		assertNoError(t, err, "reading tar entry")
		entryCount++
	}

	// Should have at least the manifests and objects directory entries
	if entryCount < 2 {
		t.Fatalf("Expected at least 2 directory entries, got %d", entryCount)
	}
}

func TestExportImportPreservesMetadata(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "metadata-test")

	// Create cache entry with rich metadata
	inputFile := filepath.Join(tempDir, "input.txt")
	createTestFile(t, memFs, inputFile, []byte("test input"))

	key := cache.Key().File(inputFile).Build()

	outputFile := filepath.Join(tempDir, "output.bin")
	createTestFile(t, memFs, outputFile, []byte("binary output"))

	err := cache.Put(key).
		File("binary", outputFile).
		Bytes("json", []byte(`{"key": "value"}`)).
		Meta("version", "1.0.0").
		Meta("author", "test").
		Commit()
	assertNoError(t, err, "Put")

	// Export
	var buf bytes.Buffer
	err = cache.Export(&buf)
	assertNoError(t, err, "Export")

	// Clear and import
	err = cache.Clear()
	assertNoError(t, err, "Clear")

	err = cache.Import(&buf)
	assertNoError(t, err, "Import")

	// Recreate the key and verify metadata
	createTestFile(t, memFs, inputFile, []byte("test input"))
	key = cache.Key().File(inputFile).Build()

	result, err := cache.Get(key)
	assertCacheHit(t, result, err, "Get after import")

	// Verify all metadata
	if result.Meta("version") != "1.0.0" {
		t.Fatalf("Expected version '1.0.0', got '%s'", result.Meta("version"))
	}
	if result.Meta("author") != "test" {
		t.Fatalf("Expected author 'test', got '%s'", result.Meta("author"))
	}

	// Verify bytes data
	jsonData := result.Bytes("json")
	if string(jsonData) != `{"key": "value"}` {
		t.Fatalf("Expected JSON data, got '%s'", string(jsonData))
	}

	// Verify file exists
	if !result.HasFile("binary") {
		t.Fatal("Expected 'binary' file after import")
	}
}

func TestImportIntoNonEmptyCache(t *testing.T) {
	cache, memFs, tempDir := setupTestCache(t, "non-empty-test")

	// Create initial entry
	inputFile1 := filepath.Join(tempDir, "input1.txt")
	createTestFile(t, memFs, inputFile1, []byte("input 1"))

	key1 := cache.Key().File(inputFile1).Build()

	outputFile1 := filepath.Join(tempDir, "output1.txt")
	createTestFile(t, memFs, outputFile1, []byte("output 1"))

	err := cache.Put(key1).
		File("out", outputFile1).
		Meta("source", "original").
		Commit()
	assertNoError(t, err, "Put original")

	// Create second entry and export it
	inputFile2 := filepath.Join(tempDir, "input2.txt")
	createTestFile(t, memFs, inputFile2, []byte("input 2"))

	key2 := cache.Key().File(inputFile2).Build()

	outputFile2 := filepath.Join(tempDir, "output2.txt")
	createTestFile(t, memFs, outputFile2, []byte("output 2"))

	err = cache.Put(key2).
		File("out", outputFile2).
		Meta("source", "to-export").
		Commit()
	assertNoError(t, err, "Put to export")

	// Export only the second entry (by exporting the whole cache)
	var buf bytes.Buffer
	err = cache.Export(&buf)
	assertNoError(t, err, "Export")

	// Delete the second entry
	err = cache.Delete(key2)
	assertNoError(t, err, "Delete")

	// Verify only first entry remains
	stats, err := cache.Stats()
	assertNoError(t, err, "Stats after delete")
	if stats.Entries != 1 {
		t.Fatalf("Expected 1 entry after delete, got %d", stats.Entries)
	}

	// Import (should restore the second entry)
	err = cache.Import(&buf)
	assertNoError(t, err, "Import")

	// Verify both entries exist
	stats, err = cache.Stats()
	assertNoError(t, err, "Stats after import")
	if stats.Entries != 2 {
		t.Fatalf("Expected 2 entries after import, got %d", stats.Entries)
	}

	// Verify both entries are accessible
	result1, err := cache.Get(key1)
	assertCacheHit(t, result1, err, "Get original after import")
	if result1.Meta("source") != "original" {
		t.Fatalf("Expected original source 'original', got '%s'", result1.Meta("source"))
	}

	result2, err := cache.Get(key2)
	assertCacheHit(t, result2, err, "Get imported after import")
	if result2.Meta("source") != "to-export" {
		t.Fatalf("Expected imported source 'to-export', got '%s'", result2.Meta("source"))
	}
}
