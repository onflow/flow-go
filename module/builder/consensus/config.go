// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"time"
)

type Config struct {
	minInterval time.Duration
	maxInterval time.Duration
	// the max number of seals to be included in a block proposal
	maxSealCount      uint
	maxGuaranteeCount uint
	maxReceiptCount   uint
	expiry            uint
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

func WithMaxSealCount(maxSealCount uint) func(*Config) {
	return func(cfg *Config) {
		cfg.maxSealCount = maxSealCount
	}
}

func WithMaxGuaranteeCount(maxGuaranteeCount uint) func(*Config) {
	return func(cfg *Config) {
		cfg.maxGuaranteeCount = maxGuaranteeCount
	}
}

func WithMaxReceiptCount(maxReceiptCount uint) func(*Config) {
	return func(cfg *Config) {
		cfg.maxReceiptCount = maxReceiptCount
	}
}
