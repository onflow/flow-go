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
	return fmt.Sprintf("cadence runtime error %s", e.err.Error())
}

// Code returns the error code for this error
func (e CadenceRuntimeError) Code() uint32 {
	return errCodeCadenceRunTimeError
}

// Unwrap returns the wrapped err
func (e CadenceRuntimeError) Unwrap() error {
	return e.err
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
	return fmt.Sprintf("address %s storage %d is over capacity %d", e.address, e.storageUsed, e.storageCapacity)
}

// Code returns the error code for this error
func (e StorageCapacityExceededError) Code() uint32 {
	return errCodeStorageCapacityExceeded
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
		"total event byte size (%d) exceeds limit (%d)",
		e.totalByteSize,
		e.limit,
	)
}

// Code returns the error code for this error
func (e EventLimitExceededError) Code() uint32 {
	return errCodeEventLimitExceededError
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
	return fmt.Sprintf("key %s has size %d which is higher than storage key size limit %d.", strings.Join([]string{e.owner, e.controller, e.key}, "/"), e.size, e.limit)
}

// Code returns the error code for this error
func (e StateKeySizeLimitError) Code() uint32 {
	return errCodeStateValueSizeLimitError
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
	return fmt.Sprintf("value %s has size %d which is higher than storage value size limit %d.", string(e.value[0:10])+"..."+string(e.value[len(e.value)-10:]), e.size, e.limit)
}

// Code returns the error code for this error
func (e StateValueSizeLimitError) Code() uint32 {
	return errCodeStateValueSizeLimitError
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
	return fmt.Sprintf("max interaction with storage has exceeded the limit (used: %d, limit %d)", e.used, e.limit)
}

// Code returns the error code for this error
func (e *LedgerIntractionLimitExceededError) Code() uint32 {
	return errCodeLedgerIntractionLimitExceededError
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
	return fmt.Sprintf("operation (%s) is not supported in this environment", e.operation)
}

// Code returns the error code for this error
func (e *OperationNotSupportedError) Code() uint32 {
	return errCodeOperationNotSupportedError
}

// EncodingUnsupportedValueError indicates that Cadence attempted
// to encode a value that is not supported.
type EncodingUnsupportedValueError struct {
	Value interpreter.Value
	Path  []string
}

func (e *EncodingUnsupportedValueError) Error() string {
	return fmt.Sprintf(
		"encoding unsupported value to path [%s]: %[1]T, %[1]v",
		strings.Join(e.Path, ","),
		e.Value,
	)
}

// Code returns the error code for this error
func (e *EncodingUnsupportedValueError) Code() uint32 {
	return errCodeEncodingUnsupportedValue
}
