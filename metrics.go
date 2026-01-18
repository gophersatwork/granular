package granular

import "time"

// MetricsHooks defines callbacks for cache events.
// All hooks are optional - nil hooks are ignored.
type MetricsHooks struct {
	// OnHit is called when a cache lookup finds an entry.
	// keyHash is the hash of the key, size is the entry size in bytes.
	OnHit func(keyHash string, size int64)

	// OnMiss is called when a cache lookup doesn't find an entry.
	OnMiss func(keyHash string)

	// OnPut is called when an entry is stored in the cache.
	// size is the total size of the stored files and data.
	OnPut func(keyHash string, size int64, duration time.Duration)

	// OnEvict is called when an entry is evicted (LRU or manual delete).
	OnEvict func(keyHash string, size int64, reason EvictReason)

	// OnError is called when an operation fails.
	OnError func(op string, err error)
}

// EvictReason indicates why an entry was evicted.
type EvictReason string

const (
	EvictReasonLRU     EvictReason = "lru"     // Evicted due to size limit
	EvictReasonExpired EvictReason = "expired" // Evicted due to age (Prune)
	EvictReasonManual  EvictReason = "manual"  // Evicted via Delete()
	EvictReasonClear   EvictReason = "clear"   // Evicted via Clear()
)

// helper to safely call hooks.
func (h *MetricsHooks) hit(keyHash string, size int64) {
	if h != nil && h.OnHit != nil {
		h.OnHit(keyHash, size)
	}
}

func (h *MetricsHooks) miss(keyHash string) {
	if h != nil && h.OnMiss != nil {
		h.OnMiss(keyHash)
	}
}

func (h *MetricsHooks) put(keyHash string, size int64, duration time.Duration) {
	if h != nil && h.OnPut != nil {
		h.OnPut(keyHash, size, duration)
	}
}

func (h *MetricsHooks) evict(keyHash string, size int64, reason EvictReason) {
	if h != nil && h.OnEvict != nil {
		h.OnEvict(keyHash, size, reason)
	}
}

func (h *MetricsHooks) error(op string, err error) {
	if h != nil && h.OnError != nil {
		h.OnError(op, err)
	}
}
