package granular

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
)

// setupBenchCache creates a cache for benchmarking
func setupBenchCache(b *testing.B) (*Cache, afero.Fs) {
	fs := afero.NewMemMapFs()
	cache, err := Open(".cache", WithFs(fs))
	if err != nil {
		b.Fatalf("Open failed: %v", err)
	}
	return cache, fs
}

// BenchmarkCacheGet_Hit benchmarks cache hits
func BenchmarkCacheGet_Hit(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create test file
	afero.WriteFile(fs, "test.txt", []byte("test content"), 0o644)

	// Pre-populate cache
	key := cache.Key().File("test.txt").Build()
	cache.Put(key).Bytes("output", []byte("cached data")).Commit()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.Get(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCacheGet_Miss benchmarks cache misses
func BenchmarkCacheGet_Miss(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "test.txt", []byte("test content"), 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := cache.Key().File("test.txt").String("iter", fmt.Sprintf("%d", i)).Build()
		_, err := cache.Get(key)
		if err != ErrCacheMiss {
			b.Fatalf("Expected cache miss, got: %v", err)
		}
	}
}

// BenchmarkCachePut_SingleFile benchmarks putting a single small file
func BenchmarkCachePut_SingleFile(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create test file (1KB)
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	afero.WriteFile(fs, "input.txt", content, 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := cache.Key().File("input.txt").String("iter", fmt.Sprintf("%d", i)).Build()
		err := cache.Put(key).Bytes("output", []byte("result")).Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCachePut_MultiFile benchmarks putting multiple files
func BenchmarkCachePut_MultiFile(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 10 test files
	for i := 0; i < 10; i++ {
		content := make([]byte, 512)
		for j := range content {
			content[j] = byte((i + j) % 256)
		}
		afero.WriteFile(fs, fmt.Sprintf("file%d.txt", i), content, 0o644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := cache.Key().
			File("file0.txt").
			File("file1.txt").
			File("file2.txt").
			File("file3.txt").
			File("file4.txt").
			String("iter", fmt.Sprintf("%d", i)).
			Build()

		wb := cache.Put(key)
		for j := 0; j < 5; j++ {
			wb = wb.Bytes(fmt.Sprintf("out%d", j), []byte(fmt.Sprintf("data%d", j)))
		}
		err := wb.Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCachePut_LargeFile benchmarks putting a large file (1MB)
func BenchmarkCachePut_LargeFile(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 1MB file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	afero.WriteFile(fs, "large.bin", content, 0o644)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := cache.Key().File("large.bin").String("iter", fmt.Sprintf("%d", i)).Build()
		err := cache.Put(key).Bytes("output", []byte("done")).Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyHash_SingleFile benchmarks key hash computation for single file
func BenchmarkKeyHash_SingleFile(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	content := make([]byte, 1024)
	afero.WriteFile(fs, "test.txt", content, 0o644)

	key := cache.Key().File("test.txt").Build()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := key.computeHash()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyHash_Glob10Files benchmarks key hash for glob with 10 matches
func BenchmarkKeyHash_Glob10Files(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 10 files
	fs.MkdirAll("src", 0o755)
	for i := 0; i < 10; i++ {
		content := make([]byte, 512)
		for j := range content {
			content[j] = byte((i + j) % 256)
		}
		afero.WriteFile(fs, fmt.Sprintf("src/file%d.go", i), content, 0o644)
	}

	key := cache.Key().Glob("src/*.go").Build()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := key.computeHash()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyHash_Glob100Files benchmarks key hash for glob with 100 matches
func BenchmarkKeyHash_Glob100Files(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 100 files in nested directories
	for i := 0; i < 10; i++ {
		dir := fmt.Sprintf("pkg%d", i)
		fs.MkdirAll(dir, 0o755)
		for j := 0; j < 10; j++ {
			content := make([]byte, 256)
			for k := range content {
				content[k] = byte((i + j + k) % 256)
			}
			afero.WriteFile(fs, fmt.Sprintf("%s/file%d.go", dir, j), content, 0o644)
		}
	}

	key := cache.Key().Glob("**/*.go").Build()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := key.computeHash()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyHash_LargeFile benchmarks key hash for single large file (10MB)
func BenchmarkKeyHash_LargeFile(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 10MB file
	content := make([]byte, 10*1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	afero.WriteFile(fs, "large.bin", content, 0o644)

	key := cache.Key().File("large.bin").Build()

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	for i := 0; i < b.N; i++ {
		_, err := key.computeHash()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentReads benchmarks concurrent cache reads
func BenchmarkConcurrentReads(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	key := cache.Key().File("test.txt").Build()
	cache.Put(key).Bytes("output", []byte("data")).Commit()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cache.Get(key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentWrites benchmarks concurrent cache writes to different keys
func BenchmarkConcurrentWrites(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)

	var counter int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Each goroutine writes to a unique key
			id := fmt.Sprintf("%d", counter)
			counter++
			key := cache.Key().File("test.txt").String("id", id).Build()
			err := cache.Put(key).Bytes("output", []byte("data")).Commit()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkManifestSave benchmarks manifest save operation
func BenchmarkManifestSave(b *testing.B) {
	cache, _ := setupBenchCache(b)
	defer cache.Close()

	m := &manifest{
		KeyHash:     "abc123def456",
		InputDescs:  []string{"file:test.txt", "string:version=1.0"},
		ExtraData:   map[string]string{"version": "1.0", "env": "test"},
		OutputFiles: map[string]string{"result": "/path/to/result.txt"},
		OutputData:  map[string]string{"data": "/path/to/data.dat"},
		OutputMeta:  map[string]string{"duration": "100ms"},
		OutputHash:  "xyz789",
		CreatedAt:   cache.now(),
		AccessedAt:  cache.now(),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		err := cache.saveManifest(m)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkManifestLoad benchmarks manifest load operation
func BenchmarkManifestLoad(b *testing.B) {
	cache, _ := setupBenchCache(b)
	defer cache.Close()

	// Save a manifest first
	m := &manifest{
		KeyHash:     "abc123def456",
		InputDescs:  []string{"file:test.txt"},
		ExtraData:   map[string]string{"version": "1.0"},
		OutputFiles: map[string]string{"result": "/path/to/result.txt"},
		OutputData:  map[string]string{"data": "/path/to/data.dat"},
		OutputMeta:  map[string]string{"duration": "100ms"},
		OutputHash:  "xyz789",
		CreatedAt:   cache.now(),
		AccessedAt:  cache.now(),
	}
	_ = cache.saveManifest(m)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cache.loadManifest("abc123def456")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGlobExpansion benchmarks glob pattern expansion
func BenchmarkGlobExpansion(b *testing.B) {
	fs := afero.NewMemMapFs()

	// Create test structure
	for i := 0; i < 10; i++ {
		dir := fmt.Sprintf("dir%d", i)
		fs.MkdirAll(dir, 0o755)
		for j := 0; j < 10; j++ {
			afero.WriteFile(fs, fmt.Sprintf("%s/file%d.go", dir, j), []byte("code"), 0o644)
			afero.WriteFile(fs, fmt.Sprintf("%s/file%d.txt", dir, j), []byte("text"), 0o644)
		}
	}

	b.Run("SimplePattern", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := expandGlob("dir0/*.go", fs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RecursivePattern", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := expandGlob("**/*.go", fs)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkMatchGlobPattern benchmarks glob pattern matching
func BenchmarkMatchGlobPattern(b *testing.B) {
	paths := []string{
		"src/pkg/core/main.go",
		"src/pkg/util/helper.go",
		"tests/unit/test.go",
		"a/b/c/d/e/f/deep.go",
	}

	patterns := []string{
		"**/*.go",
		"src/**/*.go",
		"**/core/*.go",
		"a/**/f/*.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		pattern := patterns[i%len(patterns)]
		matchesGlobPattern(path, pattern)
	}
}

// BenchmarkStatsSmallCache benchmarks Stats on small cache
func BenchmarkStatsSmallCache(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 10 entries
	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	for i := 0; i < 10; i++ {
		key := cache.Key().File("test.txt").String("id", fmt.Sprintf("%d", i)).Build()
		cache.Put(key).Bytes("output", []byte("data")).Commit()
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cache.Stats()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStatsLargeCache benchmarks Stats on larger cache
func BenchmarkStatsLargeCache(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Create 100 entries
	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	for i := 0; i < 100; i++ {
		key := cache.Key().File("test.txt").String("id", fmt.Sprintf("%d", i)).Build()
		cache.Put(key).Bytes("output", []byte("data")).Commit()
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := cache.Stats()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHas benchmarks the Has operation
func BenchmarkHas(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	key := cache.Key().File("test.txt").Build()
	cache.Put(key).Bytes("output", []byte("data")).Commit()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Has(key)
	}
}

// BenchmarkDelete benchmarks cache deletion
func BenchmarkDelete(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create entry
		key := cache.Key().File("test.txt").String("iter", fmt.Sprintf("%d", i)).Build()
		cache.Put(key).Bytes("output", []byte("data")).Commit()
		b.StartTimer()

		// Benchmark the delete
		err := cache.Delete(key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkKeyBuilder benchmarks key building
func BenchmarkKeyBuilder(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	afero.WriteFile(fs, "file1.txt", []byte("content1"), 0o644)
	afero.WriteFile(fs, "file2.txt", []byte("content2"), 0o644)
	afero.WriteFile(fs, "file3.txt", []byte("content3"), 0o644)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cache.Key().
			File("file1.txt").
			File("file2.txt").
			File("file3.txt").
			String("version", "1.0").
			String("env", "test").
			Build()
	}
}

// BenchmarkConcurrentMixedOperations benchmarks realistic mixed workload
func BenchmarkConcurrentMixedOperations(b *testing.B) {
	cache, fs := setupBenchCache(b)
	defer cache.Close()

	// Pre-populate with some entries
	afero.WriteFile(fs, "test.txt", []byte("content"), 0o644)
	for i := 0; i < 10; i++ {
		key := cache.Key().File("test.txt").String("id", fmt.Sprintf("%d", i)).Build()
		cache.Put(key).Bytes("output", []byte("data")).Commit()
	}

	b.ResetTimer()
	var counter int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := counter % 20
			counter++
			key := cache.Key().File("test.txt").String("id", fmt.Sprintf("%d", id)).Build()

			// Mixed operations: 50% read, 30% write, 20% has
			op := id % 10
			if op < 5 {
				// Read
				cache.Get(key)
			} else if op < 8 {
				// Write
				cache.Put(key).Bytes("output", []byte("data")).Commit()
			} else {
				// Has
				cache.Has(key)
			}
		}
	})
}
