package store

import "time"

type Store interface {
	Get(key string) (Value, bool)
	Set(key string, value Value) error
	SetWithExpiration(key string, value Value, expiration time.Duration) error
	Delete(key string) bool
	Clear()
	Len() int
	Close()
}

type Value interface {
	Len() int
}

type CacheType string

const (
	LRU  CacheType = "lru"
	LRU2 CacheType = "lru2"
)

type Options struct {
	MaxBytes        int64
	CleanupInterval time.Duration
	OnEvicted       func(key string, value Value) // Callback when an item is evicted
}

func DefaultOptions() Options {
	return Options{
		MaxBytes:        8 * 1024 * 1024, // 8MB
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}
}

func NewStore(cacheType CacheType, opts Options) Store {
	switch cacheType {
	case LRU2:
		return newLRU2Store(opts)
	case LRU:
		return newLRUStore(opts)
	default:
		return newLRUStore(opts)
	}
}
