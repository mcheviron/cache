package cache

type Config struct {
	shards         int
	maxSize        int
	itemsToPrune   int
	deleteBuffer   int
	promoteBuffer  int
	getsPerPromote int
	byBytes        bool
	byCount        bool
	freeListSize   int
}

func NewConfig() *Config {
	return &Config{
		shards:        16,
		maxSize:       5000,
		byBytes:       true,
		byCount:       false,
		itemsToPrune:  500,
		deleteBuffer:  1024,
		promoteBuffer: 1024,
		freeListSize:  10,
	}
}

// Shards sets the number of shards in the configuration.
// It takes an integer count as a parameter and updates the configuration's shard count.
// If the count is not a power of 2, the configuration remains unchanged.
func (c *Config) Shards(count int) *Config {
	if count == 0 || count&(count-1) != 0 {
		return c
	}
	c.shards = count
	return c
}

// MaxSize sets the maximum size for the cache.
// It takes an integer value representing the maximum size in bytes (or count).
func (c *Config) MaxSize(size int) *Config {
	c.maxSize = size
	return c
}

// ByBytes sets the cache eviction strategy to be based on the number of bytes.
// If this is set to true, the cache will be bytes-based instead of count-based.
// The maxSize parameter represents the maximum number of bytes that the cache can store.
// When the cache reaches its maximum capacity, the least recently used items will be evicted
func (c *Config) ByBytes() *Config {
	c.byBytes = true
	c.byCount = false
	return c
}

// ByCount sets the cache capacity to be managed by the number of items in the cache.
// If this is set to true, the cache will be count-based instead of bytes-based.
// The maxSize parameter represents the maximum number of objects that the cache can store.
// It is recommended to set an appropriate maxSize value when using ByCount, as the default value may be too big.
func (c *Config) ByCount() *Config {
	c.byBytes = false
	c.byCount = true
	return c
}

// ItemsToPrune sets the number of items to prune in the cache.
// This determines the number of items that will be pruned from the cache once the maxSize is hit.
func (c *Config) ItemsToPrune(count int) *Config {
	c.itemsToPrune = count
	return c
}

// DeleteBuffer sets the size of the delete buffer in the Config struct.
// The delete buffer is used to store deleted items temporarily before they are permanently removed.
// The size parameter specifies the maximum number of items that can be stored in the delete buffer.
func (c *Config) DeleteBuffer(size int) *Config {
	c.deleteBuffer = size
	return c
}

func (c *Config) PromoteBuffer(size int) *Config {
	c.promoteBuffer = size
	return c
}

// FreeListSize sets the size of the free list as a percentage of the actual size, the max size.
// The size parameter should be a value between 0 and 100, representing the percentage.
// If the size is less than 0 or greater than 100, the method does nothing and returns the current configuration.
// Returns the updated Config object.
func (c *Config) FreeListSize(size int) *Config {
	if size < 0 || size > 100 {
		return c
	}
	c.freeListSize = size
	return c
}
