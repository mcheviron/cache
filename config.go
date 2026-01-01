package cache

type Weigher func(key string, value any) int

type Config struct {
	Shards       int
	MaxSize      int
	ItemsToPrune int
	SampleSize   int
	Weigher      Weigher
}

func NewConfig() Config {
	return Config{
		Shards:       16,
		MaxSize:      5000,
		ItemsToPrune: 500,
		SampleSize:   32,
	}
}

// Build validates/normalizes config. It performs no allocations.
func (c Config) Build() Config {
	out := c
	if out.Shards == 0 || out.Shards&(out.Shards-1) != 0 {
		out.Shards = 16
	}
	if out.MaxSize <= 0 {
		out.MaxSize = 5000
	}
	if out.ItemsToPrune <= 0 {
		out.ItemsToPrune = 500
	}
	if out.SampleSize <= 0 {
		out.SampleSize = 32
	}
	return out
}
