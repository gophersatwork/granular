package granular

import (
	"errors"
	"testing"

	"github.com/spf13/afero"
)

// TestHashAlgoMismatch tests that cache entries with different hash algorithms are rejected
func TestHashAlgoMismatch(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Create cache with SHA256
	cacheSHA, err := Open(".cache", WithFs(fs), WithSHA256())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Store an entry with SHA256
	keySHA := cacheSHA.Key().String("id", "test").Build()
	err = cacheSHA.Put(keySHA).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify we can retrieve it with SHA256
	result, err := cacheSHA.Get(keySHA)
	if err != nil {
		t.Fatalf("Get with same algo failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result with same algo")
	}
	err = cacheSHA.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Open same cache with xxHash (default) - different key hash means cache miss
	cacheXX, err := Open(".cache", WithFs(fs), WithXXHash())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cacheXX.Close()
	}()

	// Same logical key with different hash function = different key hash = cache miss
	keyXX := cacheXX.Key().String("id", "test").Build()
	_, err = cacheXX.Get(keyXX)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Expected ErrCacheMiss, got: %v", err)
	}
}

// TestHashAlgoMismatchWithSameKeyHash tests the hash algo validation when manifest exists
func TestHashAlgoMismatchWithSameKeyHash(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Create cache with xxHash (default)
	cache1, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	key := cache1.Key().File("test.txt").Build()
	err = cache1.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should work with same cache
	result, err := cache1.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
	err = cache1.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen with same hash algorithm - should work
	cache2, err := Open(".cache", WithFs(fs), WithXXHash())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	key2 := cache2.Key().File("test.txt").Build()
	result, err = cache2.Get(key2)
	if err != nil {
		t.Fatalf("Get with same algo failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result with same algo")
	}
	err = cache2.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestHashAlgoMismatchReturnsSentinel injects a manifest with a different HashAlgo
// than the cache's configured algorithm, triggering the ErrHashAlgoMismatch code path.
// The existing TestHashAlgoMismatch only proves different hash functions produce
// different key hashes (ErrCacheMiss). This test exercises the actual mismatch sentinel
// where the key hash collides but the algorithm field differs.
func TestHashAlgoMismatchReturnsSentinel(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Create cache with default xxHash
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Store an entry normally
	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify it works before tampering
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}

	// Tamper with the manifest: change the HashAlgo field to a different value.
	// The key hash is the same, so the manifest will be found, but the algo check fails.
	keyHash, err := key.computeHash()
	if err != nil {
		t.Fatalf("computeHash failed: %v", err)
	}

	m, err := cache.loadManifest(keyHash)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}

	m.HashAlgo = "sha256" // different from the cache's "xxhash64"
	err = cache.saveManifest(m)
	if err != nil {
		t.Fatalf("saveManifest failed: %v", err)
	}

	// Get() should now return ErrHashAlgoMismatch
	_, err = cache.Get(key)
	if !errors.Is(err, ErrHashAlgoMismatch) {
		t.Fatalf("Expected ErrHashAlgoMismatch, got: %v", err)
	}
}

// TestWithXXHash tests the xxHash convenience option
func TestWithXXHash(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	cache, err := Open(".cache", WithFs(fs), WithXXHash())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cache.Close()
	}()

	if cache.hashAlgoName != "xxhash64" {
		t.Errorf("hashAlgoName = %q, want %q", cache.hashAlgoName, "xxhash64")
	}

	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
}

// TestWithSHA256 tests the SHA256 convenience option
func TestWithSHA256(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	cache, err := Open(".cache", WithFs(fs), WithSHA256())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cache.Close()
	}()

	if cache.hashAlgoName != "sha256" {
		t.Errorf("hashAlgoName = %q, want %q", cache.hashAlgoName, "sha256")
	}

	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result")
	}
}

// TestDefaultHashAlgoName tests that the default hash algorithm name is set correctly
func TestDefaultHashAlgoName(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cache.Close()
	}()

	if cache.hashAlgoName != DefaultHashAlgoName {
		t.Errorf("hashAlgoName = %q, want %q", cache.hashAlgoName, DefaultHashAlgoName)
	}
}

// TestManifestVersionAndHashAlgo tests that manifest stores version and hash algo
func TestManifestVersionAndHashAlgo(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	cache, err := Open(".cache", WithFs(fs), WithSHA256())
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cache.Close()
	}()

	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Load the manifest directly to verify the fields
	keyHash, err := key.computeHash()
	if err != nil {
		t.Fatalf("computeHash failed: %v", err)
	}

	m, err := cache.loadManifest(keyHash)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}

	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if m.HashAlgo != "sha256" {
		t.Errorf("HashAlgo = %q, want %q", m.HashAlgo, "sha256")
	}
}

// TestLegacyManifestBackwardsCompatibility tests that legacy manifests (version 0, no HashAlgo) work
func TestLegacyManifestBackwardsCompatibility(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	if err != nil {
		t.FailNow()
	}

	// Create cache with default hash (xxhash64)
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	key := cache.Key().File("test.txt").Build()
	err = cache.Put(key).Bytes("output", []byte("data")).Commit()
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Simulate a legacy manifest by clearing the HashAlgo field
	keyHash, err := key.computeHash()
	if err != nil {
		t.Fatalf("computeHash failed: %v", err)
	}

	m, err := cache.loadManifest(keyHash)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}

	// Clear HashAlgo to simulate a legacy manifest
	m.Version = 0
	m.HashAlgo = ""
	err = cache.saveManifest(m)
	if err != nil {
		t.Fatalf("saveManifest failed: %v", err)
	}

	err = cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen cache and try to read the entry
	cache2, err := Open(".cache", WithFs(fs))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() {
		_ = cache2.Close()
	}()

	// Should be able to read the legacy entry (assumes xxhash64)
	key2 := cache2.Key().File("test.txt").Build()
	result, err := cache2.Get(key2)
	if err != nil {
		t.Fatalf("Get failed for legacy manifest: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result for legacy manifest")
	}
}
