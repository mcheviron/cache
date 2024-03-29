package cache

import (
	"hash/fnv"
	"strings"
	"time"
)

type Cache[T any] struct {
	*Config
	queue       *queue[*Item[T]]
	shards      []*shard[T]
	size        int
	shardMask   uint32
	deletables  chan *Item[T]
	promotables chan *Item[T]
	freeList    freeList[T]
}

func New[T any](config *Config) *Cache[T] {
	c := &Cache[T]{
		queue:       newQueue[*Item[T]](),
		Config:      config,
		shardMask:   uint32(config.shards) - 1,
		shards:      make([]*shard[T], config.shards),
		deletables:  make(chan *Item[T], config.deleteBuffer),
		promotables: make(chan *Item[T], config.promoteBuffer),
		freeList:    newFreeList[T](config.maxSize / config.freeListSize),
	}
	for i := range c.shards {
		c.shards[i] = &shard[T]{
			store: make(map[string]*Item[T]),
		}
	}
	go c.worker()
	return c
}

func (c *Cache[T]) ItemCount() int {
	count := 0
	for _, b := range c.shards {
		count += b.itemCount()
	}
	return count
}

func (c *Cache[T]) Get(key string) *Item[T] {
	item := c.getShard(key).get(key)
	if item == nil {
		return nil
	}
	if !item.Expired() {
		select {
		case c.promotables <- item:
		default:
		}
	}
	return item
}

func (c *Cache[T]) Set(key string, value T, duration time.Duration) {
	var newItem *Item[T]
	if c.freeList.len() > 0 {
		newItem = c.freeList.get()
		newItem.reset(key, value, time.Now().Add(duration).UnixNano())
	} else {
		new, old := c.getShard(key).set(key, value, duration)
		if old != nil {
			c.deletables <- old
		}
		newItem = new
	}
	c.promotables <- newItem
}

func (c *Cache[T]) Delete(key string) {
	if item := c.getShard(key).delete(key); item != nil {
		c.deletables <- item
	}
}

func (c *Cache[T]) Replace(key string, value T) bool {
	item := c.getShard(key).get(key)
	if item == nil {
		return false
	}
	c.getShard(key).set(key, value, item.TTL())
	return true
}

func (c *Cache[T]) Extend(key string, duration time.Duration) bool {
	item := c.getShard(key).get(key)
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
}

func (c *Cache[T]) Range(fn func(key string, value T) bool) {
	for _, shard := range c.shards {
		if !shard.forEach(fn) {
			return
		}
	}
}

func (s *shard[T]) forEach(fn func(key string, value T) bool) bool {
	for _, item := range s.store {
		if !fn(item.key, item.value) {
			return false
		}
	}
	return true
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

func (c *Cache[T]) doPromote(item *Item[T]) bool {
	if item.promotions < 0 {
		return false
	}

	if item.node != nil {
		if item.shouldPromote(int32(c.getsPerPromote)) {
			c.queue.moveToFront(item.node)
			item.promotions = 0
		}
		return false
	}

	c.size += item.size
	item.node = c.queue.pushToFront(item)
	return true
}

func (c *Cache[T]) doDelete(item *Item[T]) {
	if item.node != nil {
		if c.freeList.len() < c.freeList.cap() {
			c.freeList.put(item)
		} else {
			c.queue.remove(item.node)
			item.node = nil
			item.promotions = -1
		}

		c.size -= item.size
	} else {
		item.promotions = -1
	}
}

func (c *Cache[T]) getShard(key string) *shard[T] {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()&c.shardMask]
}

func (c *Cache[T]) worker() {
	promoteItem := func(item *Item[T]) {
		if c.doPromote(item) && c.size > c.maxSize {
			c.gc()
		}
	}

	for {
		select {
		case item := <-c.deletables:
			c.doDelete(item)
		case item := <-c.promotables:
			promoteItem(item)
		}
	}
}

func (c *Cache[T]) gc() {
	node := c.queue.tail
	itemsToPrune := c.itemsToPrune

	if min := c.size - c.maxSize; min > itemsToPrune {
		itemsToPrune = min
	}
	for range itemsToPrune {
		if node == nil {
			break
		}

		prev := node.prev
		item := node.value
		if c.freeList.len() < c.freeList.cap() {
			c.freeList.put(item)
			c.getShard(item.key).delete(item.key)
			c.size -= item.size
		} else {
			c.getShard(item.key).delete(item.key)
			c.size -= item.size
			c.queue.remove(node)
			item.node = nil
			item.promotions = -1
		}
		node = prev
	}
}
