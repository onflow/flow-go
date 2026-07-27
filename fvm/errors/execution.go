package errors

import (
	"fmt"
	"slices"

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
) CodedFailure {
	return WrapCodedFailure(
		FailureCodePayerBalanceCheckFailure,
		err,
		"failed to check if the payer %s has sufficient balance",
		payer)
}

// NewDerivedDataCacheImplementationFailure indicate an implementation error in
// the derived data cache.
func NewDerivedDataCacheImplementationFailure(
	err error,
) CodedFailure {
	return WrapCodedFailure(
		FailureCodeDerivedDataCacheImplementationFailure,
		err,
		"implementation error in derived data cache")
}

// NewRandomSourceFailure indicate an implementation error in
// the random source provider.
func NewRandomSourceFailure(
	err error,
) CodedFailure {
	return WrapCodedFailure(
		FailureCodeRandomSourceFailure,
		err,
		"implementation error in random source provider")
}

// NewExecutionVersionProviderFailure indicates a irrecoverable failure in the execution
// version provider.
func NewExecutionVersionProviderFailure(
	err error,
) CodedFailure {
	return WrapCodedFailure(
		FailureCodeExecutionVersionProvider,
		err,
		"Failure in execution version provider")
}

// LimitKind identifies which metering limit was exceeded.
type LimitKind uint8

const (
	// LimitKindComputation is the computation (execution effort) limit.
	LimitKindComputation LimitKind = iota + 1
	// LimitKindMemory is the memory limit.
	LimitKindMemory
	// LimitKindLedgerInteraction is the ledger interaction limit
	// (total bytes read from and written to storage).
	LimitKindLedgerInteraction
	// LimitKindEvent is the total emitted event byte size limit.
	LimitKindEvent
)

func (kind LimitKind) String() string {
	switch kind {
	case LimitKindComputation:
		return "computation"
	case LimitKindMemory:
		return "memory"
	case LimitKindLedgerInteraction:
		return "ledger interaction"
	case LimitKindEvent:
		return "event byte size"
	default:
		panic(fmt.Sprintf("unknown limit kind: %d", uint8(kind)))
	}
}

// errorCode returns the ErrorCode associated with the limit kind.
// Each kind keeps its own distinct, externally visible error code.
func (kind LimitKind) errorCode() ErrorCode {
	switch kind {
	case LimitKindComputation:
		return ErrCodeComputationLimitExceededError
	case LimitKindMemory:
		return ErrCodeMemoryLimitExceededError
	case LimitKindLedgerInteraction:
		return ErrCodeLedgerInteractionLimitExceededError
	case LimitKindEvent:
		return ErrCodeEventLimitExceededError
	default:
		panic(fmt.Sprintf("unknown limit kind: %d", uint8(kind)))
	}
}

// LimitExceededError is a CodedError which indicates that execution has
// exceeded one of its metering limits. The exceeded limit is identified by
// LimitKind. Each kind retains its own distinct error code.
type LimitExceededError struct {
	codedError
	kind  LimitKind
	used  uint64
	limit uint64
}

var _ CodedError = (*LimitExceededError)(nil)

// NewLimitExceededError constructs a new LimitExceededError.
//
// INVARIANT: used > limit. Only construct this error after a meter has
// recorded usage exceeding the limit, never from "is enough left?"
// pre-checks (e.g. the EVM gas pre-check), which must use their own error
// types.
func NewLimitExceededError(kind LimitKind, used uint64, limit uint64) LimitExceededError {
	return LimitExceededError{
		codedError: NewCodedError(
			kind.errorCode(),
			"%s limit exceeded (used: %d, limit: %d)",
			kind,
			used,
			limit),
		kind:  kind,
		used:  used,
		limit: limit,
	}
}

// LimitKind returns which metering limit was exceeded.
func (err LimitExceededError) LimitKind() LimitKind {
	return err.kind
}

// Used returns the metered usage at the moment the limit was exceeded. It is
// always greater than Limit, but since metering aborts on the first detected
// excess, it does not reflect the execution's total demand.
func (err LimitExceededError) Used() uint64 {
	return err.used
}

// Limit returns the limit that was exceeded.
func (err LimitExceededError) Limit() uint64 {
	return err.limit
}

// AsLimitExceededError returns the LimitExceededError in err's chain, if any.
func AsLimitExceededError(err error) (LimitExceededError, bool) {
	var limitErr LimitExceededError
	ok := As(err, &limitErr)
	return limitErr, ok
}

// IsLimitExceededError reports whether err's chain contains a
// LimitExceededError. If any kinds are provided, the error's kind must match
// one of them.
func IsLimitExceededError(err error, kinds ...LimitKind) bool {
	limitErr, ok := AsLimitExceededError(err)
	if !ok {
		return false
	}
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, limitErr.kind)
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
