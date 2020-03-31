// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"time"
)

type Config struct {
	chainID     string
	minInterval time.Duration
	maxInterval time.Duration
}

func WithChainID(chainID string) func(*Config) {
	return func(cfg *Config) {
		cfg.chainID = chainID
	}
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
