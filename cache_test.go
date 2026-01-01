package cache_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/mcheviron/cache"
)

func TestCacheItemCount(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	count := cache.ItemCount()

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

	if item == nil || item.Expired() == false {
		t.Errorf("Expected item to be expired")
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

	if item.TTL() >= time.Second {
		t.Errorf("Expected item TTL to be less than 1 second, got %s", item.TTL())
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
	if item.TTL() < time.Minute-elapsed {
		t.Errorf("Expected item TTL to be less than 1 minute, got %s", item.TTL())
	}
}

func TestCacheExtendNonExistingItem(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	extended := cache.Extend("key1", time.Minute)

	if extended {
		t.Errorf("Expected item to not be extended")
	}
}
func TestCacheClear(t *testing.T) {
	cache := cache.New[string](cache.NewConfig())

	cache.Set("key1", "value1", time.Second)
	cache.Set("key2", "value2", time.Second)
	cache.Set("key3", "value3", time.Second)

	cache.Clear()

	count := cache.ItemCount()

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
