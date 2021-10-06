package synchronization

import (
	"time"
)

type Config struct {
	PollInterval time.Duration
	ScanInterval time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		PollInterval: 8 * time.Second,
		ScanInterval: 2 * time.Second,
	}
}

type OptionFunc func(*Config)

// WithPollInterval sets a custom interval at which we scan for poll items
func WithPollInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.PollInterval = interval
	}
}

// WithScanInterval sets a custom interval at which we scan for pending items
// and batch them for requesting.
func WithScanInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.ScanInterval = interval
	}
}
