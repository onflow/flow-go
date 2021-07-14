// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
)

type Config struct {
	blockTimestamp hotstuff.BlockTimestamp
	// the max number of seals to be included in a block proposal
	maxSealCount      uint
	maxGuaranteeCount uint
	maxReceiptCount   uint
	expiry            uint
}

func WithBlocktimer(timer hotstuff.Blocktimer) func(*Config) {
	return func(cfg *Config) {
		cfg.blockTimestamp = blockTimestamp
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
