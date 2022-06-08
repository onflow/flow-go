package errors

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidAddressError indicates that a transaction references an invalid flow Address
// in either the Authorizers or Payer field.
type InvalidAddressError struct {
	address flow.Address
	err     error
}

// Address returns the invalid address
func (e InvalidAddressError) Address() flow.Address {
	return e.address
}

// NewInvalidAddressErrorf constructs a new InvalidAddressError
func NewInvalidAddressErrorf(address flow.Address, msg string, args ...interface{}) *InvalidAddressError {
	return &InvalidAddressError{address: address, err: fmt.Errorf(msg, args...)}
}

func (e InvalidAddressError) Error() string {
	return fmt.Sprintf("%s invalid address (%s): %s", e.Code().String(), e.address.String(), e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidAddressError) Code() ErrorCode {
	return ErrCodeInvalidAddressError
}

// Unwrap unwraps the error
func (e InvalidAddressError) Unwrap() error {
	return e.err
}

// InvalidArgumentError indicates that a transaction includes invalid arguments.
// this error is the result of failure in any of the following conditions:
// - number of arguments doesn't match the template
// TODO add more cases like argument size
type InvalidArgumentError struct {
	err error
}

// NewInvalidArgumentErrorf constructs a new InvalidArgumentError
func NewInvalidArgumentErrorf(msg string, args ...interface{}) *InvalidArgumentError {
	return &InvalidArgumentError{err: fmt.Errorf(msg, args...)}
}

func (e InvalidArgumentError) Error() string {
	return fmt.Sprintf("%s transaction arguments are invalid: (%s)", e.Code().String(), e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidArgumentError) Code() ErrorCode {
	return ErrCodeInvalidArgumentError
}

// Unwrap unwraps the error
func (e InvalidArgumentError) Unwrap() error {
	return e.err
}

// InvalidLocationError indicates an invalid location is passed
type InvalidLocationError struct {
	location runtime.Location
	err      error
}

// NewInvalidLocationErrorf constructs a new InvalidLocationError
func NewInvalidLocationErrorf(location runtime.Location, msg string, args ...interface{}) *InvalidLocationError {
	return &InvalidLocationError{location: location, err: fmt.Errorf(msg, args...)}
}

func (e InvalidLocationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}

	locationStr := ""
	if e.location != nil {
		locationStr = e.location.String()
	}

	return fmt.Sprintf(
		"%s location (%s) is not a valid location: %s",
		e.Code().String(),
		locationStr,
		errMsg,
	)
}

// Code returns the error code for this error
func (e InvalidLocationError) Code() ErrorCode {
	return ErrCodeInvalidLocationError
}

// Unwrap unwraps the error
func (e InvalidLocationError) Unwrap() error {
	return e.err
}

// ValueError indicates a value is not valid value.
type ValueError struct {
	valueStr string
	err      error
}

// NewValueErrorf constructs a new ValueError
func NewValueErrorf(valueStr string, msg string, args ...interface{}) *ValueError {
	return &ValueError{valueStr: valueStr, err: fmt.Errorf(msg, args...)}
}

func (e ValueError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf("%s invalid value (%s): %s", e.Code().String(), e.valueStr, errMsg)
}

// Code returns the error code for this error type
func (e ValueError) Code() ErrorCode {
	return ErrCodeValueError
}

// Unwrap unwraps the error
func (e ValueError) Unwrap() error {
	return e.err
}

// OperationAuthorizationError indicates not enough authorization
// to perform an operations like account creation or smart contract deployment.
type OperationAuthorizationError struct {
	operation string
	err       error
}

// NewOperationAuthorizationErrorf constructs a new OperationAuthorizationError
func NewOperationAuthorizationErrorf(operation string, msg string, args ...interface{}) *OperationAuthorizationError {
	return &OperationAuthorizationError{operation: operation, err: fmt.Errorf(msg, args...)}
}

func (e OperationAuthorizationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf(
		"%s (%s) is not authorized: %s",
		e.Code().String(),
		e.operation,
		errMsg,
	)
}

// Code returns the error code for this error type
func (e OperationAuthorizationError) Code() ErrorCode {
	return ErrCodeOperationAuthorizationError
}

// Unwrap unwraps the error
func (e OperationAuthorizationError) Unwrap() error {
	return e.err
}

// AccountAuthorizationError indicates that an authorization issues
// either a transaction is missing a required signature to
// authorize access to an account or a transaction doesn't have authorization
// to performe some operations like account creation.
type AccountAuthorizationError struct {
	address flow.Address
	err     error
}

// NewAccountAuthorizationErrorf constructs a new AccountAuthorizationError
func NewAccountAuthorizationErrorf(address flow.Address, msg string, args ...interface{}) *AccountAuthorizationError {
	return &AccountAuthorizationError{address: address, err: fmt.Errorf(msg, args...)}
}

// Address returns the address of an account without enough authorization
func (e AccountAuthorizationError) Address() flow.Address {
	return e.address
}

func (e AccountAuthorizationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf(
		"%s authorization failed for account %s: %s",
		e.Code().String(),
		e.address,
		errMsg,
	)
}

// Code returns the error code for this error type
func (e AccountAuthorizationError) Code() ErrorCode {
	return ErrCodeAccountAuthorizationError
}

// Unwrap unwraps the error
func (e AccountAuthorizationError) Unwrap() error {
	return e.err
}

// FVMInternalError indicates that an internal error occurs during tx execution.
type FVMInternalError struct {
	msg string
	err error
}

// NewFVMInternalErrorf constructs a new FVMInternalError
func NewFVMInternalErrorf(msg string, args ...interface{}) *FVMInternalError {
	return &FVMInternalError{err: fmt.Errorf(msg, args...)}
}

func (e *FVMInternalError) Error() string {
	return e.msg
}

// Code returns the error code for this error type
func (e *FVMInternalError) Code() ErrorCode {
	return ErrCodeFVMInternalError
}
