package models

import (
	"errors"
	"fmt"
)

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
	return fmt.Sprintf("EVM execution has failed: %v", err.err)
}

var (
	// ErrFlexEnvReuse is returned when a flex environment is used more than once
	ErrFlexEnvReuse        = errors.New("flex env has been used")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrGasLimit            = errors.New("gas limit hit")
)
