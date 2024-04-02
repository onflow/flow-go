package types

import (
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
)

// InvalidTransactionGasCost is a gas cost we charge when
// a transaction or call fails at validation step.
// in typical evm environment this doesn't exist given
// if a transaction is invalid it won't be included
// and no fees can be charged for users even though
// the validation has used some resources, in our case
// given we charge the fees on flow transaction and we
// are doing on chain validation we can/should charge the
// user for the validation fee.
const InvalidTransactionGasCost = 1_000

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

// Result captures the result of an interaction to the emulator
// it could be the out put of a direct call or output of running an
// evm transaction.
// Its more comprehensive than typical evm receipt, usually
// the receipt generation requires some extra calculation (e.g. Deployed contract address)
// but we take a different apporach here and include more data so that
// it requires less work for anyone who tracks and consume results.
type Result struct {
	// captures error returned during validation step (pre-checks)
	ValidationError error
	// captures error returned by the EVM
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

// Invalid returns true if transaction has been rejected
func (res *Result) Invalid() bool {
	return res.ValidationError != nil
}

// Failed returns true if transaction has been executed but VM has returned some error
func (res *Result) Failed() bool {
	return res.VMError != nil
}

// SetValidationError sets the validation error
// and also sets the gas used to the fixed invalid gas usage
func (res *Result) SetValidationError(err error) {
	res.ValidationError = err
	res.GasConsumed = InvalidTransactionGasCost
}

// returns the VM error as an string, if no error it returns an empty string
func (res *Result) VMErrorString() string {
	if res.VMError != nil {
		return res.VMError.Error()
	}
	return ""
}

// Receipt constructs an EVM-style receipt
// can be used by json-rpc and other integration to be returned.
//
// This is method is also used to construct block receipt root hash
// which requires the return receipt satisfy RLP encoding and cover these feilds
// Type (txType), PostState or Status, CumulativeGasUsed, Logs and Logs Bloom
// and for each log, Address, Topics, Data (consensus fields)
// During execution we also do fill in BlockNumber, TxIndex, Index (event index)
func (res *Result) Receipt() *gethTypes.Receipt {
	if res.Invalid() {
		return nil
	}
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
	return receipt
}

// ResultSummary constructs a result summary
func (res *Result) ResultSummary() *ResultSummary {
	rs := &ResultSummary{}

	rs.GasConsumed = res.GasConsumed
	rs.DeployedContractAddress = res.DeployedContractAddress
	rs.ReturnedValue = res.ReturnedValue

	if res.Invalid() {
		rs.ErrorCode = ValidationErrorCode(res.ValidationError)
		rs.Status = StatusInvalid
		return rs
	}

	if res.Failed() {
		rs.ErrorCode = ExecutionErrorCode(res.VMError)
		rs.Status = StatusFailed
		return rs
	}

	rs.Status = StatusSuccessful
	return rs
}
