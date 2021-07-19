// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"github.com/onflow/flow-go/state/protocol"
)

type Config struct {
	blockTimer protocol.BlockTimer
	// the max number of seals to be included in a block proposal
	maxSealCount      uint
	maxGuaranteeCount uint
	maxReceiptCount   uint
	expiry            uint
}

func WithBlockTimer(timer protocol.BlockTimer) func(*Config) {
	return func(cfg *Config) {
		cfg.blockTimer = timer
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
