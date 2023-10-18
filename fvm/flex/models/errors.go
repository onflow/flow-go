package models

import (
	"errors"
	"fmt"
)

var (
	// ErrInsufficientBalance is returned when evm account doesn't have enough balance
	ErrInsufficientBalance = errors.New("insufficient balance")

	// ErrInsufficientComputation is returned when not enough computation is
	// left in the context of flow transaction to execute the evm operation.
	ErrInsufficientComputation = errors.New("insufficient computation")

	// unauthorized method call, usually emited when calls are called on EOA accounts
	ErrUnAuthroizedMethodCall = errors.New("unauthroized method call")
	// ErrInsufficientTotalSupply is returned when flow token
	// is withdraw request is there but not enough balance is on Flex vault
	// this should never happen but its a saftey measure to protect Flow against Flex issues.
	// TODO we might consider this fatal
	ErrInsufficientTotalSupply = errors.New("insufficient total supply")

	// ErrBalanceConversion is returned conversion of balance has failed, usually
	// is returned when the balance presented in attoflow has values that could
	// be marginally lost on the conversion.
	ErrBalanceConversion = errors.New("balance converion error")
)

// EVMExecutionError is a user-related error,
// emitted when a evm transaction execution or a contract call has been failed
type EVMExecutionError struct {
	err error
}

func NewEVMExecutionError(rootCause error) EVMExecutionError {
	return EVMExecutionError{
		err: rootCause,
	}
}

func (err EVMExecutionError) Unwrap() error {
	return err.err
}

func (err EVMExecutionError) Error() string {
	return fmt.Sprintf("EVM execution failed: %v", err.err)
}

func IsEVMExecutionError(err error) bool {
	return errors.As(err, &EVMExecutionError{})
}

// FatalError is user for any error that is not user related and something
// unusual has happend. Usually we stop the node when this happens
// given it might have a non-deterministic root.
type FatalError struct {
	err error
}

func NewFatalError(rootCause error) FatalError {
	return FatalError{
		err: rootCause,
	}
}

func (err FatalError) Unwrap() error {
	return err.err
}

func (err FatalError) Error() string {
	return fmt.Sprintf("fatal error: %v", err.err)
}

func IsAFatalError(err error) bool {
	return errors.As(err, &FatalError{})
}

func IsAInsufficientTotalSupplyError(err error) bool {
	return errors.Is(err, ErrInsufficientTotalSupply)
}

func IsAUnAuthroizedMethodCallError(err error) bool {
	return errors.Is(err, ErrUnAuthroizedMethodCall)
}
