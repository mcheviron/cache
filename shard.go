package cache

import "sync"

type shard[T any] struct {
	sync.RWMutex
	store map[string]*Item[T]
	items []*Item[T]
	pos   map[string]int
}

func newShard[T any]() *shard[T] {
	return &shard[T]{
		store: make(map[string]*Item[T]),
		pos:   make(map[string]int),
	}
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

func (s *shard[T]) set(key string, item *Item[T]) (old *Item[T]) {
	s.Lock()
	defer s.Unlock()

	old = s.store[key]
	s.store[key] = item

	if old == nil {
		s.pos[key] = len(s.items)
		s.items = append(s.items, item)
		return old
	}

	idx := s.pos[key]
	s.items[idx] = item
	return old
}

func (s *shard[T]) delete(key string) (old *Item[T]) {
	s.Lock()
	defer s.Unlock()

	old = s.store[key]
	if old == nil {
		return nil
	}

	delete(s.store, key)

	idx := s.pos[key]
	lastIdx := len(s.items) - 1
	if idx != lastIdx {
		s.items[idx] = s.items[lastIdx]
		s.pos[s.items[idx].key] = idx
	}
	s.items = s.items[:lastIdx]
	delete(s.pos, key)

	return old
}

func (s *shard[T]) deleteIfSame(key string, expected *Item[T]) bool {
	s.Lock()
	defer s.Unlock()

	current := s.store[key]
	if current == nil || current != expected {
		return false
	}

	delete(s.store, key)

	idx := s.pos[key]
	lastIdx := len(s.items) - 1
	if idx != lastIdx {
		s.items[idx] = s.items[lastIdx]
		s.pos[s.items[idx].key] = idx
	}
	s.items = s.items[:lastIdx]
	delete(s.pos, key)

	return true
}

func (s *shard[T]) sampleNth(n uint64) *Item[T] {
	s.RLock()
	defer s.RUnlock()
	if len(s.items) == 0 {
		return nil
	}
	return s.items[n%uint64(len(s.items))]
}

func (s *shard[T]) clear() {
	s.Lock()
	defer s.Unlock()
	s.store = make(map[string]*Item[T])
	s.items = nil
	s.pos = make(map[string]int)
}

func (s *shard[T]) forEach(fn func(key string, value T) bool) bool {
	s.RLock()
	defer s.RUnlock()
	for _, item := range s.store {
		if !fn(item.key, item.value) {
			return false
		}
	}
	return true
}
