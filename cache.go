package LCache_go

import (
	"go.uber.org/zap"
	"lcache/store"
	"sync"
	"sync/atomic"
	"time"
)

var (
	logger, _ = zap.NewProduction()
)

// encapsulates a cache entry

type Cache struct {
	mu          sync.RWMutex
	opts        CacheOptions
	store       store.Store
	hits        int64
	misses      int64
	initialized int32
	closed      int32
}

type CacheOptions struct {
	CacheType   store.CacheType // Type of cache, e.g., LRU, LRU2
	MaxBytes    int64
	CleanupTime time.Duration
	OnEvicted   func(key string, value store.Value) // Callback when an item is evicted
}

func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		MaxBytes:    8 * 1024 * 1024, // 8MB
		CleanupTime: time.Minute,
		CacheType:   store.LRU,
		OnEvicted:   nil,
	}
}

func NewCache(opts CacheOptions) *Cache {
	return &Cache{
		opts: opts,
	}
}

func (c *Cache) ensureCacheInitialized() {
	// if initialized
	if atomic.LoadInt32(&c.initialized) == 1 {
		return
	}
	// if not initialized, lock and set initialized
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized == 0 {
		c.store = store.NewStore(c.opts.CacheType, store.Options{
			MaxBytes:        c.opts.MaxBytes,
			CleanupInterval: c.opts.CleanupTime,
		})
		atomic.StoreInt32(&c.initialized, 1)
		logger.Info("Cache initialized", zap.String("cacheType", string(c.opts.CacheType)),
			zap.Int64("maxBytes", c.opts.MaxBytes))
	}
}
func OpenedAndInitialized(c *Cache) bool {
	if atomic.LoadInt32(&c.closed) == 1 {
		logger.Error("Cache is closed")
		return false
	}
	c.ensureCacheInitialized()
	return true
}

func (c *Cache) Get(key string) (ByteView, bool) {
	if !OpenedAndInitialized(c) {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.store.Get(key)
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}
	if bv, ok := value.(ByteView); ok {
		atomic.AddInt64(&c.hits, 1)
		return bv, true
	} else {
		logger.Warn("Type assertion failed for key", zap.String("key", key), zap.String("expectedType", "ByteView"))
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}
}

func (c *Cache) Add(key string, value ByteView) {
	if !OpenedAndInitialized(c) {
		logger.Warn("Attempted to add to a closed or uninitialized cache", zap.String("key", key))
		return
	}
	// add lock or not?
	//c.mu.Lock()
	//defer c.mu.Unlock()
	if err := c.store.Set(key, value); err != nil {
		logger.Warn("Failed to add key to cache", zap.String("key", key), zap.Error(err))
	}
}

func (c *Cache) AddWithExpiration(key string, value ByteView, expirationTime time.Time) {
	if !OpenedAndInitialized(c) {
		logger.Warn("Attempted to add with expiration to a closed or uninitialized cache", zap.String("key", key))
		return
	}
	expiration := time.Until(expirationTime)
	if expiration <= 0 {
		logger.Warn("Expiration time must be in the future", zap.String("key", key), zap.Duration("expiration", expiration))
		return
	}
	if err := c.store.SetWithExpiration(key, value, expiration); err != nil {
		logger.Warn("Failed to add key with expiration to cache", zap.String("key", key), zap.Error(err))
	}
}

func (c *Cache) Delete(key string) bool {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		logger.Warn("Attempted to delete from a closed cache", zap.String("key", key))
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	deleted := c.store.Delete(key)
	if deleted {
		logger.Info("Key deleted from cache", zap.String("key", key))
	} else {
		logger.Warn("Key not found for deletion", zap.String("key", key))
	}
	return deleted
}

func (c *Cache) Clear() {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		logger.Warn("Attempted to clear a closed or uninitialized cache")
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store.Clear()
	logger.Info("Cache cleared")
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
	logger.Info("Cache statistics reset")
}

func (c *Cache) Len() int {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		logger.Warn("Attempted to get length of a closed or uninitialized cache")
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	length := c.store.Len()
	logger.Info("Cache length retrieved", zap.Int("length", length))
	return length
}

func (c *Cache) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		logger.Warn("Cache is already closed")
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// check
	if c.store != nil {
		c.store.Close()
		c.store = nil
	}
	atomic.StoreInt32(&c.initialized, 0)
	logger.Info("Cache closed and resources released")
	logger.Info("Cache statistics", zap.Int64("hits", c.hits), zap.Int64("misses", c.misses))
}

func (c *Cache) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"initialized": atomic.LoadInt32(&c.initialized) == 1,
		"closed":      atomic.LoadInt32(&c.closed) == 1,
		"hits":        atomic.LoadInt64(&c.hits),
		"misses":      atomic.LoadInt64(&c.misses),
		"size":        c.Len(),
	}
	totalRequests := stats["hits"].(int64) + stats["misses"].(int64)
	if totalRequests > 0 {
		stats["hit_rate"] = float64(stats["hits"].(int64)) / float64(totalRequests)
	} else {
		stats["hit_rate"] = 0.0
	}

	return stats
}
