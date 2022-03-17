package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionError captures errors when executing a transaction/script.
// A transaction having this error has already passed validation and is included in a collection.
// the transaction will be executed by execution nodes but the result is reverted
// and in some cases there will be a penalty (or fees) for the payer, access nodes or collection nodes.
type ExecutionError interface {
	Error
}

// CadenceRuntimeError captures a collection of errors provided by cadence runtime
// it cover cadence errors such as
// NotDeclaredError, NotInvokableError, ArgumentCountError, TransactionNotDeclaredError,
// ConditionError, RedeclarationError, DereferenceError,
// OverflowError, UnderflowError, DivisionByZeroError,
// DestroyedCompositeError,  ForceAssignmentToNonNilResourceError, ForceNilError,
// TypeMismatchError, InvalidPathDomainError, OverwriteError, CyclicLinkError,
// ArrayIndexOutOfBoundsError, ...
type CadenceRuntimeError struct {
	err *runtime.Error
}

// NewCadenceRuntimeError constructs a new CadenceRuntimeError and wraps a cadence runtime error
func NewCadenceRuntimeError(err *runtime.Error) *CadenceRuntimeError {
	return &CadenceRuntimeError{err: err}
}

// IsCadenceRuntimeError returns true if error has this type
func IsCadenceRuntimeError(err error) bool {
	var t *CadenceRuntimeError
	return errors.As(err, &t)
}

func (e CadenceRuntimeError) Error() string {
	return fmt.Sprintf("%s cadence runtime error %s", e.Code().String(), e.err.Error())
}

// Code returns the error code for this error
func (e CadenceRuntimeError) Code() ErrorCode {
	return ErrCodeCadenceRunTimeError
}

// Unwrap returns the wrapped err
func (e CadenceRuntimeError) Unwrap() error {
	return e.err
}

// An TransactionFeeDeductionFailedError indicates that a there was an error deducting transaction fees from the transaction Payer
type TransactionFeeDeductionFailedError struct {
	Payer  flow.Address
	TxFees uint64
	err    error
}

// NewTransactionFeeDeductionFailedError constructs a new TransactionFeeDeductionFailedError
func NewTransactionFeeDeductionFailedError(payer flow.Address, txFees uint64, err error) *TransactionFeeDeductionFailedError {
	return &TransactionFeeDeductionFailedError{
		Payer:  payer,
		TxFees: txFees,
		err:    err,
	}
}

func (e TransactionFeeDeductionFailedError) Error() string {
	return fmt.Sprintf("%s failed to deduct %d transaction fees from %s: %s", e.Code().String(), e.TxFees, e.Payer, e.err)
}

// Code returns the error code for this error
func (e TransactionFeeDeductionFailedError) Code() ErrorCode {
	return ErrCodeTransactionFeeDeductionFailedError
}

// Unwrap returns the wrapped err
func (e TransactionFeeDeductionFailedError) Unwrap() error {
	return e.err
}

// An ComputationLimitExceededError indicates that computation has exceeded its limit.
type ComputationLimitExceededError struct {
	limit uint64
}

// NewComputationLimitExceededError constructs a new ComputationLimitExceededError
func NewComputationLimitExceededError(limit uint64) *ComputationLimitExceededError {
	return &ComputationLimitExceededError{
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
	var t *ComputationLimitExceededError
	return errors.As(err, &t)
}

// An MemoryLimitExceededError indicates that execution has exceeded its memory limits.
type MemoryLimitExceededError struct {
	limit uint64
}

// NewMemoryLimitExceededError constructs a new MemoryLimitExceededError
func NewMemoryLimitExceededError(limit uint64) *MemoryLimitExceededError {
	return &MemoryLimitExceededError{
		limit: limit,
	}
}

// Code returns the error code for this error
func (e MemoryLimitExceededError) Code() ErrorCode {
	return ErrCodeMemoryLimitExceededError
}

func (e MemoryLimitExceededError) Error() string {
	return fmt.Sprintf(
		"%s memory usage exceeds limit (%d)",
		e.Code().String(),
		e.limit,
	)
}

// IsMemoryLimitExceededError returns true if error has this type
func IsMemoryLimitExceededError(err error) bool {
	var t *MemoryLimitExceededError
	return errors.As(err, &t)
}

// An StorageCapacityExceededError indicates that an account used more storage than it has storage capacity.
type StorageCapacityExceededError struct {
	address         flow.Address
	storageUsed     uint64
	storageCapacity uint64
}

// NewStorageCapacityExceededError constructs a new StorageCapacityExceededError
func NewStorageCapacityExceededError(address flow.Address, storageUsed, storageCapacity uint64) *StorageCapacityExceededError {
	return &StorageCapacityExceededError{
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

// EventLimitExceededError indicates that the transaction has produced events with size more than limit.
type EventLimitExceededError struct {
	totalByteSize uint64
	limit         uint64
}

// NewEventLimitExceededError constructs a EventLimitExceededError
func NewEventLimitExceededError(totalByteSize, limit uint64) *EventLimitExceededError {
	return &EventLimitExceededError{totalByteSize: totalByteSize, limit: limit}
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

// A StateKeySizeLimitError indicates that the provided key has exceeded the size limit allowed by the storage
type StateKeySizeLimitError struct {
	owner      string
	controller string
	key        string
	size       uint64
	limit      uint64
}

// NewStateKeySizeLimitError constructs a StateKeySizeLimitError
func NewStateKeySizeLimitError(owner, controller, key string, size, limit uint64) *StateKeySizeLimitError {
	return &StateKeySizeLimitError{owner: owner, controller: controller, key: key, size: size, limit: limit}
}

func (e StateKeySizeLimitError) Error() string {
	return fmt.Sprintf("%s key %s has size %d which is higher than storage key size limit %d.", e.Code().String(), strings.Join([]string{e.owner, e.controller, e.key}, "/"), e.size, e.limit)
}

// Code returns the error code for this error
func (e StateKeySizeLimitError) Code() ErrorCode {
	return ErrCodeStateKeySizeLimitError
}

// A StateValueSizeLimitError indicates that the provided value has exceeded the size limit allowed by the storage
type StateValueSizeLimitError struct {
	value flow.RegisterValue
	size  uint64
	limit uint64
}

// NewStateValueSizeLimitError constructs a StateValueSizeLimitError
func NewStateValueSizeLimitError(value flow.RegisterValue, size, limit uint64) *StateValueSizeLimitError {
	return &StateValueSizeLimitError{value: value, size: size, limit: limit}
}

func (e StateValueSizeLimitError) Error() string {
	return fmt.Sprintf("%s value %s has size %d which is higher than storage value size limit %d.",
		e.Code().String(), string(e.value[0:10])+"..."+string(e.value[len(e.value)-10:]), e.size, e.limit)
}

// Code returns the error code for this error
func (e StateValueSizeLimitError) Code() ErrorCode {
	return ErrCodeStateValueSizeLimitError
}

// LedgerIntractionLimitExceededError is returned when a tx hits the maximum ledger interaction limit
type LedgerIntractionLimitExceededError struct {
	used  uint64
	limit uint64
}

// NewLedgerIntractionLimitExceededError constructs a LedgerIntractionLimitExceededError
func NewLedgerIntractionLimitExceededError(used, limit uint64) *LedgerIntractionLimitExceededError {
	return &LedgerIntractionLimitExceededError{used: used, limit: limit}
}

func (e *LedgerIntractionLimitExceededError) Error() string {
	return fmt.Sprintf("%s max interaction with storage has exceeded the limit (used: %d bytes, limit %d bytes)", e.Code().String(), e.used, e.limit)
}

// Code returns the error code for this error
func (e *LedgerIntractionLimitExceededError) Code() ErrorCode {
	return ErrCodeLedgerIntractionLimitExceededError
}

// OperationNotSupportedError is generated when an operation (e.g. getting block info) is
// not supported in the current environment.
type OperationNotSupportedError struct {
	operation string
}

// NewOperationNotSupportedError construct a new OperationNotSupportedError
func NewOperationNotSupportedError(operation string) *OperationNotSupportedError {
	return &OperationNotSupportedError{operation: operation}
}

func (e *OperationNotSupportedError) Error() string {
	return fmt.Sprintf("%s operation (%s) is not supported in this environment", e.Code().String(), e.operation)
}

// Code returns the error code for this error
func (e *OperationNotSupportedError) Code() ErrorCode {
	return ErrCodeOperationNotSupportedError
}

// EncodingUnsupportedValueError indicates that Cadence attempted
// to encode a value that is not supported.
type EncodingUnsupportedValueError struct {
	value interpreter.Value
	path  []string
}

// NewEncodingUnsupportedValueError construct a new EncodingUnsupportedValueError
func NewEncodingUnsupportedValueError(value interpreter.Value, path []string) *EncodingUnsupportedValueError {
	return &EncodingUnsupportedValueError{value: value, path: path}
}

func (e *EncodingUnsupportedValueError) Error() string {
	return fmt.Sprintf(
		"%s encoding unsupported value to path [%s]: %[1]T, %[1]v",
		e.Code().String(),
		strings.Join(e.path, ","),
		e.value,
	)
}

// Code returns the error code for this error
func (e *EncodingUnsupportedValueError) Code() ErrorCode {
	return ErrCodeEncodingUnsupportedValue
}
