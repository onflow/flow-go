package errors

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// NewInvalidAddressErrorf constructs a new CodedError which indicates that a
// transaction references an invalid flow Address in either the Authorizers or
// Payer field.
func NewInvalidAddressErrorf(
	address flow.Address,
	msg string,
	args ...any,
) CodedError {
	return NewCodedError(
		ErrCodeInvalidAddressError,
		"invalid address (%s): "+msg,
		append([]any{address.String()}, args...)...)
}

// NewInvalidArgumentErrorf constructs a new CodedError which indicates that a
// transaction includes invalid arguments. This error is the result of failure
// in any of the following conditions:
// - number of arguments doesn't match the template
//
// TODO add more cases like argument size
func NewInvalidArgumentErrorf(msg string, args ...any) CodedError {
	return NewCodedError(
		ErrCodeInvalidArgumentError,
		"transaction arguments are invalid: ("+msg+")",
		args...)
}

func IsInvalidArgumentError(err error) bool {
	return HasErrorCode(err, ErrCodeInvalidArgumentError)
}

// NewInvalidLocationErrorf constructs a new CodedError which indicates an
// invalid location is passed.
func NewInvalidLocationErrorf(
	location runtime.Location,
	msg string,
	args ...any,
) CodedError {
	locationStr := ""
	if location != nil {
		locationStr = location.String()
	}

	return NewCodedError(
		ErrCodeInvalidLocationError,
		"location (%s) is not a valid location: "+msg,
		append([]any{locationStr}, args...)...)
}

// NewValueErrorf constructs a new CodedError which indicates a value is not
// valid value.
func NewValueErrorf(
	valueStr string,
	msg string,
	args ...any,
) CodedError {
	return NewCodedError(
		ErrCodeValueError,
		"invalid value (%s): "+msg,
		append([]any{valueStr}, args...)...)
}

func IsValueError(err error) bool {
	return HasErrorCode(err, ErrCodeValueError)
}

// NewOperationAuthorizationErrorf constructs a new CodedError which indicates
// not enough authorization to perform an operations like account creation or
// smart contract deployment.
func NewOperationAuthorizationErrorf(
	operation string,
	msg string,
	args ...any,
) CodedError {
	return NewCodedError(
		ErrCodeOperationAuthorizationError,
		"(%s) is not authorized: "+msg,
		append([]any{operation}, args...)...)
}

// NewAccountAuthorizationErrorf constructs a new CodedError which indicates
// that an authorization issue either:
//   - a transaction is missing a required signature to authorize access to an
//     account, or
//   - a transaction doesn't have authorization to performe some operations like
//     account creation.
func NewAccountAuthorizationErrorf(
	address flow.Address,
	msg string,
	args ...any,
) CodedError {
	return NewCodedError(
		ErrCodeAccountAuthorizationError,
		"authorization failed for account %s: "+msg,
		append([]any{address}, args...)...)
}
