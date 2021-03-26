package errors

import (
	"errors"
	stdErrors "errors"
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

// NewInvalidAddressErrorf constructs a new InvalidAddressError
func NewInvalidAddressErrorf(msg string, err error, address flow.Address) error {
	return &InvalidAddressError{err: fmt.Errorf(msg, err), address: address}
}

// NewInvalidAddressError constructs a new InvalidAddressError
func NewInvalidAddressError(msg string, address flow.Address) error {
	return &InvalidAddressError{err: errors.New(msg), address: address}
}

func (e InvalidAddressError) Error() string {
	return fmt.Sprintf("invalid address (%s): %s", e.address.String(), e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidAddressError) Code() uint32 {
	return errCodeInvalidAddressError
}

// InvalidArgumentError indicates that a transaction includes invalid arguments.
// this error is the result of failure in any of the following conditions:
// - number of arguments doesn't match the template
// TODO add more cases like argument size
type InvalidArgumentError struct {
	err error
}

func (e InvalidArgumentError) Error() string {
	return fmt.Sprintf("transaction arguments are invalid: (%s)", e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidArgumentError) Code() uint32 {
	return errCodeInvalidArgumentError
}

// InvalidLocationError indicates an invalid location is passed
type InvalidLocationError struct {
	location runtime.Location
	err      error
}

// NewInvalidLocationError constructs a new InvalidLocationError
func NewInvalidLocationError(msg string, location runtime.Location) error {
	return &InvalidLocationError{
		location: location,
		err:      stdErrors.New(msg),
	}
}

// NewInvalidLocationErrorf constructs a new InvalidLocationError (with inner error formatting)
func NewInvalidLocationErrorf(msg string, err error, location runtime.Location) error {
	return &InvalidLocationError{
		location: location,
		err:      fmt.Errorf(msg, err),
	}
}

func (e InvalidLocationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}

	return fmt.Sprintf(
		"location (%s) is not a valid location: %s",
		e.location.String(),
		errMsg,
	)
}

// Code returns the error code for this error
func (e InvalidLocationError) Code() uint32 {
	return errCodeInvalidLocationError
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

// NewValueError constructs a new ValueError
func NewValueError(msg string, valueStr string) error {
	return &ValueError{
		valueStr: valueStr,
		err:      stdErrors.New(msg),
	}
}

// NewValueErrorf constructs a new ValueError (with inner error formatting)
func NewValueErrorf(msg string, err error, valueStr string) error {
	return &ValueError{
		valueStr: valueStr,
		err:      fmt.Errorf(msg, err),
	}
}

func (e ValueError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf("invalid value (%s): %s", e.valueStr, errMsg)
}

// Code returns the error code for this error type
func (e ValueError) Code() uint32 {
	return errCodeValueError
}

// Unwrap unwraps the error
func (e ValueError) Unwrap() error {
	return e.err
}

// OperationAuthorizationError indicates not enough authorization
// to performe an operations like account creation or smart contract deployment.
type OperationAuthorizationError struct {
	operation string
	err       error
}

// NewOperationAuthorizationError constructs a new OperationAuthorizationError
func NewOperationAuthorizationError(msg string, operation string) error {
	return &OperationAuthorizationError{
		operation: operation,
		err:       stdErrors.New(msg),
	}
}

// NewOperationAuthorizationErrorf constructs a new OperationAuthorizationError (inner error formatting)
func NewOperationAuthorizationErrorf(msg string, err error, operation string) error {
	return &OperationAuthorizationError{
		operation: operation,
		err:       fmt.Errorf(msg, err),
	}
}

func (e OperationAuthorizationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf(
		"(%s) is not authorized: %s",
		e.operation,
		errMsg,
	)
}

// Code returns the error code for this error type
func (e OperationAuthorizationError) Code() uint32 {
	return errCodeAccountAuthorizationError
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

// NewAccountAuthorizationError constructs a new account authorization error
func NewAccountAuthorizationError(msg string, address flow.Address) error {
	return &AccountAuthorizationError{
		address: address,
		err:     stdErrors.New(msg),
	}
}

// NewAccountAuthorizationErrorf constructs a new account authorization error (inner error formatting)
func NewAccountAuthorizationErrorf(msg string, err error, address flow.Address) error {
	return &AccountAuthorizationError{
		address: address,
		err:     fmt.Errorf(msg, err),
	}
}

func (e AccountAuthorizationError) Error() string {
	errMsg := ""
	if e.err != nil {
		errMsg = e.err.Error()
	}
	return fmt.Sprintf(
		"authorization failed for account %s: %s",
		e.address,
		errMsg,
	)
}

// Code returns the error code for this error type
func (e AccountAuthorizationError) Code() uint32 {
	return errCodeAccountAuthorizationError
}

// Unwrap unwraps the error
func (e AccountAuthorizationError) Unwrap() error {
	return e.err
}
