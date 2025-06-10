package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// RequireNoError returns nil if error is nil, otherwise throws an irrecoverable exception
func RequireNoError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	irrecoverable.Throw(ctx, err)
	return irrecoverable.NewException(err)
}

// RequireErrorIs returns the error if it unwraps to any of the provided target error types
// Otherwise, it throws an irrecoverable exception
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

// RequireAccessError returns the error if it is an Access sentinel error
// Otherwise, it throws an irrecoverable exception
// Note: this method will not unwrap the error. the passed error must be an instance of a sentinel
// error.
func RequireAccessError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if _, ok := err.(accessSentinel); ok {
		return err
	}

	irrecoverable.Throw(ctx, err)
	return irrecoverable.NewException(err)
}

// accessSentinel is a marker interface for errors returned by the Access API
// This is used to differentiate unexpected errors returned by endpoints
// Implement this for all new Access sentinel errors. Any that do not will be considered unexpected
// exceptions.
type accessSentinel interface {
	accessSentinel()
}

// InvalidRequestError indicates that the client's request was malformed or invalid
type InvalidRequestError struct {
	err error
}

func NewInvalidRequestError(err error) InvalidRequestError {
	return InvalidRequestError{err: err}
}

func (e InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid argument: %v", e.err)
}

func (e InvalidRequestError) Unwrap() error {
	return e.err
}

func (e InvalidRequestError) accessSentinel() {}

func IsInvalidRequestError(err error) bool {
	var errInvalidRequest InvalidRequestError
	return errors.As(err, &errInvalidRequest)
}

// DataNotFoundError indicates that the requested data was not found on the system
type DataNotFoundError struct {
	dataType string
	err      error
}

func NewDataNotFoundError(dataType string, err error) DataNotFoundError {
	return DataNotFoundError{dataType: dataType, err: err}
}

func (e DataNotFoundError) Error() string {
	return fmt.Sprintf("data not found for %s: %v", e.dataType, e.err)
}

func (e DataNotFoundError) Unwrap() error {
	return e.err
}

func (e DataNotFoundError) accessSentinel() {}

func IsDataNotFoundError(err error) bool {
	var errDataNotFound DataNotFoundError
	return errors.As(err, &errDataNotFound)
}

// InternalError indicates that a non-fatal internal error occurred
// IMPORTANT: this should only be used for benign internal errors. Fatal or irrecoverable system
// errors must be handled explicitly.
type InternalError struct {
	err error
}

func NewInternalError(err error) InternalError {
	return InternalError{err: err}
}

func (e InternalError) Error() string {
	return fmt.Sprintf("internal error: %v", e.err)
}

func (e InternalError) Unwrap() error {
	return e.err
}

func (e InternalError) accessSentinel() {}

func IsInternalError(err error) bool {
	var errInternalError InternalError
	return errors.As(err, &errInternalError)
}

// OutOfRangeError indicates that the request was for data that is outside of the available range.
// This is a more specific version of DataNotFoundError, where the data is known to eventually exist, but
// currently is not known.
// For example, querying data for a height above the current finalized height.
type OutOfRangeError struct {
	err error
}

func NewOutOfRangeError(err error) OutOfRangeError {
	return OutOfRangeError{err: err}
}

func (e OutOfRangeError) Error() string {
	return fmt.Sprintf("out of range: %v", e.err)
}

func (e OutOfRangeError) Unwrap() error {
	return e.err
}

func (e OutOfRangeError) accessSentinel() {}

func IsOutOfRangeError(err error) bool {
	var errOutOfRangeError OutOfRangeError
	return errors.As(err, &errOutOfRangeError)
}

// PreconditionFailedError indicates that a precondition for the operation was not met
// This is a more specific version of InvalidRequestError, where the request is valid, but the system
// is not currently in a state to fulfill the request (but may be in the future).
type PreconditionFailedError struct {
	err error
}

func NewPreconditionFailedError(err error) PreconditionFailedError {
	return PreconditionFailedError{err: err}
}

func (e PreconditionFailedError) Error() string {
	return fmt.Sprintf("precondition failed: %v", e.err)
}

func (e PreconditionFailedError) Unwrap() error {
	return e.err
}

func (e PreconditionFailedError) accessSentinel() {}

func IsPreconditionFailedError(err error) bool {
	var errPreconditionFailed PreconditionFailedError
	return errors.As(err, &errPreconditionFailed)
}

// RequestCanceledError indicates that the request was canceled before the server finished processing it
type RequestCanceledError struct {
	err error
}

func NewRequestCanceledError(err error) RequestCanceledError {
	return RequestCanceledError{err: err}
}

func (e RequestCanceledError) Error() string {
	return fmt.Sprintf("request canceled: %v", e.err)
}

func (e RequestCanceledError) Unwrap() error {
	return e.err
}

func (e RequestCanceledError) accessSentinel() {}

func IsRequestCanceledError(err error) bool {
	var requestCanceledError RequestCanceledError
	return errors.As(err, &requestCanceledError)
}

// RequestTimedOutError indicates that the request timed out before the server finished processing it
type RequestTimedOutError struct {
	err error
}

func NewRequestTimedOutError(err error) RequestTimedOutError {
	return RequestTimedOutError{err: err}
}

func (e RequestTimedOutError) Error() string {
	return fmt.Sprintf("request timedout: %v", e.err)
}

func (e RequestTimedOutError) Unwrap() error {
	return e.err
}

func (e RequestTimedOutError) accessSentinel() {}

func IsRequestTimedOutError(err error) bool {
	var requestTimedOutError RequestTimedOutError
	return errors.As(err, &requestTimedOutError)
}
