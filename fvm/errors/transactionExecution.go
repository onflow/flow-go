package errors

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/model/flow"
)

// TxExecutionError captures errors when executing a transaction.
// A transaction having this error has already passed validation and is included in a collection.
// the transaction will be executed by execution nodes but the result is reverted
// and in some cases there will be a penalty (or fees) for the payer, access nodes or collection nodes.
type TransactionExecutionError interface {
	// returns runtime error if the source of this error is runtime
	// return nil for all other errors
	RuntimeError() error
	TransactionError
}

// CadenceRunTimeError captures a collection of errors provided by cadence runtime
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

func (e *CadenceRuntimeError) Error() string {
	return fmt.Sprintf("cadence runtime error %s", e.Err.Error())
}

// TODO return a proper code
func (e *CadenceRuntimeError) Code() uint32 {
	return 0
}

// An StorageCapacityExceededError indicates that an account used more storage than it has storage capacity.
type StorageCapacityExceededError struct {
	Address         flow.Address
	StorageUsed     uint64
	StorageCapacity uint64
}

func (e *StorageCapacityExceededError) Error() string {
	return fmt.Sprintf("address %s storage %d is over capacity %d", e.Address, e.StorageUsed, e.StorageCapacity)
}

func (e *StorageCapacityExceededError) Code() uint32 {
	return errCodeStorageCapacityExceeded
}

// - if user can pay for the tx fees
type InsufficientTokenBalanceError struct {
}

type MaxGasExceededError struct {
}

// EventLimitExceededError indicates that the transaction has produced events with size more than limit.
type EventLimitExceededError struct {
	TotalByteSize uint64
	Limit         uint64
}

func (e *EventLimitExceededError) Error() string {
	return fmt.Sprintf(
		"total event byte size (%d) exceeds limit (%d)",
		e.TotalByteSize,
		e.Limit,
	)
}

// Code returns the error code for this error
func (e *EventLimitExceededError) Code() uint32 {
	return errCodeEventLimitExceededError
}

type MaxLedgerIntractionLimitExceededError struct {
}

// A StateKeySizeLimitError indicates that the provided key has exceeded the size limit allowed by the storage
type StateKeySizeLimitError struct {
	Owner      string
	Controller string
	Key        string
	Size       uint64
	Limit      uint64
}

func (e *StateKeySizeLimitError) Error() string {
	return fmt.Sprintf("key %s has size %d which is higher than storage key size limit %d.", fullKey(e.Owner, e.Controller, e.Key), e.Size, e.Limit)
}

func (e *StateKeySizeLimitError) Code() uint32 {
	return errCodeLedgerIntractionLimitExceededError
}

// A StateValueSizeLimitError indicates that the provided value has exceeded the size limit allowed by the storage
type StateValueSizeLimitError struct {
	Value flow.RegisterValue
	Size  uint64
	Limit uint64
}

func (e *StateValueSizeLimitError) Error() string {
	return fmt.Sprintf("value %s has size %d which is higher than storage value size limit %d.", string(e.Value[0:10])+"..."+string(e.Value[len(e.Value)-10:]), e.Size, e.Limit)
}

// A StateInteractionLimitExceededError
type StateInteractionLimitExceededError struct {
	Used  uint64
	Limit uint64
}

func (e *StateInteractionLimitExceededError) Error() string {
	return fmt.Sprintf("max interaction with storage has exceeded the limit (used: %d, limit %d)", e.Used, e.Limit)
}

// OperationNotSupportedError is generated when an operation (e.g. getting block info) is
// not supported in the current environment.
type OperationNotSupportedError struct {
	Operation string
}

func (e *OperationNotSupportedError) Error() string {
	return fmt.Sprintf("%s is not supported in this environment.", e.Operation)
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

func (e *EncodingUnsupportedValueError) Code() uint32 {
	return errCodeEncodingUnsupportedValue
}

// Notes (ramtin)
// when runtime errors are retured, we check the internal errors and if
// type is external means that we have cause the error in the first place
// probably inside the env (so we put it back???)

func fullKey(owner, controller, key string) string {
	return strings.Join([]string{owner, controller, key}, "\x1F")
}
