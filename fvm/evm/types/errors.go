package types

import (
	"errors"
	"fmt"
)

type ErrorCode uint64

// internal error codes
const ( // code reserved for no error
	ErrCodeNoError ErrorCode = 0

	// covers all other validation codes that doesn't have an specific code
	ValidationErrCodeMisc ErrorCode = 100
	// invalid balance is provided (e.g. negative value)
	ValidationErrCodeInvalidBalance ErrorCode = 101
	// insufficient computation is left in the flow transaction
	ValidationErrCodeInsufficientComputation ErrorCode = 102
	// unauthroized method call
	ValidationErrCodeUnAuthroizedMethodCall ErrorCode = 103
	// withdraw balance is prone to rounding error
	ValidationErrCodeWithdrawBalanceRounding ErrorCode = 104

	// general execution error returned for cases that don't have an specific code
	ExecutionErrCodeMisc ErrorCode = 400
)

// geth evm core errors (reserved range: [201-300) )
const (
	// the nonce of the tx is lower than the expected
	ValidationErrCodeNonceTooLow = iota + 201
	// the nonce of the tx is higher than the expected
	ValidationErrCodeNonceTooHigh
	// tx sender account has reached to the maximum nonce
	ValidationErrCodeNonceMax
	// not enough gas is available on the block to include this transaction
	ValidationErrCodeGasLimitReached
	// the transaction sender doesn't have enough funds for transfer(topmost call only).
	ValidationErrCodeInsufficientFundsForTransfer
	// creation transaction provides the init code bigger than init code size limit.
	ValidationErrCodeMaxInitCodeSizeExceeded
	// the total cost of executing a transaction is higher than the balance of the user's account.
	ValidationErrCodeInsufficientFunds
	// overflow detected when calculating the gas usage
	ValidationErrCodeGasUintOverflow
	// the transaction is specified to use less gas than required to start the invocation.
	ValidationErrCodeIntrinsicGas
	// the transaction is not supported in the current network configuration.
	ValidationErrCodeTxTypeNotSupported
	// tip was set to higher than the total fee cap
	ValidationErrCodeTipAboveFeeCap
	// an extremely big numbers is set for the tip field
	ValidationErrCodeTipVeryHigh
	// an extremely big numbers is set for the fee cap field
	ValidationErrCodeFeeCapVeryHigh
	// the transaction fee cap is less than the base fee of the block
	ValidationErrCodeFeeCapTooLow
	// the sender of a transaction is a contract
	ValidationErrCodeSenderNoEOA
	// the transaction fee cap is less than the blob gas fee of the block.
	ValidationErrCodeBlobFeeCapTooLow
)

// evm execution errors (reserved range: [301-400) )
const (
	// execution ran out of gas
	ExecutionErrCodeOutOfGas ErrorCode = iota + 301
	// contract creation code storage out of gas
	ExecutionErrCodeCodeStoreOutOfGas
	// max call depth exceeded
	ExecutionErrCodeDepth
	// insufficient balance for transfer
	ExecutionErrCodeInsufficientBalance
	// contract address collision"
	ExecutionErrCodeContractAddressCollision
	// execution reverted
	ExecutionErrCodeExecutionReverted
	// max initcode size exceeded
	ExecutionErrCodeMaxInitCodeSizeExceeded
	// max code size exceeded
	ExecutionErrCodeMaxCodeSizeExceeded
	// invalid jump destination
	ExecutionErrCodeInvalidJump
	// write protection
	ExecutionErrCodeWriteProtection
	// return data out of bounds
	ExecutionErrCodeReturnDataOutOfBounds
	// gas uint64 overflow
	ExecutionErrCodeGasUintOverflow
	// invalid code: must not begin with 0xef
	ExecutionErrCodeInvalidCode
	// nonce uint64 overflow
	ExecutionErrCodeNonceUintOverflow
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

// IsABackendError returns true if the error or
// any underlying errors is a backend error
func IsABackendError(err error) bool {
	return errors.As(err, &BackendError{})
}
