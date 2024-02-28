package cache

import "sync"

type freeList[T any] struct {
	sync.RWMutex
	items []*Item[T]
}

func newFreeList[T any](size int) freeList[T] {
	return freeList[T]{
		items: make([]*Item[T], 0, size),
	}
}

func (f *freeList[T]) get() *Item[T] {
	f.Lock()
	defer f.Unlock()

	if len(f.items) == 0 {
		return nil
	}

	i := f.items[len(f.items)-1]
	f.items = f.items[:len(f.items)-1]
	return i
}

func (f *freeList[T]) put(i *Item[T]) {
	f.Lock()
	defer f.Unlock()

	f.items = append(f.items, i)
}

func (f *freeList[T]) len() int {
	f.RLock()
	defer f.RUnlock()

	return len(f.items)
}

func (f *freeList[T]) cap() int {
	f.RLock()
	defer f.RUnlock()

	return cap(f.items)
}
