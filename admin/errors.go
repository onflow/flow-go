package admin

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// TODO: move RequireNoError and RequireErrorIs to common package and use one.
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

// InvalidAdminReqError indicates that an admin request has failed validation, and
// the request will not be processed. All Validator functions must return this error
// if the request was invalid.
type InvalidAdminReqError struct {
	Err error
}

func NewInvalidAdminReqErrorf(msg string, args ...any) InvalidAdminReqError {
	return InvalidAdminReqError{
		Err: fmt.Errorf(msg, args...),
	}
}

// NewInvalidAdminReqParameterError returns an InvalidAdminReqError indicating that
// a field of the request has an invalid value.
func NewInvalidAdminReqParameterError(field string, msg string, actualVal any) InvalidAdminReqError {
	return NewInvalidAdminReqErrorf("invalid value for field '%s': %s. Got: %v", field, msg, actualVal)
}

// NewInvalidAdminReqFormatError returns an InvalidAdminReqError indicating that
// the request data format is invalid.
func NewInvalidAdminReqFormatError(msg string, args ...any) InvalidAdminReqError {
	return NewInvalidAdminReqErrorf("invalid request data format: "+msg, args...)
}

func IsInvalidAdminParameterError(err error) bool {
	var target InvalidAdminReqError
	return errors.As(err, &target)
}

func (err InvalidAdminReqError) Error() string {
	return err.Err.Error()
}

func (err InvalidAdminReqError) Unwrap() error {
	return err.Err
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
	return fmt.Sprintf("data not found for %s: %v", e.dataType, e.err)
}

func (e DataNotFoundError) Unwrap() error {
	return e.err
}

// OutOfRangeError indicates that the request was for data that is outside the available range.
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

// RequestCanceledError indicates that the command was canceled before the server finished processing it.
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

// RequestTimedOutError indicates that the request timed out before the server finished processing it.
type RequestTimedOutError struct {
	err error
}

func NewRequestTimedOutError(err error) RequestTimedOutError {
	return RequestTimedOutError{err: err}
}

func (e RequestTimedOutError) Error() string {
	return fmt.Sprintf("request timed out: %v", e.err)
}

func (e RequestTimedOutError) Unwrap() error {
	return e.err
}

// ResourceExhausted indicates when computation or memory limits were exceeded.
type ResourceExhausted struct {
	err error
}

func NewResourceExhausted(err error) ResourceExhausted {
	return ResourceExhausted{err: err}
}

func (e ResourceExhausted) Error() string {
	return fmt.Sprintf("resource exhausted error: %v", e.err)
}

func (e ResourceExhausted) Unwrap() error {
	return e.err
}
