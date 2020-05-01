// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"time"
)

type Config struct {
	minInterval  time.Duration
	maxInterval  time.Duration
	expiryBlocks uint64
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

func WithExpiryBlocks(expiryBlocks uint64) func(*Config) {
	return func(cfg *Config) {
		cfg.expiryBlocks = expiryBlocks
	}
}
