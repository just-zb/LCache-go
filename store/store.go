package store

import "time"

type Store interface {
	Get(key string) (string, bool)
	Set(key string, value string) error
	SetWithExpiration(key string, value string, expiration time.Duration) error
	Delete(key string) bool
	Clear()
	Len() int
	Close()
}

type CacheType string

const (
	LRU  CacheType = "lru"
	LRU2 CacheType = "lru2"
)

type Options struct {
	MaxBytes      int64
	CleanInterval time.Duration
}

func DefaultOptions() Options {
	return Options{
		MaxBytes:      8 * 1024 * 1024, // 8MB
		CleanInterval: time.Minute,
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
