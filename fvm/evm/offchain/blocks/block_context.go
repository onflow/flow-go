package blocks

import (
	"fmt"

	gethCommon "github.com/onflow/go-ethereum/common"
	"github.com/onflow/go-ethereum/eth/tracers"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// NewBlockContext creates a new block context for the given chain ID and height.
// This is for use in offchain re-execution of transactions.
// It includes special casing for some historical block heights:
//   - On Mainnet and Testnet the block hash list was stuck in a loop of 256 block hashes until fixed.
//     https://github.com/onflow/flow-go/issues/6552
//   - The coinbase address was different on testnet until https://github.com/onflow/flow-evm-gateway/pull/491.
func NewBlockContext(
	chainID flow.ChainID,
	height uint64,
	timestamp uint64,
	getHashByHeight func(uint64) gethCommon.Hash,
	prevRandao gethCommon.Hash,
	tracer *tracers.Tracer,
) (types.BlockContext, error) {

	// coinbase address fix
	miner := types.CoinbaseAddress
	if chainID == flow.Testnet && height < coinbaseAddressChangeEVMHeightTestnet {
		miner = oldCoinbaseAddressTestnet
	}

	fmt.Printf("height: %v, miner: %v\n", height, miner)

	return types.BlockContext{
		ChainID:                types.EVMChainIDFromFlowChainID(chainID),
		BlockNumber:            height,
		BlockTimestamp:         timestamp,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
		DirectCallGasPrice:     types.DefaultDirectCallGasPrice,
		GasFeeCollector:        miner,
		GetHashFunc: func(n uint64) gethCommon.Hash {
			// For block heights greater than or equal to the current,
			// return an empty block hash.
			if n >= height {
				return gethCommon.Hash{}
			}
			// If the given block height, is more than 256 blocks
			// in the past, return an empty block hash.
			if height-n > 256 {
				return gethCommon.Hash{}
			}

			return getHashByHeight(n)

		},
		Random: prevRandao,
		Tracer: tracer,
	}, nil
}
