package cache

import (
	"hash/fnv"
	"time"
)

type Cache[T any] struct {
	*Config
	queue       *queue[*Item[T]]
	buckets     []*bucket[T]
	size        int
	bucketMask  uint32
	deletables  chan *Item[T]
	promotables chan *Item[T]
}

func New[T any](config *Config) *Cache[T] {
	c := &Cache[T]{
		queue:       newQueue[*Item[T]](),
		Config:      config,
		bucketMask:  uint32(config.buckets) - 1,
		buckets:     make([]*bucket[T], config.buckets),
		deletables:  make(chan *Item[T], config.deleteBuffer),
		promotables: make(chan *Item[T], config.promoteBuffer),
	}
	for i := range c.buckets {
		c.buckets[i] = &bucket[T]{
			store: make(map[string]*Item[T]),
		}
	}
	go c.worker()
	return c
}

func (c *Cache[T]) ItemCount() int {
	count := 0
	for _, b := range c.buckets {
		count += b.itemCount()
	}
	return count
}

func (c *Cache[T]) Get(key string) *Item[T] {
	item := c.bucket(key).get(key)
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
	c.bucket(key).set(key, value, duration)
}

func (c *Cache[T]) Delete(key string) {
	c.bucket(key).delete(key)
}

func (c *Cache[T]) Replace(key string, value T) bool {
	item := c.bucket(key).get(key)
	if item == nil {
		return false
	}
	c.bucket(key).set(key, value, item.TTL())
	return true
}

func (c *Cache[T]) Extend(key string, duration time.Duration) bool {
	item := c.bucket(key).get(key)
	if item == nil {
		return false
	}
	item.Extend(duration)
	return true
}

func (c *Cache[T]) Clear() {
	for _, b := range c.buckets {
		b.clear()
	}
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

	c.size += int(item.size)
	item.node = c.queue.pushToFront(item)
	return true
}

func (c *Cache[T]) doDelete(item *Item[T]) {
	if item.node != nil {
		c.queue.remove(item.node)
		item.node = nil
		c.size -= item.size
	} else {
		item.promotions = -1
	}
}

func (c *Cache[T]) bucket(key string) *bucket[T] {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.buckets[h.Sum32()&c.bucketMask]
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
		c.bucket(item.key).delete(item.key)
		c.size -= item.size
		c.queue.remove(node)
		item.node = nil
		item.promotions = -1
		node = prev
	}
}
