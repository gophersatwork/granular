package granular

import "sync"

// numKeyShards is the number of shards used for per-key locking.
// Must be a power of two for efficient modular arithmetic.
const numKeyShards = 256

// keyLocks provides per-key locking to allow concurrent access to different keys.
// Uses a sharded approach with numKeyShards shards to balance parallelism and memory overhead.
// The shard is determined by the first two hex characters of the key hash, parsed as a byte.
type keyLocks struct {
	shards [numKeyShards]*sync.Mutex
}

// newKeyLocks creates a new keyLocks instance with initialized shards.
func newKeyLocks() *keyLocks {
	kl := &keyLocks{}
	for i := range kl.shards {
		kl.shards[i] = &sync.Mutex{}
	}
	return kl
}

// shardIndex returns the shard index for the given hex-encoded key hash.
// Parses the first two hex characters as a byte (0-255) for full shard utilization.
func shardIndex(keyHash string) byte {
	if len(keyHash) < 2 {
		return 0
	}
	return hexVal(keyHash[0])<<4 | hexVal(keyHash[1])
}

// hexVal converts a single hex ASCII character to its numeric value (0-15).
func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

// lockKey acquires an exclusive lock for the shard containing the key.
func (kl *keyLocks) lockKey(keyHash string) {
	kl.shards[shardIndex(keyHash)].Lock()
}

// unlockKey releases the exclusive lock for the shard containing the key.
func (kl *keyLocks) unlockKey(keyHash string) {
	kl.shards[shardIndex(keyHash)].Unlock()
}
