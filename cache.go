package cache

import (
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
)

// Cache is a sharded in-memory cache with TTL and size-based eviction.
//
// The cache stores items in shard maps for concurrent access. Each successful
// Get updates an access tick on the item. When the total weight exceeds
// Config.MaxWeight, eviction samples candidates across shards and evicts the
// item with the oldest access tick.
//
// TTL note:
//
// Expiration is not enforced automatically. Get/Peek may return an expired item.
// Consumers should check Item.Expired() if needed.
//
// Concurrency:
//
// Cache methods are safe for concurrent use.
//
// Returned pointers:
//
// Get/Peek return pointers to items owned by the cache. If the key is later
// evicted/deleted, previously returned pointers remain valid but may point to
// an item no longer reachable from the cache.
type Cache[T any] struct {
	cfg       Config
	shards    []*shard[T]
	shardMask uint32

	accessClock uint64
	size        int64

	weigher func(key string, value T) int
}

// New constructs a cache from the provided config.
//
// New calls config.Build() internally.
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

// ItemCount returns the number of keys currently stored.
func (c *Cache[T]) ItemCount() int {
	count := 0
	for _, b := range c.shards {
		count += b.itemCount()
	}
	return count
}

// Len is an alias for ItemCount.
func (c *Cache[T]) Len() int {
	return c.ItemCount()
}

// IsEmpty reports whether the cache is empty.
func (c *Cache[T]) IsEmpty() bool {
	return c.Len() == 0
}

// Get returns the item for key, touching access metadata.
//
// Returns nil if the key is not present.
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

// Peek returns the item for key without updating access metadata.
//
// This is useful for "check without affecting eviction".
func (c *Cache[T]) Peek(key string) *Item[T] {
	return c.getShard(key).get(key)
}

// Set inserts or updates a key with a TTL.
//
// If key existed, the previous item is replaced.
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

// Delete removes key if present.
func (c *Cache[T]) Delete(key string) {
	if item := c.getShard(key).delete(key); item != nil {
		atomic.AddInt64(&c.size, -int64(item.weight))
	}
}

// Replace updates the value of an existing key while preserving its TTL.
//
// Returns false if the key does not exist.
func (c *Cache[T]) Replace(key string, value T) bool {
	item := c.Peek(key)
	if item == nil {
		return false
	}
	c.Set(key, value, item.TTL())
	return true
}

// Extend updates the expiration time of an existing key to now+duration.
//
// Returns false if the key does not exist.
func (c *Cache[T]) Extend(key string, duration time.Duration) bool {
	item := c.Peek(key)
	if item == nil {
		return false
	}
	item.Extend(duration)
	return true
}

// Clear removes all items.
func (c *Cache[T]) Clear() {
	for _, s := range c.shards {
		s.clear()
	}
	atomic.StoreInt64(&c.size, 0)
}

// Range calls fn for each key/value. If fn returns false, iteration stops.
//
// The callback runs under shard read locks; keep it quick.
func (c *Cache[T]) Range(fn func(key string, value T) bool) {
	for _, shard := range c.shards {
		if !shard.forEach(fn) {
			return
		}
	}
}

// Filter returns all items whose key contains pattern.
//
// Filter touches access metadata for matched keys (because it calls Get).
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
