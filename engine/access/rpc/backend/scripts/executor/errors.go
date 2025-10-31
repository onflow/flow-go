package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// TODO: move RequireNoError and RequireErrorIs to common package (irrecoverable) and update usages.
// RequireNoError returns nil if error is nil, otherwise throws an irrecoverable exception.
func RequireNoError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	irrecoverable.Throw(ctx, err)
	return irrecoverable.NewException(err)
}

// RequireErrorIs returns the error if it unwraps to any of the provided target error types.
// Otherwise, it throws an irrecoverable exception.
func RequireErrorIs(ctx context.Context, err error, targetErrs ...error) error {
	if err == nil {
		return nil
	}

	for _, targetErr := range targetErrs {
		if errors.Is(err, targetErr) {
			return err
		}
	}

	irrecoverable.Throw(ctx, err)
	return irrecoverable.NewException(err)
}

// RequireExecutorError returns the error if it is an Executor sentinel error, otherwise, it throws an
// irrecoverable exception.
//
// This can be used for more complex endpoints that call into other methods to ensure all unexpected
// errors are handled.
// Note: this method will not unwrap the error. the passed error must be an instance of a sentinel
// error.
func RequireExecutorError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(executorSentinel); ok {
		return err
	}

	irrecoverable.Throw(ctx, err)
	return irrecoverable.NewException(err)
}

// executorSentinel is a marker interface for errors returned by the ScriptExecutor.
// This is used to differentiate unexpected errors returned by Execute.
// Implement this for all new ScriptExecutor sentinel errors. Any that do not will be considered unexpected
// exceptions.
type executorSentinel interface {
	executorSentinel()
}

// InvalidArgumentError indicates that the script was malformed or invalid.
type InvalidArgumentError struct {
	err error
}

func NewInvalidArgumentError(err error) InvalidArgumentError {
	return InvalidArgumentError{err: err}
}

func (e InvalidArgumentError) Error() string {
	return fmt.Sprintf("script execution invalid argument: %v", e.err)
}

func (e InvalidArgumentError) Unwrap() error {
	return e.err
}

func (e InvalidArgumentError) executorSentinel() {}

func IsInvalidArgumentError(err error) bool {
	var errInvalidArgument InvalidArgumentError
	return errors.As(err, &errInvalidArgument)
}

// DataNotFoundError indicates that the requested data was not found on the system.
type DataNotFoundError struct {
	dataType string
	err      error
}

func NewDataNotFoundError(dataType string, err error) DataNotFoundError {
	return DataNotFoundError{dataType: dataType, err: err}
}

func (e DataNotFoundError) Error() string {
	return fmt.Sprintf("script execution data not found for %s: %v", e.dataType, e.err)
}

func (e DataNotFoundError) Unwrap() error {
	return e.err
}

func (e DataNotFoundError) executorSentinel() {}

func IsDataNotFoundError(err error) bool {
	var errDataNotFound DataNotFoundError
	return errors.As(err, &errDataNotFound)
}

// InternalError indicates that a non-fatal internal error occurred.
// IMPORTANT: this should only be used for benign internal errors. Fatal or irrecoverable system
// errors must be handled explicitly.
type InternalError struct {
	err error
}

func NewInternalError(err error) InternalError {
	return InternalError{err: err}
}

func (e InternalError) Error() string {
	return fmt.Sprintf("script execution internal error: %v", e.err)
}

func (e InternalError) Unwrap() error {
	return e.err
}

func (e InternalError) executorSentinel() {}

func IsInternalError(err error) bool {
	var errInternalError InternalError
	return errors.As(err, &errInternalError)
}

// OutOfRangeError indicates that the request was for data that is outside the available range.
// This is a more specific version of DataNotFoundError, where the data is known to eventually exist, but
// currently is not known.
type OutOfRangeError struct {
	err error
}

func NewOutOfRangeError(err error) OutOfRangeError {
	return OutOfRangeError{err: err}
}

func (e OutOfRangeError) Error() string {
	return fmt.Sprintf("script execution out of range: %v", e.err)
}

func (e OutOfRangeError) Unwrap() error {
	return e.err
}

func (e OutOfRangeError) executorSentinel() {}

func IsOutOfRangeError(err error) bool {
	var errOutOfRangeError OutOfRangeError
	return errors.As(err, &errOutOfRangeError)
}

// PreconditionFailedError indicates that a precondition for the operation was not met.
// This is a more specific version of InvalidRequestError, where the request is valid, but the system
// is not currently in a state to fulfill the request (but may be in the future).
type PreconditionFailedError struct {
	err error
}

func NewPreconditionFailedError(err error) PreconditionFailedError {
	return PreconditionFailedError{err: err}
}

func (e PreconditionFailedError) Error() string {
	return fmt.Sprintf("script execution precondition failed: %v", e.err)
}

func (e PreconditionFailedError) Unwrap() error {
	return e.err
}

func (e PreconditionFailedError) executorSentinel() {}

func IsPreconditionFailedError(err error) bool {
	var errPreconditionFailed PreconditionFailedError
	return errors.As(err, &errPreconditionFailed)
}

// ScriptExecutionCanceledError indicates that the request was canceled before the server finished processing it.
type ScriptExecutionCanceledError struct {
	err error
}

func NewScriptExecutionCanceledError(err error) ScriptExecutionCanceledError {
	return ScriptExecutionCanceledError{err: err}
}

func (e ScriptExecutionCanceledError) Error() string {
	return fmt.Sprintf("script execution canceled: %v", e.err)
}

func (e ScriptExecutionCanceledError) Unwrap() error {
	return e.err
}

func (e ScriptExecutionCanceledError) executorSentinel() {}

func IsScriptExecutionCanceledError(err error) bool {
	var scriptExecutionCanceledError ScriptExecutionCanceledError
	return errors.As(err, &scriptExecutionCanceledError)
}

// ScriptExecutionTimedOutError indicates that the request timed out before the server finished processing it.
type ScriptExecutionTimedOutError struct {
	err error
}

func NewScriptExecutionTimedOutError(err error) ScriptExecutionTimedOutError {
	return ScriptExecutionTimedOutError{err: err}
}

func (e ScriptExecutionTimedOutError) Error() string {
	return fmt.Sprintf("script execution timed out: %v", e.err)
}

func (e ScriptExecutionTimedOutError) Unwrap() error {
	return e.err
}

func (e ScriptExecutionTimedOutError) executorSentinel() {}

func IsScriptExecutionTimedOutError(err error) bool {
	var scriptExecutionTimedOutError ScriptExecutionTimedOutError
	return errors.As(err, &scriptExecutionTimedOutError)
}

// ServiceUnavailable indicates that a requested service is unavailable.
type ServiceUnavailable struct {
	err error
}

func NewServiceUnavailable(err error) ServiceUnavailable {
	return ServiceUnavailable{err: err}
}

func (e ServiceUnavailable) Error() string {
	return fmt.Sprintf("script execution service unavailable error: %v", e.err)
}

func (e ServiceUnavailable) Unwrap() error {
	return e.err
}

func (e ServiceUnavailable) executorSentinel() {}

func IsServiceUnavailable(err error) bool {
	var errServiceUnavailable ServiceUnavailable
	return errors.As(err, &errServiceUnavailable)
}

// ResourceExhausted indicates when computation or memory limits were exceeded.
type ResourceExhausted struct {
	err error
}

func NewResourceExhausted(err error) ResourceExhausted {
	return ResourceExhausted{err: err}
}

func (e ResourceExhausted) Error() string {
	return fmt.Sprintf("script execution resource exhausted error: %v", e.err)
}

func (e ResourceExhausted) Unwrap() error {
	return e.err
}

func (e ResourceExhausted) executorSentinel() {}

func IsResourceExhausted(err error) bool {
	var errResourceExhausted ResourceExhausted
	return errors.As(err, &errResourceExhausted)
}
