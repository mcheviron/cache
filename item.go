package cache

import (
	"reflect"
	"sync/atomic"
	"time"
)

// Item is an entry stored in a Cache.
//
// Items are owned by the cache. Methods are safe to call concurrently.
//
// Note: the cache may delete/evict a key, but an *Item previously returned by
// Get/Peek remains a valid pointer (Go GC keeps it alive). It may represent a
// value no longer reachable from the cache.
type Item[T any] struct {
	value      T
	key        string
	expires    int64
	weight     int
	lastAccess uint64
}

func newItem[T any](key string, value T, expires int64, weight int) *Item[T] {
	return &Item[T]{
		key:     key,
		value:   value,
		expires: expires,
		weight:  weight,
	}
}

// Value returns the stored value.
func (i *Item[T]) Value() T {
	return i.value
}

// Key returns the item key.
func (i *Item[T]) Key() string {
	return i.key
}

// Extend sets the expiration time to now + duration.
func (i *Item[T]) Extend(duration time.Duration) {
	atomic.StoreInt64(&i.expires, time.Now().Add(duration).UnixNano())
}

// Expired reports whether the item is expired at the current time.
func (i *Item[T]) Expired() bool {
	expires := atomic.LoadInt64(&i.expires)
	return expires < time.Now().UnixNano()
}

// TTL returns the remaining time-to-live.
//
// If the item is expired, TTL may be <= 0.
func (i *Item[T]) TTL() time.Duration {
	expires := atomic.LoadInt64(&i.expires)
	return time.Nanosecond * time.Duration(expires-time.Now().UnixNano())
}

func (i *Item[T]) touch(tick uint64) {
	atomic.StoreUint64(&i.lastAccess, tick)
}

func (i *Item[T]) lastAccessTick() uint64 {
	return atomic.LoadUint64(&i.lastAccess)
}

func defaultWeigh[T any](key string, value T) int {
	return len(key) + int(reflect.TypeOf(value).Size())
}
