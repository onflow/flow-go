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
	VMError error
	// type of transaction defined by the evm package
	// see DirectCallTxType as extra type we added type for direct calls.
	TxType uint8
	// total gas consumed during an opeartion
	GasConsumed uint64
	// the address where the contract is deployed (if any)
	DeployedContractAddress Address
	// returned value from a function call
	ReturnedValue []byte
	// EVM logs (events that are emited by evm)
	Logs []*gethTypes.Log
	// TX hash holdes the cached value of tx hash
	TxHash gethCommon.Hash
}

func (res *Result) Failed() bool {
	return res.VMError != nil
}

func (res *Result) VMErrorString() string {
	if res.VMError != nil {
		return res.VMError.Error()
	}
	return ""
}

// Receipt constructs an EVM-style receipt
// can be used by json-rpc and other integration to be returned.
func (res *Result) Receipt() *gethTypes.ReceiptForStorage {
	receipt := &gethTypes.Receipt{
		Type:              res.TxType,
		CumulativeGasUsed: res.GasConsumed, // TODO: update to capture cumulative
		Logs:              res.Logs,
		ContractAddress:   res.DeployedContractAddress.ToCommon(),
	}
	if res.Failed() {
		receipt.Status = gethTypes.ReceiptStatusFailed
	} else {
		receipt.Status = gethTypes.ReceiptStatusSuccessful
	}

	receipt.Bloom = gethTypes.CreateBloom(gethTypes.Receipts{receipt})
	return (*gethTypes.ReceiptForStorage)(receipt)
}

// Status captures the status of an interaction to the emulator
type Status uint8

var (
	StatusUnknown Status = 0
	// StatusInvalid shows that the transaction was not a valid
	// transaction and rejected to be executed and included in any block.
	StatusInvalid Status = 1
	// StatusFailed shows that the transaction has been executed,
	// but the output of the execution was an error
	// for this case a block is formed and receipts are available
	StatusFailed Status = 2
	// StatusFailed shows that the transaction has been executed and the execution has returned success
	// for this case a block is formed and receipts are available
	StatusSuccessful Status = 3
)

// ResultSummary summerizes the outcome of a EVM call or tx run
type ResultSummary struct {
	Status                  Status
	ErrorCode               ErrorCode
	GasConsumed             uint64
	DeployedContractAddress Address
	ReturnedValue           Data
}

func NewResultSummary(res *Result, validationError error) *ResultSummary {
	rs := &ResultSummary{}

	if res != nil {
		rs.GasConsumed = res.GasConsumed
		rs.DeployedContractAddress = res.DeployedContractAddress
		rs.ReturnedValue = res.ReturnedValue
	}

	if validationError != nil {
		rs.ErrorCode = ValidationErrorCode(validationError)
		rs.Status = StatusInvalid
		return rs
	}

	if res.VMError != nil {
		rs.ErrorCode = ExecutionErrorCode(res.VMError)
		rs.Status = StatusFailed
		return rs
	}

	rs.Status = StatusSuccessful
	return rs
}
