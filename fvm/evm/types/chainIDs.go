package types

import (
	"math/big"

	"github.com/onflow/flow-go/model/flow"
)

var (
	FlowEVMTestnetChainID = big.NewInt(646)
	FlowEVMMainnetChainID = big.NewInt(747)
)

func EVMChainIDFromFlowChainID(flowChainID flow.ChainID) *big.Int {
	// default evm chain ID is testnet
	chainID := FlowEVMTestnetChainID
	if flowChainID == flow.Mainnet {
		chainID = FlowEVMMainnetChainID
	}
	return chainID
}
