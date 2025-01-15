package types

import (
	"fmt"

	"github.com/onflow/go-ethereum/accounts/abi"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	"github.com/onflow/go-ethereum/rlp"
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

// ChecksumLength captures number of bytes a checksum uses
const ChecksumLength = 4

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
	// StatusSuccessful shows that the transaction has been executed and the execution has returned success
	// for this case a block is formed and receipts are available
	StatusSuccessful Status = 3
)

// ResultSummary summarizes the outcome of a EVM call or tx run
type ResultSummary struct {
	Status                  Status
	ErrorCode               ErrorCode
	ErrorMessage            string
	GasConsumed             uint64
	GasRefund               uint64
	DeployedContractAddress *Address
	ReturnedData            Data
}

// NewInvalidResult creates a new result that hold transaction validation
// error as well as the defined gas cost for validation.
func NewInvalidResult(tx *gethTypes.Transaction, err error) *Result {
	return &Result{
		TxType:          tx.Type(),
		TxHash:          tx.Hash(),
		ValidationError: err,
		GasConsumed:     InvalidTransactionGasCost,
	}
}

// Result captures the result of an interaction to the emulator
// it could be the output of a direct call or output of running an
// evm transaction.
// Its more comprehensive than typical evm receipt, usually
// the receipt generation requires some extra calculation (e.g. Deployed contract address)
// but we take a different approach here and include more data so that
// it requires less work for anyone who tracks and consume results.
type Result struct {
	// captures error returned during validation step (pre-checks)
	ValidationError error
	// captures error returned by the EVM
	VMError error
	// type of transaction defined by the evm package
	// see DirectCallTxType as extra type we added type for direct calls.
	TxType uint8
	// total gas consumed during execution
	GasConsumed uint64
	// total gas used by the block after this tx execution
	CumulativeGasUsed uint64
	// total gas refunds after transaction execution
	GasRefund uint64
	// the address where the contract is deployed (if any)
	DeployedContractAddress *Address
	// returned data from a function call
	ReturnedData []byte
	// EVM logs (events that are emitted by evm)
	Logs []*gethTypes.Log
	// TX hash holds the cached value of tx hash
	TxHash gethCommon.Hash
	// transaction block inclusion index
	Index uint16
	// PrecompiledCalls captures an encoded list of calls to the precompile
	// during the execution of transaction
	PrecompiledCalls []byte
	// StateChangeCommitment captures a commitment over the state change (delta)
	StateChangeCommitment []byte
}

// Invalid returns true if transaction has been rejected
func (res *Result) Invalid() bool {
	return res.ValidationError != nil
}

// Failed returns true if transaction has been executed but VM has returned some error
func (res *Result) Failed() bool {
	return res.VMError != nil
}

// Successful returns true if transaction has been executed without any errors
func (res *Result) Successful() bool {
	return !res.Failed() && !res.Invalid()
}

// SetValidationError sets the validation error
// and also sets the gas used to the fixed invalid gas usage
func (res *Result) SetValidationError(err error) {
	res.ValidationError = err
	// for invalid transactions we only set the gasConsumed
	// for metering reasons, yet we do not set the CumulativeGasUsed
	// since we won't consider then for the block construction purposes
	res.GasConsumed = InvalidTransactionGasCost
}

// returns the VM error as an string, if no error it returns an empty string
func (res *Result) VMErrorString() string {
	if res.VMError != nil {
		return res.VMError.Error()
	}
	return ""
}

// ErrorMsg returns the error message, if any VM or Validation error
// both error would never happen at the same time
// but if it happens the priority is by validation error
func (res *Result) ErrorMsg() string {
	errorMsg := ""
	if res.VMError != nil {
		errorMsg = res.VMError.Error()
	}
	if res.ValidationError != nil {
		errorMsg = res.ValidationError.Error()
	}
	return errorMsg
}

// ErrorMessageWithRevertReason returns the error message, if any VM or Validation
// error occurred. Execution reverts coming from `assert` or `require` Solidity
// statements, are parsed into their human-friendly representation.
func (res *Result) ErrorMessageWithRevertReason() string {
	errorMessage := res.ErrorMsg()

	if res.ResultSummary().ErrorCode == ExecutionErrCodeExecutionReverted {
		reason, errUnpack := abi.UnpackRevert(res.ReturnedData)
		if errUnpack == nil {
			errorMessage = fmt.Sprintf("%v: %v", gethVM.ErrExecutionReverted.Error(), reason)
		}
	}

	return errorMessage
}

// RLPEncodedLogs returns the rlp encoding of the logs
func (res *Result) RLPEncodedLogs() ([]byte, error) {
	var encodedLogs []byte
	var err error
	if len(res.Logs) > 0 {
		encodedLogs, err = rlp.EncodeToBytes(res.Logs)
		if err != nil {
			return encodedLogs, err
		}
	}
	return encodedLogs, nil
}

// DeployedContractAddressString returns an string of the deployed address
// it returns an empty string if the deployed address is nil
func (res *Result) DeployedContractAddressString() string {
	deployedAddress := ""
	if res.DeployedContractAddress != nil {
		deployedAddress = res.DeployedContractAddress.String()
	}
	return deployedAddress
}

// StateChangeChecksum constructs a checksum
// based on the state change commitment on the result
func (res *Result) StateChangeChecksum() [ChecksumLength]byte {
	return SliceToChecksumLength(res.StateChangeCommitment)
}

// SliceToChecksumLength cuts the first 4 bytes of the input and convert it into checksum
func SliceToChecksumLength(input []byte) [ChecksumLength]byte {
	// the first 4 bytes of StateChangeCommitment is used as checksum
	var checksum [ChecksumLength]byte
	if len(input) >= ChecksumLength {
		copy(checksum[:ChecksumLength], input[:ChecksumLength])
	}
	return checksum
}

// Receipt constructs an EVM-style receipt
// can be used by json-rpc and other integration to be returned.
//
// This is method is also used to construct block receipt root hash
// which requires the return receipt satisfy RLP encoding and cover these fields
// Type (txType), PostState or Status, CumulativeGasUsed, Logs and Logs Bloom
// and for each log, Address, Topics, Data (consensus fields)
// During execution we also do fill in BlockNumber, TxIndex, Index (event index)
func (res *Result) Receipt() *gethTypes.Receipt {
	if res.Invalid() {
		return nil
	}

	receipt := &gethTypes.Receipt{
		TxHash:            res.TxHash,
		GasUsed:           res.GasConsumed,
		CumulativeGasUsed: res.CumulativeGasUsed,
		Logs:              res.Logs,
	}

	// only add tx type if not direct call
	if res.TxType != DirectCallTxType {
		receipt.Type = res.TxType
	}

	if res.DeployedContractAddress != nil {
		receipt.ContractAddress = res.DeployedContractAddress.ToCommon()
	}
	if res.Failed() {
		receipt.Status = gethTypes.ReceiptStatusFailed
	} else {
		receipt.Status = gethTypes.ReceiptStatusSuccessful
	}

	receipt.Bloom = gethTypes.CreateBloom(gethTypes.Receipts{receipt})
	return receipt
}

// LightReceipt constructs a light receipt from the result
// that is used for storing in block proposal.
func (res *Result) LightReceipt() *LightReceipt {
	if res.Invalid() {
		return nil
	}

	receipt := &LightReceipt{
		CumulativeGasUsed: res.CumulativeGasUsed,
	}

	receipt.Logs = make([]LightLog, len(res.Logs))
	for i, l := range res.Logs {
		receipt.Logs[i] = LightLog{
			Address: l.Address,
			Topics:  l.Topics,
			Data:    l.Data,
		}
	}

	// only add tx type if not direct call
	if res.TxType != DirectCallTxType {
		receipt.Type = res.TxType
	}

	// add status
	if res.Failed() {
		receipt.Status = uint8(gethTypes.ReceiptStatusFailed)
	} else {
		receipt.Status = uint8(gethTypes.ReceiptStatusSuccessful)
	}

	return receipt
}

// ResultSummary constructs a result summary
func (res *Result) ResultSummary() *ResultSummary {
	rs := &ResultSummary{
		GasConsumed:             res.GasConsumed,
		GasRefund:               res.GasRefund,
		DeployedContractAddress: res.DeployedContractAddress,
		ReturnedData:            res.ReturnedData,
		Status:                  StatusSuccessful,
	}

	if res.Invalid() {
		rs.ErrorCode = ValidationErrorCode(res.ValidationError)
		rs.ErrorMessage = res.ValidationError.Error()
		rs.Status = StatusInvalid
		return rs
	}

	if res.Failed() {
		rs.ErrorCode = ExecutionErrorCode(res.VMError)
		rs.ErrorMessage = res.VMError.Error()
		rs.Status = StatusFailed
		return rs
	}

	return rs
}

// LightLog captures only consensus fields of an EVM log
// used by the LightReceipt
type LightLog struct {
	// address of the contract that generated the event
	Address gethCommon.Address
	// list of topics provided by the contract.
	Topics []gethCommon.Hash
	// supplied by the contract, usually ABI-encoded
	Data []byte
}

// LightReceipt captures only the consensus fields of
// a receipt, making storage of receipts for the purpose
// of trie building more storage efficient.
//
// Note that we don't store Bloom as we can reconstruct it
// later. We don't have PostState and we use a uint8 for
// status as there is currently only acts as boolean.
// Data shows that using the light receipt results in 60% storage reduction
// for block proposals and the extra overheads are manageable.
type LightReceipt struct {
	Type              uint8
	Status            uint8
	CumulativeGasUsed uint64
	Logs              []LightLog
}

// ToReceipt constructs a Receipt from the LightReceipt
// Warning, this only populates the consensus fields
// and if you want the full data, use the receipt
// from the result.
func (lr *LightReceipt) ToReceipt() *gethTypes.Receipt {
	receipt := &gethTypes.Receipt{
		Type:              lr.Type,
		Status:            uint64(lr.Status),
		CumulativeGasUsed: lr.CumulativeGasUsed,
	}

	receipt.Logs = make([]*gethTypes.Log, len(lr.Logs))
	for i, l := range lr.Logs {
		receipt.Logs[i] = &gethTypes.Log{
			Address: l.Address,
			Topics:  l.Topics,
			Data:    l.Data,
		}
	}

	receipt.Bloom = gethTypes.CreateBloom(gethTypes.Receipts{receipt})
	return receipt
}
