package errors

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// NewInvalidAddressErrorf constructs a new CodedError which indicates that a
// transaction references an invalid flow Address in either the Authorizers or
// Payer field.
func NewInvalidAddressErrorf(
	address flow.Address,
	msg string,
	args ...interface{},
) *CodedError {
	return NewCodedError(
		ErrCodeInvalidAddressError,
		"invalid address (%s): "+msg,
		append([]interface{}{address.String()}, args...)...)
}

// InvalidArgumentError indicates that a transaction includes invalid arguments.
// this error is the result of failure in any of the following conditions:
// - number of arguments doesn't match the template
// TODO add more cases like argument size
type InvalidArgumentError struct {
	errorWrapper
}

// NewInvalidArgumentErrorf constructs a new InvalidArgumentError
func NewInvalidArgumentErrorf(msg string, args ...interface{}) InvalidArgumentError {
	return InvalidArgumentError{
		errorWrapper: errorWrapper{
			err: fmt.Errorf(msg, args...),
		},
	}
}

func (e InvalidArgumentError) Error() string {
	return fmt.Sprintf("%s transaction arguments are invalid: (%s)", e.Code().String(), e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidArgumentError) Code() ErrorCode {
	return ErrCodeInvalidArgumentError
}

// NewInvalidLocationErrorf constructs a new CodedError which indicates an
// invalid location is passed.
func NewInvalidLocationErrorf(
	location runtime.Location,
	msg string,
	args ...interface{},
) *CodedError {
	locationStr := ""
	if location != nil {
		locationStr = location.String()
	}

	return NewCodedError(
		ErrCodeInvalidLocationError,
		"location (%s) is not a valid location: "+msg,
		append([]interface{}{locationStr}, args...)...)
}

// ValueError indicates a value is not valid value.
type ValueError struct {
	errorWrapper

	valueStr string
}

// NewValueErrorf constructs a new ValueError
func NewValueErrorf(valueStr string, msg string, args ...interface{}) ValueError {
	return ValueError{valueStr: valueStr,
		errorWrapper: errorWrapper{
			err: fmt.Errorf(msg, args...),
		},
	}
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

func IsValueError(err error) bool {
	var valErr ValueError
	return As(err, &valErr)
}

// NewOperationAuthorizationErrorf constructs a new CodedError which indicates
// not enough authorization to perform an operations like account creation or
// smart contract deployment.
func NewOperationAuthorizationErrorf(
	operation string,
	msg string,
	args ...interface{},
) *CodedError {
	return NewCodedError(
		ErrCodeOperationAuthorizationError,
		"(%s) is not authorized: "+msg,
		append([]interface{}{operation}, args...)...)
}

// AccountAuthorizationError indicates that an authorization issues
// either a transaction is missing a required signature to
// authorize access to an account or a transaction doesn't have authorization
// to performe some operations like account creation.
type AccountAuthorizationError struct {
	errorWrapper

	address flow.Address
}

// NewAccountAuthorizationErrorf constructs a new AccountAuthorizationError
func NewAccountAuthorizationErrorf(address flow.Address, msg string, args ...interface{}) AccountAuthorizationError {
	return AccountAuthorizationError{
		address: address,
		errorWrapper: errorWrapper{
			err: fmt.Errorf(msg, args...),
		},
	}
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
