// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"time"
)

type Config struct {
	minInterval time.Duration
	maxInterval time.Duration
	expiry      uint
}

func WithMinInterval(minInterval time.Duration) func(*Config) {
	return func(cfg *Config) {
		cfg.minInterval = minInterval
	}
}

func WithMaxInterval(maxInterval time.Duration) func(*Config) {
	return func(cfg *Config) {
		cfg.maxInterval = maxInterval
	}
}
