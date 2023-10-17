package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// tx type 255 is used for direct calls from bridged accounts
var DirectCallTxType = uint8(255)

type Result struct {
	Failed                  bool
	TxType                  uint8
	GasConsumed             uint64
	StateRootHash           common.Hash
	DeployedContractAddress FlexAddress
	ReturnedValue           []byte
	Logs                    []*types.Log
}

// Receipt constructs an EVM receipt
func (res *Result) Receipt() *types.ReceiptForStorage {
	receipt := &types.Receipt{
		Type:              res.TxType,
		PostState:         res.StateRootHash[:],
		CumulativeGasUsed: res.GasConsumed, // TODO: update to capture cumulative
		Logs:              res.Logs,
		ContractAddress:   res.DeployedContractAddress.ToCommon(),
	}
	if res.Failed {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}

	// Note: bloom can be reconstructed from receipt logs thats why we don't populate it here.
	// receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	return (*types.ReceiptForStorage)(receipt)
}
