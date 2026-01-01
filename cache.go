package cache

import (
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
)

type Cache[T any] struct {
	cfg       Config
	shards    []*shard[T]
	shardMask uint32

	accessClock uint64
	size        int64

	weigher func(key string, value T) int
}

func New[T any](config Config) *Cache[T] {
	cfg := config.Build()

	c := &Cache[T]{
		cfg:       cfg,
		shardMask: uint32(cfg.Shards) - 1,
		shards:    make([]*shard[T], cfg.Shards),
	}
	for i := range c.shards {
		c.shards[i] = newShard[T]()
	}

	if cfg.Weigher != nil {
		c.weigher = func(key string, value T) int {
			return cfg.Weigher(key, any(value))
		}
	} else {
		c.weigher = defaultWeigh[T]
	}

	return c
}

func (c *Cache[T]) ItemCount() int {
	count := 0
	for _, b := range c.shards {
		count += b.itemCount()
	}
	return count
}

func (c *Cache[T]) Len() int {
	return c.ItemCount()
}

func (c *Cache[T]) IsEmpty() bool {
	return c.Len() == 0
}

func (c *Cache[T]) Get(key string) *Item[T] {
	item := c.getShard(key).get(key)
	if item == nil {
		return nil
	}

	if !item.Expired() {
		item.touch(c.nextTick())
	}

	return item
}

func (c *Cache[T]) Peek(key string) *Item[T] {
	return c.getShard(key).get(key)
}

func (c *Cache[T]) Set(key string, value T, duration time.Duration) {
	expires := time.Now().Add(duration).UnixNano()
	weight := c.weigher(key, value)
	item := newItem(key, value, expires, weight)
	item.touch(c.nextTick())

	old := c.getShard(key).set(key, item)
	if old != nil {
		atomic.AddInt64(&c.size, -int64(old.weight))
	}
	atomic.AddInt64(&c.size, int64(item.weight))

	c.evictIfNeeded()
}

func (c *Cache[T]) Delete(key string) {
	if item := c.getShard(key).delete(key); item != nil {
		atomic.AddInt64(&c.size, -int64(item.weight))
	}
}

func (c *Cache[T]) Replace(key string, value T) bool {
	item := c.Peek(key)
	if item == nil {
		return false
	}
	c.Set(key, value, item.TTL())
	return true
}

func (c *Cache[T]) Extend(key string, duration time.Duration) bool {
	item := c.Peek(key)
	if item == nil {
		return false
	}
	item.Extend(duration)
	return true
}

func (c *Cache[T]) Clear() {
	for _, s := range c.shards {
		s.clear()
	}
	atomic.StoreInt64(&c.size, 0)
}

func (c *Cache[T]) Range(fn func(key string, value T) bool) {
	for _, shard := range c.shards {
		if !shard.forEach(fn) {
			return
		}
	}
}

func (c *Cache[T]) Filter(pattern string) []*Item[T] {
	var result []*Item[T]
	c.Range(func(key string, value T) bool {
		if strings.Contains(key, pattern) {
			item := c.Get(key)
			if item != nil {
				result = append(result, item)
			}
		}
		return true
	})
	return result
}

func (c *Cache[T]) getShard(key string) *shard[T] {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return c.shards[h.Sum32()&c.shardMask]
}

func (c *Cache[T]) evictIfNeeded() {
	for evicted := 0; evicted < c.cfg.ItemsToPrune; evicted++ {
		if int(atomic.LoadInt64(&c.size)) <= c.cfg.MaxWeight {
			return
		}

		candidate := c.pickEvictionCandidate()
		if candidate == nil {
			return
		}

		if c.getShard(candidate.key).deleteIfSame(candidate.key, candidate) {
			atomic.AddInt64(&c.size, -int64(candidate.weight))
		}
	}
}

func (c *Cache[T]) pickEvictionCandidate() *Item[T] {
	var best *Item[T]
	bestTick := uint64(^uint64(0))

	for i := 0; i < c.cfg.SampleSize; i++ {
		shard := c.shards[rand.IntN(len(c.shards))]
		item := shard.sampleNth(rand.Uint64())
		if item == nil {
			continue
		}

		tick := item.lastAccessTick()
		if best == nil || tick < bestTick {
			best = item
			bestTick = tick
		}
	}

	if best != nil {
		return best
	}

	var oldest *Item[T]
	oldestTick := uint64(^uint64(0))
	c.Range(func(_ string, _ T) bool { return true })
	for _, shard := range c.shards {
		shard.RLock()
		for _, item := range shard.store {
			tick := item.lastAccessTick()
			if oldest == nil || tick < oldestTick {
				oldest = item
				oldestTick = tick
			}
		}
		shard.RUnlock()
	}
	return oldest
}

func (c *Cache[T]) nextTick() uint64 {
	return atomic.AddUint64(&c.accessClock, 1)
}
