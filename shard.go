package cache

import (
	"sync"
	"time"
)

type shard[T any] struct {
	sync.RWMutex
	store map[string]*Item[T]
}

func (s *shard[T]) itemCount() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.store)
}

func (s *shard[T]) get(key string) *Item[T] {
	s.RLock()
	defer s.RUnlock()
	return s.store[key]
}

func (s *shard[T]) set(key string, value T, duration time.Duration) (*Item[T], *Item[T]) {
	expires := time.Now().Add(duration).UnixNano()
	item := newItem(key, value, expires)
	s.Lock()
	existing := s.store[key]
	s.store[key] = item
	s.Unlock()
	return item, existing
}

func (s *shard[T]) delete(key string) *Item[T] {
	s.Lock()
	item := s.store[key]
	delete(s.store, key)
	s.Unlock()
	return item
}

func (s *shard[T]) clear() {
	s.Lock()
	s.store = make(map[string]*Item[T])
	s.Unlock()
}
