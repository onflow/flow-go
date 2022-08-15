package synchronization

import (
	"time"

	core "github.com/onflow/flow-go/module/chainsync"
)

type Config struct {
	PollInterval time.Duration
	ScanInterval time.Duration
}

func DefaultConfig() *Config {
	scanInterval := 2 * time.Second
	pollInterval := time.Duration(core.DefaultQueuedHeightMultiplicity) * scanInterval
	return &Config{
		PollInterval: pollInterval,
		ScanInterval: scanInterval,
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
