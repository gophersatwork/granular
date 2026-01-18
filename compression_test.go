package granular

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestCompressionRoundTrip(t *testing.T) {
	testData := []byte(strings.Repeat("hello world compression test ", 1000))

	tests := []struct {
		name        string
		compression CompressionType
	}{
		{"none", CompressionNone},
		{"gzip", CompressionGzip},
		{"zstd", CompressionZstd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := Open("", WithFs(afero.NewMemMapFs()), WithCompression(tt.compression))
			if err != nil {
				t.Fatalf("failed to open cache: %v", err)
			}

			// Store data
			key := cache.Key().String("test", "key").Build()
			err = cache.Put(key).Bytes("data", testData).Commit()
			if err != nil {
				t.Fatalf("failed to store data: %v", err)
			}

			// Retrieve data
			result, err := cache.Get(key)
			if err != nil {
				t.Fatalf("failed to get data: %v", err)
			}

			got := result.Bytes("data")
			if !bytes.Equal(got, testData) {
				t.Errorf("data mismatch: got %d bytes, want %d bytes", len(got), len(testData))
			}
		})
	}
}

func TestCompressionWithFiles(t *testing.T) {
	testContent := []byte(strings.Repeat("file content for compression test ", 500))

	tests := []struct {
		name        string
		compression CompressionType
	}{
		{"none", CompressionNone},
		{"gzip", CompressionGzip},
		{"zstd", CompressionZstd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			cache, err := Open("", WithFs(fs), WithCompression(tt.compression))
			if err != nil {
				t.Fatalf("failed to open cache: %v", err)
			}

			// Create source file
			srcPath := "/src/test.txt"
			if err := fs.MkdirAll("/src", 0o755); err != nil {
				t.Fatalf("failed to create src dir: %v", err)
			}
			if err := afero.WriteFile(fs, srcPath, testContent, 0o644); err != nil {
				t.Fatalf("failed to write source file: %v", err)
			}

			// Store file
			key := cache.Key().String("test", "file").Build()
			err = cache.Put(key).File("output", srcPath).Commit()
			if err != nil {
				t.Fatalf("failed to store file: %v", err)
			}

			// Retrieve file
			result, err := cache.Get(key)
			if err != nil {
				t.Fatalf("failed to get result: %v", err)
			}

			// Copy to destination
			dstPath := "/dst/test.txt"
			if err := result.CopyFile("output", dstPath); err != nil {
				t.Fatalf("failed to copy file: %v", err)
			}

			// Verify content
			got, err := afero.ReadFile(fs, dstPath)
			if err != nil {
				t.Fatalf("failed to read destination file: %v", err)
			}
			if !bytes.Equal(got, testContent) {
				t.Errorf("content mismatch: got %d bytes, want %d bytes", len(got), len(testContent))
			}
		})
	}
}

func TestCompressionReducesSize(t *testing.T) {
	// Highly compressible data
	testData := []byte(strings.Repeat("aaaaaaaaaa", 10000))

	fs := afero.NewMemMapFs()

	// Store without compression
	cacheNone, err := Open("/cache-none", WithFs(fs), WithCompression(CompressionNone))
	if err != nil {
		t.Fatalf("failed to open uncompressed cache: %v", err)
	}
	keyNone := cacheNone.Key().String("test", "none").Build()
	if err := cacheNone.Put(keyNone).Bytes("data", testData).Commit(); err != nil {
		t.Fatalf("failed to store uncompressed: %v", err)
	}

	// Store with gzip
	cacheGzip, err := Open("/cache-gzip", WithFs(fs), WithCompression(CompressionGzip))
	if err != nil {
		t.Fatalf("failed to open gzip cache: %v", err)
	}
	keyGzip := cacheGzip.Key().String("test", "gzip").Build()
	if err := cacheGzip.Put(keyGzip).Bytes("data", testData).Commit(); err != nil {
		t.Fatalf("failed to store gzip: %v", err)
	}

	// Store with zstd
	cacheZstd, err := Open("/cache-zstd", WithFs(fs), WithCompression(CompressionZstd))
	if err != nil {
		t.Fatalf("failed to open zstd cache: %v", err)
	}
	keyZstd := cacheZstd.Key().String("test", "zstd").Build()
	if err := cacheZstd.Put(keyZstd).Bytes("data", testData).Commit(); err != nil {
		t.Fatalf("failed to store zstd: %v", err)
	}

	// Get stats
	statsNone, _ := cacheNone.Stats()
	statsGzip, _ := cacheGzip.Stats()
	statsZstd, _ := cacheZstd.Stats()

	t.Logf("Uncompressed size: %d bytes", statsNone.TotalSize)
	t.Logf("Gzip size: %d bytes (%.1f%% of original)", statsGzip.TotalSize, float64(statsGzip.TotalSize)/float64(statsNone.TotalSize)*100)
	t.Logf("Zstd size: %d bytes (%.1f%% of original)", statsZstd.TotalSize, float64(statsZstd.TotalSize)/float64(statsNone.TotalSize)*100)

	// Verify compression actually reduces size
	if statsGzip.TotalSize >= statsNone.TotalSize {
		t.Errorf("gzip should reduce size: got %d, uncompressed %d", statsGzip.TotalSize, statsNone.TotalSize)
	}
	if statsZstd.TotalSize >= statsNone.TotalSize {
		t.Errorf("zstd should reduce size: got %d, uncompressed %d", statsZstd.TotalSize, statsNone.TotalSize)
	}
}

func TestCompressWriterDecompressReader(t *testing.T) {
	testData := []byte("test data for compression")

	tests := []struct {
		name        string
		compression CompressionType
	}{
		{"none", CompressionNone},
		{"gzip", CompressionGzip},
		{"zstd", CompressionZstd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			var buf bytes.Buffer
			w, err := compressWriter(&buf, tt.compression)
			if err != nil {
				t.Fatalf("compressWriter failed: %v", err)
			}
			if _, err := w.Write(testData); err != nil {
				t.Fatalf("write failed: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close failed: %v", err)
			}

			// Decompress
			r, err := decompressReader(&buf, tt.compression)
			if err != nil {
				t.Fatalf("decompressReader failed: %v", err)
			}
			var result bytes.Buffer
			if _, err := result.ReadFrom(r); err != nil {
				t.Fatalf("read failed: %v", err)
			}
			if err := r.Close(); err != nil {
				t.Fatalf("reader close failed: %v", err)
			}

			if !bytes.Equal(result.Bytes(), testData) {
				t.Errorf("roundtrip failed: got %q, want %q", result.Bytes(), testData)
			}
		})
	}
}
