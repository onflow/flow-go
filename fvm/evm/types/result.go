package types

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// Result captures the result of an interaction to the emulator
// it could be the out put of a direct call or output of running an
// evm transaction.
// Its more comprehensive than typical evm receipt, usually
// the receipt generation requires some extra calculation (e.g. Deployed contract address)
// but we take a different apporach here and include more data so that
// it requires less work for anyone who tracks and consume results.
type Result struct {
	// a boolean that is set to false if the execution has failed (non-fatal)
	Failed bool
	// type of transaction defined by the evm package
	// see DirectCallTxType as extra type we added type for direct calls.
	TxType uint8
	// total gas consumed during an opeartion
	GasConsumed uint64
	// the root hash of the state after execution
	StateRootHash gethCommon.Hash
	// the address where the contract is deployed (if any)
	DeployedContractAddress Address
	// returned value from a function call
	ReturnedValue []byte
	// EVM logs (events that are emited by evm)
	Logs []*gethTypes.Log
}

// Receipt constructs an EVM-style receipt
// can be used by json-rpc and other integration to be returned.
func (res *Result) Receipt() *gethTypes.ReceiptForStorage {
	receipt := &gethTypes.Receipt{
		Type:              res.TxType,
		PostState:         res.StateRootHash[:],
		CumulativeGasUsed: res.GasConsumed, // TODO: update to capture cumulative
		Logs:              res.Logs,
		ContractAddress:   res.DeployedContractAddress.ToCommon(),
	}
	if res.Failed {
		receipt.Status = gethTypes.ReceiptStatusFailed
	} else {
		receipt.Status = gethTypes.ReceiptStatusSuccessful
	}

	receipt.Bloom = gethTypes.CreateBloom(gethTypes.Receipts{receipt})
	return (*gethTypes.ReceiptForStorage)(receipt)
}
