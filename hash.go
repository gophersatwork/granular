package granular

import (
	"fmt"
	"hash"
	"io"
	"sync"
)

// Default size for the buffer used when hashing files
const defaultBufferSize = 32 * 1024 // 32KB

// bufferPool is a pool of byte slices used for file I/O during hashing
var bufferPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, defaultBufferSize)
		return &buffer
	},
}

// hashFile hashes the content from a reader using the provided hash function.
func hashFile(content io.Reader, h hash.Hash) error {
	bufPtr := bufferPool.Get().(*[]byte)
	buffer := *bufPtr
	defer bufferPool.Put(bufPtr)

	// Hash the file content
	_, err := io.CopyBuffer(h, content, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}
	return nil
}
