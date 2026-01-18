package granular

import (
	"compress/gzip"
	"io"

	"github.com/klauspost/compress/zstd"
)

// CompressionType defines the compression algorithm to use.
type CompressionType string

const (
	CompressionNone CompressionType = ""
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
)

// compressWriter wraps a writer with compression.
func compressWriter(w io.Writer, ct CompressionType) (io.WriteCloser, error) {
	switch ct {
	case CompressionGzip:
		return gzip.NewWriter(w), nil
	case CompressionZstd:
		return zstd.NewWriter(w)
	default:
		return &nopWriteCloser{w}, nil
	}
}

// decompressReader wraps a reader with decompression.
func decompressReader(r io.Reader, ct CompressionType) (io.ReadCloser, error) {
	switch ct {
	case CompressionGzip:
		return gzip.NewReader(r)
	case CompressionZstd:
		dec, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return dec.IOReadCloser(), nil
	default:
		return io.NopCloser(r), nil
	}
}

type nopWriteCloser struct{ io.Writer }

func (n *nopWriteCloser) Close() error { return nil }
