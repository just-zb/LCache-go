package LCache_go

import (
	"sync"
	"sync/atomic"
	"time"
)

// encapsulates a cache entry

type Cache struct {
	mu          sync.RWMutex
	opts        CacheOptions
	hits        int64
	misses      int64
	initialized int32
	closed      int32
}

type CacheOptions struct {
	MaxBytes    int64
	CleanupTime time.Duration
}

func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		MaxBytes:    8 * 1024 * 1024, // 8MB
		CleanupTime: time.Minute,
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
		//TODO create the cache store here
	}
}

//TODO: implement cache methods like Get, Set, Delete, clear, len,close,stats,AddWithExpiration
