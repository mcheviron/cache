package cache

import (
	"sync"
	"time"
)

type bucket[T any] struct {
	sync.RWMutex
	store map[string]*Item[T]
}

func (b *bucket[T]) itemCount() int {
	b.RLock()
	defer b.RUnlock()
	return len(b.store)
}

func (b *bucket[T]) get(key string) *Item[T] {
	b.RLock()
	defer b.RUnlock()
	return b.store[key]
}

func (b *bucket[T]) set(key string, value T, duration time.Duration) (*Item[T], *Item[T]) {
	expires := time.Now().Add(duration).UnixNano()
	item := newItem(key, value, expires)
	b.Lock()
	existing := b.store[key]
	b.store[key] = item
	b.Unlock()
	return item, existing
}

func (b *bucket[T]) delete(key string) *Item[T] {
	b.Lock()
	item := b.store[key]
	delete(b.store, key)
	b.Unlock()
	return item
}

func (b *bucket[T]) clear() {
	b.Lock()
	b.store = make(map[string]*Item[T])
	b.Unlock()
}
