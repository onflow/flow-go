package emulator

import (
	gethTypes "github.com/onflow/go-ethereum/core/types"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// GetDefaultSigner returns a signer which is compatible with the default config
func GetDefaultSigner() gethTypes.Signer {
	cfg := types.NewConfig(
		types.WithBlockNumber(types.BlockNumberForEVMRules),
		types.WithBlockTime(types.BlockTimeForEVMRules))
	return GetSigner(cfg)
}

// GetSigner returns a evm signer object that is compatible with the given config
//
// Despite its misleading name, signer encapsulates transaction signature validation functionality and
// does not provide actual signing functionality.
// we kept the same name to be consistent with EVM naming.
func GetSigner(cfg *types.Config) gethTypes.Signer {
	return gethTypes.MakeSigner(
		cfg.ChainConfig,
		cfg.BlockContext.BlockNumber,
		cfg.BlockContext.Time,
	)
}
