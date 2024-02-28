package cache

import (
	"sync"
	"time"
)

type shard[T any] struct {
	sync.RWMutex
	store map[string]*Item[T]
}

func (b *shard[T]) itemCount() int {
	b.RLock()
	defer b.RUnlock()
	return len(b.store)
}

func (b *shard[T]) get(key string) *Item[T] {
	b.RLock()
	defer b.RUnlock()
	return b.store[key]
}

func (b *shard[T]) set(key string, value T, duration time.Duration) (*Item[T], *Item[T]) {
	expires := time.Now().Add(duration).UnixNano()
	item := newItem(key, value, expires)
	b.Lock()
	existing := b.store[key]
	b.store[key] = item
	b.Unlock()
	return item, existing
}

func (b *shard[T]) delete(key string) *Item[T] {
	b.Lock()
	item := b.store[key]
	delete(b.store, key)
	b.Unlock()
	return item
}

func (b *shard[T]) clear() {
	b.Lock()
	b.store = make(map[string]*Item[T])
	b.Unlock()
}
