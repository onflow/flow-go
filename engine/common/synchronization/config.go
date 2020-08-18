package synchronization

import (
	"time"
)

type Config struct {
	pollInterval time.Duration
	scanInterval time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		pollInterval: 8 * time.Second,
		scanInterval: 2 * time.Second,
	}
}

type OptionFunc func(*Config)

// WithPollInterval sets a custom interval at which we scan for poll items
func WithPollInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.pollInterval = interval
	}
}

// WithScanInterval sets a custom interval at which we scan for pending items
// and batch them for requesting.
func WithScanInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.scanInterval = interval
	}
}
