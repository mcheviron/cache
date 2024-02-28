package cache

import (
	"hash/fnv"
	"time"
)

type freeList[T any] []*Item[T]
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

func (f *freeList[T]) get() *Item[T] {
	if len(*f) == 0 {
		return nil
	}
	i := (*f)[len(*f)-1]
	*f = (*f)[:len(*f)-1]
	return i
}

func New[T any](config *Config) *Cache[T] {
	c := &Cache[T]{
		queue:       newQueue[*Item[T]](),
		Config:      config,
		shardMask:   uint32(config.shards) - 1,
		shards:      make([]*shard[T], config.shards),
		deletables:  make(chan *Item[T], config.deleteBuffer),
		promotables: make(chan *Item[T], config.promoteBuffer),
		freeList:    make([]*Item[T], 0, int(config.freeListSize*float32(config.maxSize))),
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
	if len(c.freeList) > 0 {
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
	for _, b := range c.shards {
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

	c.size += item.size
	item.node = c.queue.pushToFront(item)
	return true
}

func (c *Cache[T]) doDelete(item *Item[T]) {
	if item.node != nil {
		if len(c.freeList) < cap(c.freeList) {
			c.freeList = append(c.freeList, item)
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
		if len(c.freeList) < cap(c.freeList) {
			c.freeList = append(c.freeList, item)
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
