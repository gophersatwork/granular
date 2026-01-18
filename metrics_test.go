package granular

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestMetricsHooks_OnHitOnMiss(t *testing.T) {
	var hitCount, missCount atomic.Int32
	var lastHitKeyHash, lastMissKeyHash string
	var lastHitSize int64
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnHit: func(keyHash string, size int64) {
			mu.Lock()
			lastHitKeyHash = keyHash
			lastHitSize = size
			mu.Unlock()
			hitCount.Add(1)
		},
		OnMiss: func(keyHash string) {
			mu.Lock()
			lastMissKeyHash = keyHash
			mu.Unlock()
			missCount.Add(1)
		},
	}

	cache, err := Open("", WithFs(afero.NewMemMapFs()), WithMetrics(hooks))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	// Build a key
	key := cache.Key().String("test", "value").Build()

	// First get should be a miss
	_, err = cache.Get(key)
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	if missCount.Load() != 1 {
		t.Errorf("expected missCount=1, got %d", missCount.Load())
	}
	if hitCount.Load() != 0 {
		t.Errorf("expected hitCount=0, got %d", hitCount.Load())
	}
	mu.Lock()
	if lastMissKeyHash == "" {
		t.Error("expected lastMissKeyHash to be set")
	}
	mu.Unlock()

	// Put an entry
	err = cache.Put(key).Bytes("data", []byte("hello world")).Commit()
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Second get should be a hit
	result, err := cache.Get(key)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if hitCount.Load() != 1 {
		t.Errorf("expected hitCount=1, got %d", hitCount.Load())
	}
	if missCount.Load() != 1 {
		t.Errorf("expected missCount=1 (unchanged), got %d", missCount.Load())
	}
	mu.Lock()
	if lastHitKeyHash == "" {
		t.Error("expected lastHitKeyHash to be set")
	}
	if lastHitSize <= 0 {
		t.Errorf("expected lastHitSize > 0, got %d", lastHitSize)
	}
	mu.Unlock()
}

func TestMetricsHooks_OnPut(t *testing.T) {
	var putCount atomic.Int32
	var lastPutKeyHash string
	var lastPutSize int64
	var lastPutDuration time.Duration
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnPut: func(keyHash string, size int64, duration time.Duration) {
			mu.Lock()
			lastPutKeyHash = keyHash
			lastPutSize = size
			lastPutDuration = duration
			mu.Unlock()
			putCount.Add(1)
		},
	}

	cache, err := Open("", WithFs(afero.NewMemMapFs()), WithMetrics(hooks))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "value").Build()
	data := []byte("hello world, this is test data")

	err = cache.Put(key).Bytes("data", data).Commit()
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	if putCount.Load() != 1 {
		t.Errorf("expected putCount=1, got %d", putCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	if lastPutKeyHash == "" {
		t.Error("expected lastPutKeyHash to be set")
	}
	if lastPutSize != int64(len(data)) {
		t.Errorf("expected lastPutSize=%d, got %d", len(data), lastPutSize)
	}
	if lastPutDuration <= 0 {
		t.Errorf("expected lastPutDuration > 0, got %v", lastPutDuration)
	}
}

func TestMetricsHooks_OnEvict_Manual(t *testing.T) {
	var evictCount atomic.Int32
	var lastEvictKeyHash string
	var lastEvictReason EvictReason
	var lastEvictSize int64
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnEvict: func(keyHash string, size int64, reason EvictReason) {
			mu.Lock()
			lastEvictKeyHash = keyHash
			lastEvictSize = size
			lastEvictReason = reason
			mu.Unlock()
			evictCount.Add(1)
		},
	}

	cache, err := Open("", WithFs(afero.NewMemMapFs()), WithMetrics(hooks))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "value").Build()
	err = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Delete should trigger evict with manual reason
	err = cache.Delete(key)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if evictCount.Load() != 1 {
		t.Errorf("expected evictCount=1, got %d", evictCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	if lastEvictKeyHash == "" {
		t.Error("expected lastEvictKeyHash to be set")
	}
	if lastEvictReason != EvictReasonManual {
		t.Errorf("expected reason=%s, got %s", EvictReasonManual, lastEvictReason)
	}
	if lastEvictSize <= 0 {
		t.Errorf("expected lastEvictSize > 0, got %d", lastEvictSize)
	}
}

func TestMetricsHooks_OnEvict_Clear(t *testing.T) {
	var evictCount atomic.Int32
	var evictReasons []EvictReason
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnEvict: func(keyHash string, size int64, reason EvictReason) {
			mu.Lock()
			evictReasons = append(evictReasons, reason)
			mu.Unlock()
			evictCount.Add(1)
		},
	}

	cache, err := Open("", WithFs(afero.NewMemMapFs()), WithMetrics(hooks))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	// Add multiple entries
	for i := 0; i < 3; i++ {
		key := cache.Key().String("index", fmt.Sprintf("%d", i)).Build()
		err = cache.Put(key).Bytes("data", []byte("hello")).Commit()
		if err != nil {
			t.Fatalf("failed to put: %v", err)
		}
	}

	// Clear should trigger evict for all entries
	err = cache.Clear()
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	if evictCount.Load() != 3 {
		t.Errorf("expected evictCount=3, got %d", evictCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	for i, reason := range evictReasons {
		if reason != EvictReasonClear {
			t.Errorf("eviction %d: expected reason=%s, got %s", i, EvictReasonClear, reason)
		}
	}
}

func TestMetricsHooks_OnEvict_LRU(t *testing.T) {
	var evictCount atomic.Int32
	var lastEvictReason EvictReason
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnEvict: func(keyHash string, size int64, reason EvictReason) {
			mu.Lock()
			lastEvictReason = reason
			mu.Unlock()
			evictCount.Add(1)
		},
	}

	// Create cache with small max size (100 bytes)
	cache, err := Open("",
		WithFs(afero.NewMemMapFs()),
		WithMetrics(hooks),
		WithMaxSize(100),
	)
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	// Create data that fills most of the cache
	data1 := make([]byte, 60) // 60 bytes
	for i := range data1 {
		data1[i] = 'a'
	}

	// Add first entry (should fit - 60 bytes < 100 bytes)
	key1 := cache.Key().String("key", "1").Build()
	err = cache.Put(key1).Bytes("data", data1).Commit()
	if err != nil {
		t.Fatalf("failed to put key1: %v", err)
	}

	// Add second entry (should trigger LRU eviction of first - 60 + 60 > 100)
	data2 := make([]byte, 60) // 60 bytes
	for i := range data2 {
		data2[i] = 'b'
	}
	key2 := cache.Key().String("key", "2").Build()
	err = cache.Put(key2).Bytes("data", data2).Commit()
	if err != nil {
		t.Fatalf("failed to put key2: %v", err)
	}

	if evictCount.Load() < 1 {
		t.Errorf("expected at least 1 eviction, got %d", evictCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	if lastEvictReason != EvictReasonLRU {
		t.Errorf("expected reason=%s, got %s", EvictReasonLRU, lastEvictReason)
	}
}

func TestMetricsHooks_OnEvict_Expired(t *testing.T) {
	var evictCount atomic.Int32
	var lastEvictReason EvictReason
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnEvict: func(keyHash string, size int64, reason EvictReason) {
			mu.Lock()
			lastEvictReason = reason
			mu.Unlock()
			evictCount.Add(1)
		},
	}

	// Control time for testing
	now := time.Now()
	cache, err := Open("",
		WithFs(afero.NewMemMapFs()),
		WithMetrics(hooks),
		WithNowFunc(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	// Add entry "in the past"
	key := cache.Key().String("test", "value").Build()
	err = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Advance time
	now = now.Add(2 * time.Hour)

	// Prune entries older than 1 hour
	count, err := cache.Prune(1 * time.Hour)
	if err != nil {
		t.Fatalf("failed to prune: %v", err)
	}
	if count != 1 {
		t.Errorf("expected prune count=1, got %d", count)
	}

	if evictCount.Load() != 1 {
		t.Errorf("expected evictCount=1, got %d", evictCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	if lastEvictReason != EvictReasonExpired {
		t.Errorf("expected reason=%s, got %s", EvictReasonExpired, lastEvictReason)
	}
}

func TestMetricsHooks_NilHooksDoNotPanic(t *testing.T) {
	// Cache with nil metrics
	cache, err := Open("", WithFs(afero.NewMemMapFs()))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "value").Build()

	// None of these should panic
	_, _ = cache.Get(key) // miss with nil metrics
	_ = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	_, _ = cache.Get(key) // hit with nil metrics
	_ = cache.Delete(key)

	// Put again and clear
	_ = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	_ = cache.Clear()
}

func TestMetricsHooks_PartialHooks(t *testing.T) {
	// Only set OnHit, leave others nil
	var hitCount atomic.Int32
	hooks := &MetricsHooks{
		OnHit: func(keyHash string, size int64) {
			hitCount.Add(1)
		},
		// OnMiss, OnPut, OnEvict, OnError are all nil
	}

	cache, err := Open("", WithFs(afero.NewMemMapFs()), WithMetrics(hooks))
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	key := cache.Key().String("test", "value").Build()

	// These should not panic even though some hooks are nil
	_, _ = cache.Get(key) // miss - OnMiss is nil, should not panic
	_ = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	_, _ = cache.Get(key) // hit - OnHit is set
	_ = cache.Delete(key) // evict - OnEvict is nil, should not panic

	if hitCount.Load() != 1 {
		t.Errorf("expected hitCount=1, got %d", hitCount.Load())
	}
}

func TestMetricsHooks_PruneUnused_Expired(t *testing.T) {
	var evictCount atomic.Int32
	var lastEvictReason EvictReason
	var mu sync.Mutex

	hooks := &MetricsHooks{
		OnEvict: func(keyHash string, size int64, reason EvictReason) {
			mu.Lock()
			lastEvictReason = reason
			mu.Unlock()
			evictCount.Add(1)
		},
	}

	// Control time for testing
	now := time.Now()
	cache, err := Open("",
		WithFs(afero.NewMemMapFs()),
		WithMetrics(hooks),
		WithNowFunc(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	defer cache.Close()

	// Add entry "in the past"
	key := cache.Key().String("test", "value").Build()
	err = cache.Put(key).Bytes("data", []byte("hello")).Commit()
	if err != nil {
		t.Fatalf("failed to put: %v", err)
	}

	// Advance time
	now = now.Add(2 * time.Hour)

	// PruneUnused entries not accessed in 1 hour
	count, err := cache.PruneUnused(1 * time.Hour)
	if err != nil {
		t.Fatalf("failed to prune unused: %v", err)
	}
	if count != 1 {
		t.Errorf("expected prune count=1, got %d", count)
	}

	if evictCount.Load() != 1 {
		t.Errorf("expected evictCount=1, got %d", evictCount.Load())
	}

	mu.Lock()
	defer mu.Unlock()

	if lastEvictReason != EvictReasonExpired {
		t.Errorf("expected reason=%s, got %s", EvictReasonExpired, lastEvictReason)
	}
}

func TestMetricsHooks_HelperMethodsNilSafe(t *testing.T) {
	// Test that helper methods on nil MetricsHooks don't panic
	var h *MetricsHooks

	// None of these should panic
	h.hit("keyhash", 100)
	h.miss("keyhash")
	h.put("keyhash", 100, time.Second)
	h.evict("keyhash", 100, EvictReasonLRU)
	h.error("op", nil)
}
