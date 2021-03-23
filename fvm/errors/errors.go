package errors

import (
	stdErrors "errors"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
)

// TransactionError is a validation or execution transaction error
type TransactionError interface {
	// Code returns the code for this error
	Code() uint32
	// and anything else that is needed to be an error
	error
}

// VMError captures fatal vm errors,
// we capture this type of error instead of panicing
// to collect all necessary data before crashing
type VMError interface {
	FailureCode() uint16
	error
}

// Is is a utility function for checking the type of err (supports unwrapping)
func Is(source, target error) bool {
	return stdErrors.Is(source, target)
}

// As provides error.As utility functionality
func As(err error, target interface{}) bool {
	return stdErrors.As(err, target)
}

// SplitErrorTypes splits the error into transaction errors and vm fatal errors
func SplitErrorTypes(err error) (txError TransactionError, vmError VMError) {
	switch err.(type) {
	case VMError:
		return nil, err.(VMError)
	case TransactionError:
		return err.(TransactionError), nil
	default:
		if err != nil {
			// capture anything else as unknown failures
			return nil, &UnknownFailure{Err: err}
		}
	}
	return nil, nil
}

// HandleRuntimeError splits the error into transaction errors and vm fatal errors
func HandleRuntimeError(err error) (txError TransactionError, vmErr VMError) {
	var runErr runtime.Error
	var ok bool
	// if is not a runtime error return as vm error
	// this should never happen unless a bug in the code
	if runErr, ok = err.(runtime.Error); !ok {
		return nil, &UnknownFailure{runErr}
	}
	innerErr := runErr.Err

	// External errors are reported by the runtime but originate from the VM.
	// External errors may be fatal or non-fatal, so additional handling by SplitErrorTypes
	if externalErr, ok := innerErr.(interpreter.ExternalError); ok {
		if recoveredErr, ok := externalErr.Recovered.(error); ok {
			// If the recovered value is an error, pass it to the original
			// error handler to distinguish between fatal and non-fatal errors.
			return SplitErrorTypes(recoveredErr)
		}
		// if not recovered return
		return nil, &UnknownFailure{externalErr}
	}

	// All other errors are non-fatal Cadence errors.
	return &CadenceRuntimeError{Err: &runErr}, nil
}
