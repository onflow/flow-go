package evm

import (
	"github.com/ethereum/go-ethereum/core/types"
)

// GetDefaultSigner returns the signer with default config
func GetDefaultSigner() types.Signer {
	cfg := NewConfig(WithBlockNumber(BlockNumberForEVMRules))
	return GetSigner(cfg)
}

// GetSigner a signer compatible with this env
func GetSigner(cfg *Config) types.Signer {
	signer := types.MakeSigner(cfg.ChainConfig, cfg.BlockContext.BlockNumber, cfg.BlockContext.Time)
	return signer
}
