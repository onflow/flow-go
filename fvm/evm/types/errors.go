package types

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidBalance is returned when an invalid balance is provided for transfer (e.g. negative)
	ErrInvalidBalance = NewEVMValidationError(errors.New("invalid balance for transfer"))

	// ErrInsufficientComputation is returned when not enough computation is
	// left in the context of flow transaction to execute the evm operation.
	ErrInsufficientComputation = NewEVMValidationError(errors.New("insufficient computation"))

	// ErrUnAuthroizedMethodCall method call, usually emited when calls are called on EOA accounts
	ErrUnAuthroizedMethodCall = NewEVMValidationError(errors.New("unauthroized method call"))

	// ErrWithdrawBalanceRounding is returned when withdraw call has a balance that could
	// yeild to rounding error, i.e. the balance contains fractions smaller than 10^8 Flow (smallest unit allowed to transfer).
	ErrWithdrawBalanceRounding = NewEVMValidationError(errors.New("withdraw failed! the balance is susceptible to the rounding error"))

	// ErrInsufficientTotalSupply is returned when flow token
	// is withdraw request is there but not enough balance is on EVM vault
	// this should never happen but its a saftey measure to protect Flow against EVM issues.
	ErrInsufficientTotalSupply = NewFatalError(errors.New("insufficient total supply"))

	// ErrNotImplemented is a fatal error when something is called that is not implemented
	ErrNotImplemented = NewFatalError(errors.New("a functionality is called that is not implemented"))
)

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
// is a state error
func IsAStateError(err error) bool {
	return errors.As(err, &StateError{})
}

// FatalError is used for any error that is not user related and something
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

// IsWithdrawBalanceRoundingError returns true if the error type is
// ErrWithdrawBalanceRounding
func IsWithdrawBalanceRoundingError(err error) bool {
	return errors.Is(err, ErrWithdrawBalanceRounding)
}

// IsAUnAuthroizedMethodCallError returns true if the error type is
// UnAuthroizedMethodCallError
func IsAUnAuthroizedMethodCallError(err error) bool {
	return errors.Is(err, ErrUnAuthroizedMethodCall)
}

// BackendError is a non-fatal error wraps errors returned from the backend
type BackendError struct {
	err error
}

// NewBackendError returns a new BackendError
func NewBackendError(rootCause error) BackendError {
	return BackendError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying evm error
func (err BackendError) Unwrap() error {
	return err.err
}

func (err BackendError) Error() string {
	return fmt.Sprintf("backend error: %v", err.err)
}

// IsABackendError returns true if the error or any underlying errors
// is a backend error
func IsABackendError(err error) bool {
	return errors.As(err, &BackendError{})
}
