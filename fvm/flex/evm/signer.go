package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

var DefaultBlockNumberForEVMRules = big.NewInt(1) // anything bigger than 0

// GetDefaultSigner returns the signer with default config
func GetDefaultSigner() types.Signer {
	cfg := NewConfig(WithBlockNumber(DefaultBlockNumberForEVMRules))
	return GetSigner(cfg)
}

// GetSigner a signer compatible with this env
func GetSigner(cfg *Config) types.Signer {
	signer := types.MakeSigner(cfg.ChainConfig, cfg.BlockContext.BlockNumber, cfg.BlockContext.Time)
	return signer
}
