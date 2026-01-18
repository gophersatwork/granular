package granular

import "sync"

// keyLocks provides per-key locking to allow concurrent access to different keys.
// Uses a sharded approach with 256 shards to balance parallelism and memory overhead.
// The shard is determined by the first byte of the key hash.
type keyLocks struct {
	shards [256]*sync.RWMutex // Sharded by first byte of key hash
}

// newKeyLocks creates a new keyLocks instance with initialized shards.
func newKeyLocks() *keyLocks {
	kl := &keyLocks{}
	for i := range kl.shards {
		kl.shards[i] = &sync.RWMutex{}
	}
	return kl
}

// lockKey acquires an exclusive lock for the shard containing the key.
func (kl *keyLocks) lockKey(keyHash string) {
	shard := keyHash[0] // First byte determines shard
	kl.shards[shard].Lock()
}

// unlockKey releases the exclusive lock for the shard containing the key.
func (kl *keyLocks) unlockKey(keyHash string) {
	shard := keyHash[0]
	kl.shards[shard].Unlock()
}

// rlockKey acquires a read lock for the shard containing the key.
func (kl *keyLocks) rlockKey(keyHash string) {
	shard := keyHash[0]
	kl.shards[shard].RLock()
}

// runlockKey releases the read lock for the shard containing the key.
func (kl *keyLocks) runlockKey(keyHash string) {
	shard := keyHash[0]
	kl.shards[shard].RUnlock()
}
