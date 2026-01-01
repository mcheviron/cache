package cache_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/mcheviron/cache"
)

func ExampleCache_basic() {
	c := cache.New[string](cache.NewConfig())
	c.Set("k", "v", time.Minute)

	item := c.Get("k")
	fmt.Println(item.Value())
	// Output: v
}

func ExampleCache_withWeigher() {
	cfg := cache.NewConfig()
	cfg.MaxWeight = 1024
	cfg.Weigher = func(key string, value any) int {
		// Example: count the key plus string bytes.
		s, _ := value.(string)
		return len(key) + len(s)
	}

	c := cache.New[string](cfg)
	c.Set("k", "hello", time.Minute)

	fmt.Println(c.Get("k").Value())
	// Output: hello
}

func ExampleCache_peekVsGet() {
	c := cache.New[string](cache.NewConfig())
	c.Set("k", "v", time.Minute)

	_ = c.Peek("k")
	_ = c.Get("k")

	fmt.Println("ok")
	// Output: ok
}

func TestCacheItemCount(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	count := cache.Len()

	if count != 3 {
		t.Errorf("Expected item count to be 3, got %d", count)
	}
}

func TestNewCache(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	if cache == nil {
		t.Errorf("Expected cache to be not nil")
	}
}
func TestCacheGet(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)

	item := cache.Get("key1")

	if item == nil {
		t.Errorf("Expected item to be not nil")
	}

	if item.Value() != "value1" {
		t.Errorf("Expected item value to be 'value1', got '%s'", item.Value())
	}
}

func TestCacheGetExpiredItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Nanosecond)

	time.Sleep(time.Millisecond)

	item := cache.Get("key1")

	if item != nil {
		t.Errorf("Expected expired item to be treated as miss")
	}
}
func TestCacheDelete(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	cache.Delete("key2")

	item := cache.Get("key2")

	if item != nil {
		t.Errorf("Expected item to be nil after deletion")
	}
}

func TestCacheDeleteNonExistingKey(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)

	cache.Delete("key3")

	item := cache.Get("key3")

	if item != nil {
		t.Errorf("Expected item to be nil for non-existing key")
	}
}
func TestCacheReplaceExistingItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)

	replaced := cache.Replace("key1", "newvalue")

	if !replaced {
		t.Errorf("Expected item to be replaced")
	}

	item := cache.Get("key1")

	if item.Value() != "newvalue" {
		t.Errorf("Expected item value to be 'newvalue', got '%s'", item.Value())
	}

	// TTL preservation is approximate; allow equality and small jitter.
	if item.TTL() > time.Second {
		t.Errorf("Expected item TTL to be <= 1 second, got %s", item.TTL())
	}
}

func TestCacheReplaceNonExistingItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	replaced := cache.Replace("key1", "value1")

	if replaced {
		t.Errorf("Expected item to not be replaced")
	}

	item := cache.Get("key1")

	if item != nil {
		t.Errorf("Expected item to be nil for non-existing key")
	}
}
func TestCacheExtendExistingItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)

	start := time.Now()
	extended := cache.Extend("key1", time.Minute)

	if !extended {
		t.Errorf("Expected item to be extended")
	}

	item := cache.Get("key1")
	elapsed := time.Since(start)
	// Allow a small scheduling/measurement jitter window.
	minExpected := time.Minute - elapsed - 10*time.Millisecond
	if item.TTL() < minExpected {
		t.Errorf("Expected item TTL to be near 1 minute, got %s", item.TTL())
	}
}

func TestCacheExtendNonExistingItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	extended := cache.Extend("key1", time.Minute)

	if extended {
		t.Errorf("Expected item to not be extended")
	}
}
func TestCacheEvictsLruWhenOverWeight(t *testing.T) {
	cfg := cache.NewConfig()
	cfg.Shards = 2
	cfg.MaxWeight = 30
	cfg.ItemsToPrune = 10
	cfg.SampleSize = 1024
	cfg.EvictionPolicy = cache.SampledLru
	cfg.Weigher = func(key string, value any) int {
		return 10
	}

	c := cache.New[int](cfg)

	c.Set("k1", 1, time.Minute)
	c.Set("k2", 2, time.Minute)
	c.Set("k3", 3, time.Minute)

	// Touch k2 so k1 becomes LRU.
	if c.Get("k2") == nil {
		t.Fatalf("expected k2")
	}

	// Insert enough to force eviction.
	c.Set("k4", 4, time.Minute)

	if c.Peek("k1") != nil {
		t.Fatalf("expected k1 to be evicted")
	}
}

func TestCacheEvictsLhdWhenOverWeight(t *testing.T) {
	cfg := cache.NewConfig()
	cfg.Shards = 2
	cfg.MaxWeight = 20
	cfg.ItemsToPrune = 10
	cfg.SampleSize = 1024
	cfg.EvictionPolicy = cache.SampledLhd
	cfg.Weigher = func(key string, value any) int {
		return 10
	}

	c := cache.New[int](cfg)

	c.Set("k1", 1, time.Minute)
	c.Set("k2", 2, time.Minute)

	for i := 0; i < 10; i++ {
		if c.Get("k1") == nil {
			t.Fatalf("expected k1")
		}
	}

	c.Set("k3", 3, time.Minute)

	if c.Peek("k2") != nil {
		t.Fatalf("expected k2 to be evicted")
	}
	if c.Peek("k1") == nil {
		t.Fatalf("expected k1 to remain")
	}
	if c.Peek("k3") == nil {
		t.Fatalf("expected k3 to remain")
	}
}

func TestCacheClear(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	cache.Clear()

	count := cache.Len()

	if count != 0 {
		t.Errorf("Expected item count to be 0 after clearing cache, got %d", count)
	}
}
func TestForEach(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	keys := make([]string, 0)
	values := make([]string, 0)

	fn := func(key string, value string) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	}

	cache.Range(fn)

	expectedKeys := []string{"key1", "key2", "key3"}
	expectedValues := []string{"value1", "value2", "value3"}

	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Errorf("Expected keys to be %v, got %v", expectedKeys, keys)
	}

	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("Expected values to be %v, got %v", expectedValues, values)
	}
}
func TestCacheFilter(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	filtered := cache.Filter("key")

	expectedKeys := []string{"key1", "key2", "key3"}
	expectedValues := []string{"value1", "value2", "value3"}

	if len(filtered) != len(expectedKeys) {
		t.Errorf("Expected filtered items count to be %d, got %d", len(expectedKeys), len(filtered))
	}

	for i, item := range filtered {
		if item.Key() != expectedKeys[i] {
			t.Errorf("Expected filtered item key to be '%s', got '%s'", expectedKeys[i], item.Key())
		}

		if item.Value() != expectedValues[i] {
			t.Errorf("Expected filtered item value to be '%s', got '%s'", expectedValues[i], item.Value())
		}
	}
}
func TestCacheFilterEmptyResult(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	filtered := cache.Filter("nonexistent")

	if len(filtered) != 0 {
		t.Errorf("Expected filtered items count to be 0, got %d", len(filtered))
	}
}
