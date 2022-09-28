package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// CadenceRuntimeError captures a collection of errors provided by cadence runtime
// it cover cadence errors such as
// NotDeclaredError, NotInvokableError, ArgumentCountError, TransactionNotDeclaredError,
// ConditionError, RedeclarationError, DereferenceError,
// OverflowError, UnderflowError, DivisionByZeroError,
// DestroyedCompositeError,  ForceAssignmentToNonNilResourceError, ForceNilError,
// TypeMismatchError, InvalidPathDomainError, OverwriteError, CyclicLinkError,
// ArrayIndexOutOfBoundsError, ...
// The Cadence error might have occurred because of an inner fvm Error.
type CadenceRuntimeError struct {
	errorWrapper
}

// NewCadenceRuntimeError constructs a new CadenceRuntimeError and wraps a cadence runtime error
func NewCadenceRuntimeError(err runtime.Error) CadenceRuntimeError {
	return CadenceRuntimeError{
		errorWrapper: errorWrapper{
			err: err,
		},
	}
}

func (e CadenceRuntimeError) Error() string {
	return fmt.Sprintf("%s cadence runtime error %s", e.Code().String(), e.err.Error())
}

// Code returns the error code for this error
func (e CadenceRuntimeError) Code() ErrorCode {
	return ErrCodeCadenceRunTimeError
}

func IsCadenceRuntimeError(err error) bool {
	var t CadenceRuntimeError
	return As(err, &t)
}

// An TransactionFeeDeductionFailedError indicates that a there was an error deducting transaction fees from the transaction Payer
type TransactionFeeDeductionFailedError struct {
	errorWrapper

	Payer  flow.Address
	TxFees uint64
}

// NewTransactionFeeDeductionFailedError constructs a new TransactionFeeDeductionFailedError
func NewTransactionFeeDeductionFailedError(
	payer flow.Address,
	txFees uint64,
	err error,
) TransactionFeeDeductionFailedError {
	return TransactionFeeDeductionFailedError{
		Payer:  payer,
		TxFees: txFees,
		errorWrapper: errorWrapper{
			err: err,
		},
	}
}

func (e TransactionFeeDeductionFailedError) Error() string {
	return fmt.Sprintf("%s failed to deduct %d transaction fees from %s: %s", e.Code().String(), e.TxFees, e.Payer, e.err)
}

// Code returns the error code for this error
func (e TransactionFeeDeductionFailedError) Code() ErrorCode {
	return ErrCodeTransactionFeeDeductionFailedError
}

// An ComputationLimitExceededError indicates that computation has exceeded its limit.
type ComputationLimitExceededError struct {
	limit uint64
}

// NewComputationLimitExceededError constructs a new ComputationLimitExceededError
func NewComputationLimitExceededError(limit uint64) ComputationLimitExceededError {
	return ComputationLimitExceededError{
		limit: limit,
	}
}

// Code returns the error code for this error
func (e ComputationLimitExceededError) Code() ErrorCode {
	return ErrCodeComputationLimitExceededError
}

func (e ComputationLimitExceededError) Error() string {
	return fmt.Sprintf(
		"%s computation exceeds limit (%d)",
		e.Code().String(),
		e.limit,
	)
}

// IsComputationLimitExceededError returns true if error has this type
func IsComputationLimitExceededError(err error) bool {
	var t ComputationLimitExceededError
	return errors.As(err, &t)
}

// NewMemoryLimitExceededError constructs a new CodedError which indicates
// that execution has exceeded its memory limits.
func NewMemoryLimitExceededError(limit uint64) *CodedError {
	return NewCodedError(
		ErrCodeMemoryLimitExceededError,
		"memory usage exceeds limit (%d)",
		limit)
}

// IsMemoryLimitExceededError returns true if error has this code.
func IsMemoryLimitExceededError(err error) bool {
	return HasErrorCode(err, ErrCodeMemoryLimitExceededError)
}

// An StorageCapacityExceededError indicates that an account used more storage than it has storage capacity.
type StorageCapacityExceededError struct {
	address         flow.Address
	storageUsed     uint64
	storageCapacity uint64
}

// NewStorageCapacityExceededError constructs a new StorageCapacityExceededError
func NewStorageCapacityExceededError(address flow.Address, storageUsed, storageCapacity uint64) StorageCapacityExceededError {
	return StorageCapacityExceededError{
		address:         address,
		storageUsed:     storageUsed,
		storageCapacity: storageCapacity,
	}
}

func (e StorageCapacityExceededError) Error() string {
	return fmt.Sprintf("%s The account with address (%s) uses %d bytes of storage which is over its capacity (%d bytes). Capacity can be increased by adding FLOW tokens to the account.", e.Code().String(), e.address, e.storageUsed, e.storageCapacity)
}

// Code returns the error code for this error
func (e StorageCapacityExceededError) Code() ErrorCode {
	return ErrCodeStorageCapacityExceeded
}

func IsStorageCapacityExceededError(err error) bool {
	var t StorageCapacityExceededError
	return As(err, &t)
}

// EventLimitExceededError indicates that the transaction has produced events with size more than limit.
type EventLimitExceededError struct {
	totalByteSize uint64
	limit         uint64
}

// NewEventLimitExceededError constructs a EventLimitExceededError
func NewEventLimitExceededError(totalByteSize, limit uint64) EventLimitExceededError {
	return EventLimitExceededError{
		totalByteSize: totalByteSize,
		limit:         limit,
	}
}

func (e EventLimitExceededError) Error() string {
	return fmt.Sprintf(
		"%s total event byte size (%d) exceeds limit (%d)",
		e.Code().String(),
		e.totalByteSize,
		e.limit,
	)
}

// Code returns the error code for this error
func (e EventLimitExceededError) Code() ErrorCode {
	return ErrCodeEventLimitExceededError
}

// NewStateKeySizeLimitError constructs a CodedError which indicates that the
// provided key has exceeded the size limit allowed by the storage.
func NewStateKeySizeLimitError(
	owner string,
	key string,
	size uint64,
	limit uint64,
) *CodedError {
	return NewCodedError(
		ErrCodeStateKeySizeLimitError,
		"key %s has size %d which is higher than storage key size limit %d.",
		strings.Join([]string{owner, key}, "/"),
		size,
		limit)
}

// A StateValueSizeLimitError indicates that the provided value has exceeded the size limit allowed by the storage
type StateValueSizeLimitError struct {
	value flow.RegisterValue
	size  uint64
	limit uint64
}

// NewStateValueSizeLimitError constructs a StateValueSizeLimitError
func NewStateValueSizeLimitError(value flow.RegisterValue, size, limit uint64) StateValueSizeLimitError {
	return StateValueSizeLimitError{
		value: value,
		size:  size,
		limit: limit,
	}
}

func (e StateValueSizeLimitError) Error() string {
	valueStr := ""
	if len(e.value) > 23 {
		valueStr = string(e.value[0:10]) + "..." + string(e.value[len(e.value)-10:])
	} else {
		valueStr = string(e.value)
	}

	return fmt.Sprintf("%s value %s has size %d which is higher than storage value size limit %d.",
		e.Code().String(), valueStr, e.size, e.limit)
}

// Code returns the error code for this error
func (e StateValueSizeLimitError) Code() ErrorCode {
	return ErrCodeStateValueSizeLimitError
}

// NewLedgerInteractionLimitExceededError constructs a CodeError. It is
// returned when a tx hits the maximum ledger interaction limit.
func NewLedgerInteractionLimitExceededError(
	used uint64,
	limit uint64,
) *CodedError {
	return NewCodedError(
		ErrCodeLedgerInteractionLimitExceededError,
		"max interaction with storage has exceeded the limit "+
			"(used: %d bytes, limit %d bytes)",
		used,
		limit)
}

func IsLedgerInteractionLimitExceededError(err error) bool {
	return HasErrorCode(err, ErrCodeLedgerInteractionLimitExceededError)
}

// OperationNotSupportedError is generated when an operation (e.g. getting block info) is
// not supported in the current environment.
type OperationNotSupportedError struct {
	operation string
}

// NewOperationNotSupportedError construct a new OperationNotSupportedError
func NewOperationNotSupportedError(operation string) OperationNotSupportedError {
	return OperationNotSupportedError{
		operation: operation,
	}
}

func (e OperationNotSupportedError) Error() string {
	return fmt.Sprintf("%s operation (%s) is not supported in this environment", e.Code().String(), e.operation)
}

// Code returns the error code for this error
func (e OperationNotSupportedError) Code() ErrorCode {
	return ErrCodeOperationNotSupportedError
}

func IsOperationNotSupportedError(err error) bool {
	var t OperationNotSupportedError
	return As(err, &t)
}

// NewScriptExecutionCancelledError construct a new CodedError which indicates
// that Cadence Script execution has been cancelled (e.g. request connection
// has been droped)
//
// note: this error is used by scripts only and won't be emitted for
// transactions since transaction execution has to be deterministic.
func NewScriptExecutionCancelledError(err error) *CodedError {
	return WrapCodedError(
		ErrCodeScriptExecutionCancelledError,
		err,
		"script execution is cancelled")
}

// ScriptExecutionTimedOutError indicates that Cadence Script execution
// has been taking more time than what is allowed.
//
// note: this error is used by scripts only and
// won't be emitted for transactions since transaction execution has to be deterministic.
type ScriptExecutionTimedOutError struct {
}

// NewScriptExecutionTimedOutError construct a new ScriptExecutionTimedOutError
func NewScriptExecutionTimedOutError() ScriptExecutionTimedOutError {
	return ScriptExecutionTimedOutError{}
}

func (e ScriptExecutionTimedOutError) Error() string {
	return fmt.Sprintf(
		"%s script execution is timed out and did not finish executing within the maximum execution time allowed",
		e.Code().String(),
	)
}

// Code returns the error code for this error
func (e ScriptExecutionTimedOutError) Code() ErrorCode {
	return ErrCodeScriptExecutionTimedOutError
}

// NewCouldNotGetExecutionParameterFromStateError constructs a new CodedError
// which indicates that computation has exceeded its limit.
func NewCouldNotGetExecutionParameterFromStateError(
	address string,
	path string,
) *CodedError {
	return NewCodedError(
		ErrCodeCouldNotDecodeExecutionParameterFromState,
		"could not get execution parameter from the state "+
			"(address: %s path: %s)",
		address,
		path)
}
