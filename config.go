package cache

type Weigher func(key string, value any) int

type Config struct {
	Shards       int
	MaxWeight    int
	ItemsToPrune int
	SampleSize   int
	Weigher      Weigher
}

func NewConfig() Config {
	return Config{
		Shards:       16,
		MaxWeight:    5000,
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
	if out.MaxWeight <= 0 {
		out.MaxWeight = 5000
	}
	if out.ItemsToPrune <= 0 {
		out.ItemsToPrune = 500
	}
	if out.SampleSize <= 0 {
		out.SampleSize = 32
	}
	return out
}
