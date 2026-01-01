package cache

import (
	"hash/fnv"
	"math/bits"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"
)

// Cache is a sharded in-memory cache with TTL and size-based eviction.
//
// The cache stores items in shard maps for concurrent access. Each successful
// Get updates an access tick on the item. When the total weight exceeds
// Config.MaxWeight, eviction samples candidates across shards and evicts using
// the configured eviction policy (default: sampled LRU).
//
// TTL note:
//
// By default, expired items are treated as cache misses.
//
// Set Config.ExpirationPolicy=ReturnExpired to allow Get/Peek to return expired items.
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

	shards := cfg.Shards
	if shards < 1 || shards > int(^uint32(0)) {
		shards = 16
	}

	c := &Cache[T]{
		cfg: cfg,
		//nolint:gosec // shards is bounded to uint32; mask fits.
		shardMask: uint32(shards - 1),
		shards:    make([]*shard[T], shards),
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

	if c.cfg.ExpirationPolicy == TreatExpiredAsMiss && item.Expired() {
		return nil
	}

	item.recordHit(c.nextTick())
	return item
}

// Peek returns the item for key without updating access metadata.
//
// This is useful for "check without affecting eviction".
func (c *Cache[T]) Peek(key string) *Item[T] {
	item := c.getShard(key).get(key)
	if item == nil {
		return nil
	}
	if c.cfg.ExpirationPolicy == TreatExpiredAsMiss && item.Expired() {
		return nil
	}
	return item
}

func (c *Cache[T]) peekRaw(key string) *Item[T] {
	return c.getShard(key).get(key)
}

// Set inserts or updates a key with a TTL.
//
// If key existed, the previous item is replaced.
func (c *Cache[T]) Set(key string, value T, duration time.Duration) {
	expires := time.Now().Add(duration).UnixNano()
	weight := c.weigher(key, value)
	item := newItem(key, value, expires, weight)
	tick := c.nextTick()
	item.setCreatedTick(tick)
	item.touch(tick)

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
	item := c.peekRaw(key)
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
	item := c.peekRaw(key)
	if item == nil {
		return false
	}
	item.Extend(duration)
	item.touch(c.nextTick())
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
	// Deterministic path for small caches: scan everything.
	if c.ItemCount() <= c.cfg.SampleSize {
		switch c.cfg.EvictionPolicy {
		case SampledLhd:
			return c.scanLeastHitDense()
		default:
			return c.scanOldest()
		}
	}

	switch c.cfg.EvictionPolicy {
	case SampledLhd:
		if best := c.pickSampledLhd(); best != nil {
			return best
		}
		return c.scanLeastHitDense()
	default:
		if best := c.pickSampledLru(); best != nil {
			return best
		}
		return c.scanOldest()
	}
}

func (c *Cache[T]) pickSampledLru() *Item[T] {
	var best *Item[T]
	bestTick := uint64(^uint64(0))

	for i := 0; i < c.cfg.SampleSize; i++ {
		//nolint:gosec // non-crypto randomness is fine for eviction sampling
		shard := c.shards[rand.IntN(len(c.shards))]
		//nolint:gosec // non-crypto randomness is fine for eviction sampling
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

	return best
}

func (c *Cache[T]) pickSampledLhd() *Item[T] {
	nowTick := atomic.LoadUint64(&c.accessClock)
	if nowTick == 0 {
		nowTick = 1
	}

	var best *Item[T]

	for i := 0; i < c.cfg.SampleSize; i++ {
		//nolint:gosec // non-crypto randomness is fine for eviction sampling
		shard := c.shards[rand.IntN(len(c.shards))]
		//nolint:gosec // non-crypto randomness is fine for eviction sampling
		item := shard.sampleNth(rand.Uint64())
		if item == nil {
			continue
		}
		if best == nil || c.lhdIsBetterCandidate(item, best, nowTick) {
			best = item
		}
	}

	return best
}

func (c *Cache[T]) scanOldest() *Item[T] {
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

func (c *Cache[T]) scanLeastHitDense() *Item[T] {
	nowTick := atomic.LoadUint64(&c.accessClock)
	if nowTick == 0 {
		nowTick = 1
	}

	var best *Item[T]
	for _, shard := range c.shards {
		shard.RLock()
		for _, item := range shard.store {
			if best == nil || c.lhdIsBetterCandidate(item, best, nowTick) {
				best = item
			}
		}
		shard.RUnlock()
	}
	return best
}

func (c *Cache[T]) lhdIsBetterCandidate(candidate, current *Item[T], nowTick uint64) bool {
	candHits, candDenom := c.lhdStats(candidate, nowTick)
	curHits, curDenom := c.lhdStats(current, nowTick)

	candHi, candLo := bits.Mul64(candHits, curDenom)
	curHi, curLo := bits.Mul64(curHits, candDenom)

	if candHi != curHi {
		// Smaller hit density is a better eviction candidate.
		return candHi < curHi
	}
	if candLo != curLo {
		// Smaller hit density is a better eviction candidate.
		return candLo < curLo
	}

	// Prefer evicting heavier items when densities are equal.
	if candidate.weight != current.weight {
		return candidate.weight > current.weight
	}

	// Finally, fall back to LRU.
	return candidate.lastAccessTick() < current.lastAccessTick()
}

func (c *Cache[T]) lhdStats(item *Item[T], nowTick uint64) (hits uint64, denom uint64) {
	created := item.createdTickValue()
	age := uint64(0)
	if nowTick > created {
		age = nowTick - created
	}
	age = max(age, 1)

	weight := uint64(0)
	if item.weight > 0 {
		//nolint:gosec // weight is validated to be positive before conversion
		weight = uint64(item.weight)
	}
	weight = max(weight, 1)

	d := age * weight
	d = max(d, 1)

	return item.hitsValue(), d
}

func (c *Cache[T]) nextTick() uint64 {
	return atomic.AddUint64(&c.accessClock, 1)
}
