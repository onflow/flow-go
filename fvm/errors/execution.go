package errors

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// NewCadenceRuntimeError constructs a new CodedError which captures a
// collection of errors provided by cadence runtime. It cover cadence errors
// such as:
//
// NotDeclaredError, NotInvokableError, ArgumentCountError,
// TransactionNotDeclaredError, ConditionError, RedeclarationError,
// DereferenceError, OverflowError, UnderflowError, DivisionByZeroError,
// DestroyedCompositeError,  ForceAssignmentToNonNilResourceError,
// ForceNilError, TypeMismatchError, InvalidPathDomainError, OverwriteError,
// CyclicLinkError, ArrayIndexOutOfBoundsError, ...
//
// The Cadence error might have occurred because of an inner fvm Error.
func NewCadenceRuntimeError(err runtime.Error) CodedError {
	return WrapCodedError(
		ErrCodeCadenceRunTimeError,
		err,
		"cadence runtime error")
}

func IsCadenceRuntimeError(err error) bool {
	return HasErrorCode(err, ErrCodeCadenceRunTimeError)
}

// NewTransactionFeeDeductionFailedError constructs a new CodedError which
// indicates that a there was an error deducting transaction fees from the
// transaction Payer
func NewTransactionFeeDeductionFailedError(
	payer flow.Address,
	txFees uint64,
	err error,
) CodedError {
	return WrapCodedError(
		ErrCodeTransactionFeeDeductionFailedError,
		err,
		"failed to deduct %d transaction fees from %s",
		txFees,
		payer)
}

// IsTransactionFeeDeductionFailedError returns true if error has this code.
func IsTransactionFeeDeductionFailedError(err error) bool {
	return HasErrorCode(err, ErrCodeTransactionFeeDeductionFailedError)
}

// NewInsufficientPayerBalanceError constructs a new CodedError which
// indicates that the payer has insufficient balance to attempt transaction execution.
func NewInsufficientPayerBalanceError(
	payer flow.Address,
	requiredBalance cadence.UFix64,
) CodedError {
	return NewCodedError(
		ErrCodeInsufficientPayerBalance,
		"payer %s has insufficient balance to attempt transaction execution (required balance: %s)",
		payer,
		requiredBalance,
	)
}

// IsInsufficientPayerBalanceError returns true if error has this code.
func IsInsufficientPayerBalanceError(err error) bool {
	return HasErrorCode(err, ErrCodeInsufficientPayerBalance)
}

// indicates that a there was an error checking the payers balance.
// This is an implementation error most likely between the smart contract and FVM interaction
// and should not happen in regular execution.
func NewPayerBalanceCheckFailure(
	payer flow.Address,
	err error,
) CodedError {
	return WrapCodedError(
		FailureCodePayerBalanceCheckFailure,
		err,
		"failed to check if the payer %s has sufficient balance",
		payer)
}

// NewDerivedDataCacheImplementationFailure indicate an implementation error in
// the derived data cache.
func NewDerivedDataCacheImplementationFailure(
	err error,
) CodedError {
	return WrapCodedError(
		FailureCodeDerivedDataCacheImplementationFailure,
		err,
		"implementation error in derived data cache")
}

// NewRandomSourceFailure indicate an implementation error in
// the random source provider.
func NewRandomSourceFailure(
	err error,
) CodedError {
	return WrapCodedError(
		FailureCodeRandomSourceFailure,
		err,
		"implementation error in random source provider")
}

// NewComputationLimitExceededError constructs a new CodedError which indicates
// that computation has exceeded its limit.
func NewComputationLimitExceededError(limit uint64) CodedError {
	return NewCodedError(
		ErrCodeComputationLimitExceededError,
		"computation exceeds limit (%d)",
		limit)
}

// IsComputationLimitExceededError returns true if error has this code.
func IsComputationLimitExceededError(err error) bool {
	return HasErrorCode(err, ErrCodeComputationLimitExceededError)
}

// NewMemoryLimitExceededError constructs a new CodedError which indicates
// that execution has exceeded its memory limits.
func NewMemoryLimitExceededError(limit uint64) CodedError {
	return NewCodedError(
		ErrCodeMemoryLimitExceededError,
		"memory usage exceeds limit (%d)",
		limit)
}

// IsMemoryLimitExceededError returns true if error has this code.
func IsMemoryLimitExceededError(err error) bool {
	return HasErrorCode(err, ErrCodeMemoryLimitExceededError)
}

// NewStorageCapacityExceededError constructs a new CodedError which indicates
// that an account used more storage than it has storage capacity.
func NewStorageCapacityExceededError(
	address flow.Address,
	storageUsed uint64,
	storageCapacity uint64,
) CodedError {
	return NewCodedError(
		ErrCodeStorageCapacityExceeded,
		"The account with address (%s) uses %d bytes of storage which is "+
			"over its capacity (%d bytes). Capacity can be increased by "+
			"adding FLOW tokens to the account.",
		address,
		storageUsed,
		storageCapacity)
}

func IsStorageCapacityExceededError(err error) bool {
	return HasErrorCode(err, ErrCodeStorageCapacityExceeded)
}

// NewEventLimitExceededError constructs a CodedError which indicates that the
// transaction has produced events with size more than limit.
func NewEventLimitExceededError(
	totalByteSize uint64,
	limit uint64) CodedError {
	return NewCodedError(
		ErrCodeEventLimitExceededError,
		"total event byte size (%d) exceeds limit (%d)",
		totalByteSize,
		limit)
}

// NewStateKeySizeLimitError constructs a CodedError which indicates that the
// provided key has exceeded the size limit allowed by the storage.
func NewStateKeySizeLimitError(
	id flow.RegisterID,
	size uint64,
	limit uint64,
) CodedError {
	return NewCodedError(
		ErrCodeStateKeySizeLimitError,
		"key %s has size %d which is higher than storage key size limit %d.",
		id,
		size,
		limit)
}

// NewStateValueSizeLimitError constructs a CodedError which indicates that the
// provided value has exceeded the size limit allowed by the storage.
func NewStateValueSizeLimitError(
	value flow.RegisterValue,
	size uint64,
	limit uint64,
) CodedError {
	valueStr := ""
	if len(value) > 23 {
		valueStr = string(value[0:10]) + "..." + string(value[len(value)-10:])
	} else {
		valueStr = string(value)
	}

	return NewCodedError(
		ErrCodeStateValueSizeLimitError,
		"value %s has size %d which is higher than storage value size "+
			"limit %d.",
		valueStr,
		size,
		limit)
}

// NewLedgerInteractionLimitExceededError constructs a CodeError. It is
// returned when a tx hits the maximum ledger interaction limit.
func NewLedgerInteractionLimitExceededError(
	used uint64,
	limit uint64,
) CodedError {
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

// NewOperationNotSupportedError construct a new CodedError. It is generated
// when an operation (e.g. getting block info) is not supported in the current
// environment.
func NewOperationNotSupportedError(operation string) CodedError {
	return NewCodedError(
		ErrCodeOperationNotSupportedError,
		"operation (%s) is not supported in this environment",
		operation)
}

func IsOperationNotSupportedError(err error) bool {
	return HasErrorCode(err, ErrCodeOperationNotSupportedError)
}

// NewScriptExecutionCancelledError construct a new CodedError which indicates
// that Cadence Script execution has been cancelled (e.g. request connection
// has been droped)
//
// note: this error is used by scripts only and won't be emitted for
// transactions since transaction execution has to be deterministic.
func NewScriptExecutionCancelledError(err error) CodedError {
	return WrapCodedError(
		ErrCodeScriptExecutionCancelledError,
		err,
		"script execution is cancelled")
}

// NewScriptExecutionTimedOutError construct a new CodedError which indicates
// that Cadence Script execution has been taking more time than what is allowed.
//
// note: this error is used by scripts only and won't be emitted for
// transactions since transaction execution has to be deterministic.
func NewScriptExecutionTimedOutError() CodedError {
	return NewCodedError(
		ErrCodeScriptExecutionTimedOutError,
		"script execution is timed out and did not finish executing within "+
			"the maximum execution time allowed")
}

// NewCouldNotGetExecutionParameterFromStateError constructs a new CodedError
// which indicates that computation has exceeded its limit.
func NewCouldNotGetExecutionParameterFromStateError(
	address string,
	path string,
) CodedError {
	return NewCodedError(
		ErrCodeCouldNotDecodeExecutionParameterFromState,
		"could not get execution parameter from the state "+
			"(address: %s path: %s)",
		address,
		path)
}

// NewInvalidInternalStateAccessError constructs a new CodedError which
// indicates that the cadence program attempted to directly access flow's
// internal state.
func NewInvalidInternalStateAccessError(
	id flow.RegisterID,
	opType string,
) CodedError {
	return NewCodedError(
		ErrCodeInvalidInternalStateAccessError,
		"could not directly %s flow internal state (%s)",
		opType,
		id)
}

// NewEVMError constructs a new CodedError which captures a
// collection of errors provided by (non-fatal) evm runtime.
func NewEVMError(err error) CodedError {
	return WrapCodedError(
		ErrEVMExecutionError,
		err,
		"evm runtime error")
}

// IsEVMError returns true if error is an EVM error
func IsEVMError(err error) bool {
	return HasErrorCode(err, ErrEVMExecutionError)
}
