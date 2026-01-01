// Package cache provides a sharded in-memory cache with TTL and size-based eviction.
//
// Key properties:
//
//   - Sharded map storage for concurrent access.
//   - TTL per item (expiration is not enforced automatically).
//     Get/Peek can return expired items; call Item.Expired() if needed.
//   - Size-based eviction using sampled-by-access eviction.
//     When the cache exceeds Config.MaxWeight, it samples candidates across shards
//     and evicts the item with the oldest access tick.
//   - Optional Config.Weigher for accurate weighting of heap-backed values.
//
// # Configuration
//
// Config is a plain struct (no builder pattern). Set the fields you care about
// and pass it to New. Internally, New calls Config.Build() to validate and
// normalize fields; Build performs no allocations.
//
// # Concurrency
//
// Cache operations are safe for concurrent use.
// Items returned from Get/Peek are pointers; the cache may later delete/evict a
// key, but the pointed-to Item remains valid (it is not freed).
package cache
