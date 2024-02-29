package cache

import (
	"testing"
)

func TestFreeListGet(t *testing.T) {
	fl := newFreeList[int](10)

	item := &Item[int]{value: 42}
	fl.put(item)

	got := fl.get()

	if got != item {
		t.Errorf("Expected item to be %v, got %v", item, got)
	}
}

func TestFreeListPut(t *testing.T) {
	fl := newFreeList[int](10)

	item := &Item[int]{value: 42}
	fl.put(item)

	if len(fl.items) != 1 {
		t.Errorf("Expected free list length to be 1, got %d", len(fl.items))
	}

	if fl.items[0] != item {
		t.Errorf("Expected item to be %v, got %v", item, fl.items[0])
	}
}

func TestFreeListLen(t *testing.T) {
	fl := newFreeList[int](10)

	item := &Item[int]{value: 42}
	fl.put(item)

	if fl.len() != 1 {
		t.Errorf("Expected free list length to be 1, got %d", fl.len())
	}
}

func TestFreeListCap(t *testing.T) {
	fl := newFreeList[int](10)

	if fl.cap() != 10 {
		t.Errorf("Expected free list capacity to be 10, got %d", fl.cap())
	}
}
