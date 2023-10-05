package evm

import "github.com/ethereum/go-ethereum/core/types"

// GetSigner a signer compatible with this env
func GetSigner() types.Signer {
	cfg := NewConfig()
	signer := types.MakeSigner(cfg.ChainConfig, BlockNumberForEVMRules, cfg.BlockContext.Time)
	return signer
}
