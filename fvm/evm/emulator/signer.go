package emulator

import (
	"math/big"

	"github.com/onflow/go-ethereum/core/types"
)

var defaultBlockNumberForEVMRules = big.NewInt(1) // anything bigger than 0

// GetDefaultSigner returns a signer which is compatible with the default config
func GetDefaultSigner() types.Signer {
	cfg := NewConfig(WithBlockNumber(defaultBlockNumberForEVMRules))
	return GetSigner(cfg)
}

// GetSigner returns a evm signer object that is compatible with the given config
//
// Despite its misleading name, signer encapsulates transaction signature validation functionality and
// does not provide actual signing functionality.
// we kept the same name to be consistent with EVM naming.
func GetSigner(cfg *Config) types.Signer {
	return types.MakeSigner(
		cfg.ChainConfig,
		cfg.BlockContext.BlockNumber,
		cfg.BlockContext.Time,
	)
}
