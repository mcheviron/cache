package cache

import (
	"reflect"
	"sync/atomic"
	"time"
)

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

func (i *Item[T]) Value() T {
	return i.value
}

func (i *Item[T]) Key() string {
	return i.key
}

func (i *Item[T]) Extend(duration time.Duration) {
	atomic.StoreInt64(&i.expires, time.Now().Add(duration).UnixNano())
}

func (i *Item[T]) Expired() bool {
	expires := atomic.LoadInt64(&i.expires)
	return expires < time.Now().UnixNano()
}

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
