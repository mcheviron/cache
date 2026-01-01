package cache

// Weigher returns an application-defined weight for a key/value pair.
//
// When set on Config, the cache uses it to compute the cost of storing an item.
// This is important for heap-backed values (strings/slices/maps) where the
// default `reflect.TypeOf(value).Size()` underestimates true memory usage.
//
// The returned weight is in arbitrary "units". Eviction triggers when the sum
// of weights exceeds Config.MaxWeight.
//
// The cache calls Weigher frequently; keep it fast and allocation-free.
//
// Note: the value is passed as `any` so callers can type-assert.
// Example: `v, _ := value.(MyType)`.
type Weigher func(key string, value any) int

// Config controls cache behavior.
//
// Config is a plain struct (no builder pattern). Set fields directly and pass
// it to New. New calls Build() to validate/normalize fields.
//
// Build performs no allocations.
type Config struct {
	// Shards controls the number of independent shard maps.
	//
	// Must be a power of two. If invalid, Build() replaces it with a default.
	Shards int

	// MaxWeight is the cache capacity threshold.
	//
	// When the total stored weight exceeds MaxWeight, the cache evicts items.
	MaxWeight int

	// ItemsToPrune limits how many items can be evicted per Set call.
	ItemsToPrune int

	// SampleSize controls the number of candidates sampled during eviction.
	//
	// Larger values improve eviction quality but increase eviction cost.
	SampleSize int

	// EvictionPolicy controls which eviction strategy is used when over capacity.
	//
	// The default (zero value) is SampledLru.
	EvictionPolicy EvictionPolicy

	// Weigher is an optional custom weight function.
	//
	// If nil, the cache uses `len(key) + reflect.TypeOf(value).Size()`.
	Weigher Weigher

	// ExpirationPolicy controls how get/peek treats expired entries.
	//
	// The default (zero value) is TreatExpiredAsMiss.
	ExpirationPolicy ExpirationPolicy
}

type EvictionPolicy uint8

const (
	SampledLru EvictionPolicy = iota
	SampledLhd
)

// NewConfig returns a default configuration.
type ExpirationPolicy uint8

const (
	// TreatExpiredAsMiss returns nil for expired items.
	TreatExpiredAsMiss ExpirationPolicy = iota
	// ReturnExpired allows Get/Peek to return expired items.
	ReturnExpired
)

func NewConfig() Config {
	return Config{
		Shards:       16,
		MaxWeight:    5000,
		ItemsToPrune: 500,
		SampleSize:   32,
	}
}

// Build validates/normalizes config. It performs no allocations.
//
// If a field is invalid (e.g., shards not power-of-two), Build replaces it with
// a default.
func (c Config) Build() Config {
	out := c
	if out.Shards == 0 || out.Shards&(out.Shards-1) != 0 {
		out.Shards = 16
	}
	if out.MaxWeight <= 0 {
		out.MaxWeight = 5000
	}
	if out.ItemsToPrune <= 0 {
		out.ItemsToPrune = 500
	}
	if out.SampleSize <= 0 {
		out.SampleSize = 32
	}
	if out.EvictionPolicy != SampledLru && out.EvictionPolicy != SampledLhd {
		out.EvictionPolicy = SampledLru
	}
	if out.ExpirationPolicy != TreatExpiredAsMiss && out.ExpirationPolicy != ReturnExpired {
		out.ExpirationPolicy = TreatExpiredAsMiss
	}
	return out
}
