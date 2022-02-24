package synchronization

import (
	"time"
)

type Config struct {
	SyncHeightInterval time.Duration
	RequestInterval    time.Duration
	MaxRequestSize     uint64
}

const DefaultSyncHeightInterval = 8 * time.Second
const DefaultRequestInterval = 2 * time.Second
const DefaultMaxRequestSize = 64

type OptionFunc func(*Config)

// WithSyncHeightInterval sets a custom interval at which we send Sync Height requests
func WithSyncHeightInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.SyncHeightInterval = interval
	}
}

// WithRequestInterval sets a custom interval at which we check for requestable items
// from the Sync Core and send requests for them
func WithRequestInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.RequestInterval = interval
	}
}

// WithRequestInterval sets the maximum size of a request / response
func WithMaxRequestSize(size uint64) OptionFunc {
	return func(cfg *Config) {
		cfg.MaxRequestSize = size
	}
}
