package cache

import (
	"reflect"
	"sync/atomic"
	"time"
)

type Item[T any] struct {
	value      T
	key        string
	node       *Node[*Item[T]]
	expires    int64
	size       int
	promotions int32
}

func newItem[T any](key string, value T, expires int64) *Item[T] {
	return &Item[T]{
		key:     key,
		value:   value,
		expires: expires,
		size:    int(reflect.TypeOf(value).Size()), // add this in the cache to not compute it every time
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
	expires := atomic.LoadInt64(&i.expires) // this field is acccessed concurrently
	return expires < time.Now().UnixNano()
}

func (i *Item[T]) TTL() time.Duration {
	expires := atomic.LoadInt64(&i.expires)
	return time.Nanosecond * time.Duration(expires-time.Now().UnixNano())
}

func (i *Item[T]) shouldPromote(getsPerPromote int32) bool {
	i.promotions++
	return i.promotions == getsPerPromote
}
