package errors

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionExecutionError captures errors when executing a transaction.
// A transaction having this error has already passed validation and is included in a collection.
// the transaction will be executed by execution nodes but the result is reverted
// and in some cases there will be a penalty (or fees) for the payer, access nodes or collection nodes.
type TransactionExecutionError interface {
	TransactionError
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
	Err runtime.Error
}

func (e CadenceRuntimeError) Error() string {
	return fmt.Sprintf("cadence runtime error %s", e.Err.Error())
}

// Code returns the error code for this error
func (e CadenceRuntimeError) Code() uint32 {
	return errCodeCadenceRunTimeError
}

// Is returns true if the given error type is CadenceRuntimeError
func (e CadenceRuntimeError) Is(target error) bool {
	_, ok := target.(*CadenceRuntimeError)
	return ok
}

// An StorageCapacityExceededError indicates that an account used more storage than it has storage capacity.
type StorageCapacityExceededError struct {
	Address         flow.Address
	StorageUsed     uint64
	StorageCapacity uint64
}

func (e StorageCapacityExceededError) Error() string {
	return fmt.Sprintf("address %s storage %d is over capacity %d", e.Address, e.StorageUsed, e.StorageCapacity)
}

// Code returns the error code for this error
func (e StorageCapacityExceededError) Code() uint32 {
	return errCodeStorageCapacityExceeded
}

// Is returns true if the given error type is StorageCapacityExceededError
func (e StorageCapacityExceededError) Is(target error) bool {
	_, ok := target.(*StorageCapacityExceededError)
	return ok
}

// EventLimitExceededError indicates that the transaction has produced events with size more than limit.
type EventLimitExceededError struct {
	TotalByteSize uint64
	Limit         uint64
}

func (e EventLimitExceededError) Error() string {
	return fmt.Sprintf(
		"total event byte size (%d) exceeds limit (%d)",
		e.TotalByteSize,
		e.Limit,
	)
}

// Code returns the error code for this error
func (e EventLimitExceededError) Code() uint32 {
	return errCodeEventLimitExceededError
}

// Is returns true if the given error type is EventLimitExceededError
func (e EventLimitExceededError) Is(target error) bool {
	_, ok := target.(*EventLimitExceededError)
	return ok
}

// A StateKeySizeLimitError indicates that the provided key has exceeded the size limit allowed by the storage
type StateKeySizeLimitError struct {
	Owner      string
	Controller string
	Key        string
	Size       uint64
	Limit      uint64
}

func (e StateKeySizeLimitError) Error() string {
	return fmt.Sprintf("key %s has size %d which is higher than storage key size limit %d.", strings.Join([]string{e.Owner, e.Controller, e.Key}, "/"), e.Size, e.Limit)
}

// Code returns the error code for this error
func (e StateKeySizeLimitError) Code() uint32 {
	return errCodeStateValueSizeLimitError
}

// Is returns true if the given error type is StateKeySizeLimitError
func (e StateKeySizeLimitError) Is(target error) bool {
	_, ok := target.(*StateKeySizeLimitError)
	return ok
}

// A StateValueSizeLimitError indicates that the provided value has exceeded the size limit allowed by the storage
type StateValueSizeLimitError struct {
	Value flow.RegisterValue
	Size  uint64
	Limit uint64
}

func (e StateValueSizeLimitError) Error() string {
	return fmt.Sprintf("value %s has size %d which is higher than storage value size limit %d.", string(e.Value[0:10])+"..."+string(e.Value[len(e.Value)-10:]), e.Size, e.Limit)
}

// Code returns the error code for this error
func (e StateValueSizeLimitError) Code() uint32 {
	return errCodeStateValueSizeLimitError
}

// Is returns true if the given error type is StateValueSizeLimitError
func (e StateValueSizeLimitError) Is(target error) bool {
	_, ok := target.(*StateValueSizeLimitError)
	return ok
}

// LedgerIntractionLimitExceededError is returned when a tx hits the maximum ledger interaction limit
type LedgerIntractionLimitExceededError struct {
	Used  uint64
	Limit uint64
}

func (e *LedgerIntractionLimitExceededError) Error() string {
	return fmt.Sprintf("max interaction with storage has exceeded the limit (used: %d, limit %d)", e.Used, e.Limit)
}

// Code returns the error code for this error
func (e *LedgerIntractionLimitExceededError) Code() uint32 {
	return errCodeLedgerIntractionLimitExceededError
}

// Is returns true if the given error type is LedgerIntractionLimitExceededError
func (e *LedgerIntractionLimitExceededError) Is(target error) bool {
	_, ok := target.(*LedgerIntractionLimitExceededError)
	return ok
}

// OperationNotSupportedError is generated when an operation (e.g. getting block info) is
// not supported in the current environment.
type OperationNotSupportedError struct {
	Operation string
}

func (e *OperationNotSupportedError) Error() string {
	return fmt.Sprintf("%s is not supported in this environment.", e.Operation)
}

// Code returns the error code for this error
func (e *OperationNotSupportedError) Code() uint32 {
	return errCodeOperationNotSupportedError
}

// Is returns true if the given error type is OperationNotSupportedError
func (e *OperationNotSupportedError) Is(target error) bool {
	_, ok := target.(*OperationNotSupportedError)
	return ok
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

// Is returns true if the given error type is EncodingUnsupportedValueError
func (e *EncodingUnsupportedValueError) Is(target error) bool {
	_, ok := target.(*EncodingUnsupportedValueError)
	return ok
}

// InvalidBlockHeightError indicates an invalid block height for the current epoch
type InvalidBlockHeightError struct {
	MinHeight       uint64
	MaxHeight       uint64
	RequestedHeight uint64
}

func (e *InvalidBlockHeightError) Error() string {
	return fmt.Sprintf(
		"requested height (%d) is not in the range(%d, %d)",
		e.RequestedHeight,
		e.MinHeight,
		e.MaxHeight,
	)
}

// Code returns the error code for this error
func (e *InvalidBlockHeightError) Code() uint32 {
	return errCodeInvalidBlockHeightError
}

// Is returns true if the given error type is InvalidBlockHeightError
func (e *InvalidBlockHeightError) Is(target error) bool {
	_, ok := target.(*InvalidBlockHeightError)
	return ok
}
