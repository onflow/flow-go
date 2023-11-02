package emulator

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

var DefaultBlockNumberForEVMRules = big.NewInt(1) // anything bigger than 0

// GetDefaultSigner returns a signer which is compatible with the default config
func GetDefaultSigner() types.Signer {
	cfg := NewConfig(WithBlockNumber(DefaultBlockNumberForEVMRules))
	return GetSigner(cfg)
}

// GetSigner returns a signer that is compatible with the given config
func GetSigner(cfg *Config) types.Signer {
	signer := types.MakeSigner(cfg.ChainConfig, cfg.BlockContext.BlockNumber, cfg.BlockContext.Time)
	return signer
}
