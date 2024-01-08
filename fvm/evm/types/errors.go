package types

import (
	"errors"
	"fmt"
)

var (
	// ErrAccountDoesNotExist is returned when evm account doesn't exist
	ErrAccountDoesNotExist = errors.New("account does not exist")

	// ErrInsufficientBalance is returned when evm account doesn't have enough balance
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrInsufficientComputation is returned when not enough computation is
	// left in the context of flow transaction to execute the evm operation.
	ErrInsufficientComputation = errors.New("insufficient computation")

	// unauthorized method call, usually emited when calls are called on EOA accounts
	ErrUnAuthroizedMethodCall = errors.New("unauthroized method call")
	// ErrInsufficientTotalSupply is returned when flow token
	// is withdraw request is there but not enough balance is on EVM vault
	// this should never happen but its a saftey measure to protect Flow against EVM issues.
	// TODO: we might consider this fatal
	ErrInsufficientTotalSupply = errors.New("insufficient total supply")

	// ErrBalanceConversion is returned conversion of balance has failed, usually
	// is returned when the balance presented in attoflow has values that could
	// be marginally lost on the conversion.
	ErrBalanceConversion = errors.New("balance converion error")

	// ErrNotImplemented is a fatal error when something is called that is not implemented
	ErrNotImplemented = NewFatalError(errors.New("a functionality is called that is not implemented"))
)

// EVMExecutionError is a non-fatal error, returned when execution of
// an evm transaction or direct call has failed.
type EVMExecutionError struct {
	err error
}

// NewEVMExecutionError returns a new EVMExecutionError
func NewEVMExecutionError(rootCause error) EVMExecutionError {
	return EVMExecutionError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying evm error
func (err EVMExecutionError) Unwrap() error {
	return err.err
}

func (err EVMExecutionError) Error() string {
	return fmt.Sprintf("EVM execution error: %v", err.err)
}

// IsEVMValidationError returns true if the error or any underlying errors
// is of the type EVM execution error
func IsEVMExecutionError(err error) bool {
	return errors.As(err, &EVMExecutionError{})
}

// EVMValidationError is a non-fatal error, returned when validation steps of an EVM transaction
// or direct call has failed.
type EVMValidationError struct {
	err error
}

// NewEVMValidationError returns a new EVMValidationError
func NewEVMValidationError(rootCause error) EVMValidationError {
	return EVMValidationError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying evm error
func (err EVMValidationError) Unwrap() error {
	return err.err
}

func (err EVMValidationError) Error() string {
	return fmt.Sprintf("EVM validation error: %v", err.err)
}

// IsEVMValidationError returns true if the error or any underlying errors
// is of the type EVM validation error
func IsEVMValidationError(err error) bool {
	return errors.As(err, &EVMValidationError{})
}

// StateError is a non-fatal error, returned when a state operation
// has failed (e.g. reaching storage interaction limit)
type StateError struct {
	err error
}

// NewStateError returns a new StateError
func NewStateError(rootCause error) StateError {
	return StateError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying evm error
func (err StateError) Unwrap() error {
	return err.err
}

func (err StateError) Error() string {
	return fmt.Sprintf("state error: %v", err.err)
}

// IsAStateError returns true if the error or any underlying errors
// is of the type EVM validation error
func IsAStateError(err error) bool {
	return errors.As(err, &StateError{})
}

// FatalError is user for any error that is not user related and something
// unusual has happend. Usually we stop the node when this happens
// given it might have a non-deterministic root.
type FatalError struct {
	err error
}

// NewFatalError returns a new FatalError
func NewFatalError(rootCause error) FatalError {
	return FatalError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying fatal error
func (err FatalError) Unwrap() error {
	return err.err
}

func (err FatalError) Error() string {
	return fmt.Sprintf("fatal error: %v", err.err)
}

// IsAFatalError returns true if the error or underlying error
// is of fatal type.
func IsAFatalError(err error) bool {
	return errors.As(err, &FatalError{})
}

// IsAInsufficientTotalSupplyError returns true if the
// error type is InsufficientTotalSupplyError
func IsAInsufficientTotalSupplyError(err error) bool {
	return errors.Is(err, ErrInsufficientTotalSupply)
}

// IsAUnAuthroizedMethodCallError returns true if the error type is
// UnAuthroizedMethodCallError
func IsAUnAuthroizedMethodCallError(err error) bool {
	return errors.Is(err, ErrUnAuthroizedMethodCall)
}
