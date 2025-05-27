package store

import (
	"container/list"
	"sync"
	"time"
)

type lRUStore struct {
	mu              sync.RWMutex
	list            *list.List
	items           map[string]*list.Element
	expires         map[string]time.Time
	maxBytes        int64
	usedBytes       int64
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	closeCh         chan bool
	onEvicted       func(key string, value Value)
}

type lruEntry struct {
	key   string
	value Value
}

func newLRUStore(opt Options) *lRUStore {

	store := &lRUStore{
		list:            list.New(),
		items:           make(map[string]*list.Element),
		expires:         make(map[string]time.Time),
		maxBytes:        opt.MaxBytes,
		cleanupInterval: opt.CleanupInterval,
		closeCh:         make(chan bool),
		cleanupTicker:   time.NewTicker(opt.CleanupInterval),
	}
	return store
}

func (l *lRUStore) Get(key string) (Value, bool) {
	l.mu.RLocker()
	elem, ok := l.items[key]
	if !ok {
		l.mu.RUnlock()
		return nil, false
	}
	value := elem.Value.(*lruEntry).value
	l.mu.RUnlock()

	// lru strategy: 将访问的元素移动到链表头部
	l.mu.Lock()
	if _, ok := l.items[key]; ok {
		l.list.MoveToFront(elem)
	}
	l.mu.Unlock()

	return value, true
}

func (l *lRUStore) Set(key string, value Value) error {
	return l.SetWithExpiration(key, value, 0)
}

func (l *lRUStore) SetWithExpiration(key string, value Value, expiration time.Duration) error {
	if value == nil {
		l.Delete(key)
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		// If the key already exists, update the value and move it to the front
		oldEntry := elem.Value.(*lruEntry)
		l.usedBytes -= int64(oldEntry.value.Len())
		l.usedBytes += int64(value.Len())
		oldEntry.value = value
		l.list.MoveToFront(elem)
		if expiration > 0 {
			l.expires[key] = time.Now().Add(expiration)
		}
	} else {
		// If the key does not exist, create a new entry
		entry := &lruEntry{key: key, value: value}
		elem := l.list.PushFront(entry)
		l.items[key] = elem
		l.usedBytes += int64(value.Len())
		if expiration > 0 {
			l.expires[key] = time.Now().Add(expiration)
		}
	}
	l.evict()
	return nil
}

func (l *lRUStore) Delete(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		l.list.Remove(elem)
		delete(l.items, key)
		l.usedBytes -= int64(elem.Value.(*lruEntry).value.Len())
		delete(l.expires, key)
		return true
	} else {
		return false
	}
}

func (l *lRUStore) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.onEvicted != nil {
		for key, elem := range l.items {
			l.onEvicted(key, elem.Value.(*lruEntry).value)
		}
	}

	l.list.Init()
	l.items = make(map[string]*list.Element)
	l.expires = make(map[string]time.Time)
	l.usedBytes = 0
}

func (l *lRUStore) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.list.Len()
}

func (l *lRUStore) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cleanupTicker != nil {
		l.cleanupTicker.Stop()
	}
	close(l.closeCh)
}

func (l *lRUStore) evict() {
	// to clean up expired items and items exceeding maxBytes, need to hold the lock
	now := time.Now()

	// Clean up expired items
	for key, expireTime := range l.expires {
		if expireTime.Before(now) {
			if elem, ok := l.items[key]; ok {
				l.list.Remove(elem)
				delete(l.items, key)
				l.usedBytes -= int64(elem.Value.(*lruEntry).value.Len())
				delete(l.expires, key)
			} else {
				delete(l.expires, key)
			}
		}
	}
	// Clean up items exceeding maxBytes
	for {
		if l.maxBytes > 0 && l.usedBytes > l.maxBytes && l.list.Len() > 0 {
			elem := l.list.Back()
			if elem == nil {
				break
			}
			entry := elem.Value.(*lruEntry)
			l.list.Remove(elem)
			delete(l.items, entry.key)
			l.usedBytes -= int64(entry.value.Len())
			delete(l.expires, entry.key)
		} else {
			break
		}
	}
}

func (l *lRUStore) CleanupStore() {
	for {
		select {
		case <-l.closeCh:
			return
		case <-l.cleanupTicker.C:
			l.mu.Lock()
			l.evict()
			l.mu.Unlock()
		}
	}
}
