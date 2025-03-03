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

// InvalidRequest indicates that the client's request was malformed or invalid
type InvalidRequest struct {
	err error
}

func NewInvalidRequest(err error) InvalidRequest {
	return InvalidRequest{err: err}
}

func (e InvalidRequest) Error() string {
	return fmt.Sprintf("invalid argument: %v", e.err)
}

func (e InvalidRequest) Unwrap() error {
	return e.err
}

func IsInvalidRequest(err error) bool {
	var errInvalidRequest InvalidRequest
	return errors.As(err, &errInvalidRequest)
}

// DataNotFound indicates that the requested data was not found on the system
type DataNotFound struct {
	dataType string
	err      error
}

func NewDataNotFound(dataType string, err error) DataNotFound {
	return DataNotFound{dataType: dataType, err: err}
}

func (e DataNotFound) Error() string {
	return fmt.Sprintf("data not found for %s: %v", e.dataType, e.err)
}

func (e DataNotFound) Unwrap() error {
	return e.err
}

func IsDataNotFound(err error) bool {
	var errDataNotFound DataNotFound
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

func IsInternalError(err error) bool {
	var errInternalError InternalError
	return errors.As(err, &errInternalError)
}

// OutOfRangeError indicates that the request was for data that is outside of the available range.
// This is a more specific version of DataNotFound, where the data is known to eventually exist, but
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

func IsOutOfRangeError(err error) bool {
	var errOutOfRangeError OutOfRangeError
	return errors.As(err, &errOutOfRangeError)
}

// FailedPrecondition indicates that a precondition for the operation was not met
// This is a more specific version of InvalidRequest, where the request is valid, but the system
// is not currently in a state to fulfill the request (but may be in the future).
type FailedPrecondition struct {
	err error
}

func NewFailedPrecondition(err error) FailedPrecondition {
	return FailedPrecondition{err: err}
}

func (e FailedPrecondition) Error() string {
	return fmt.Sprintf("precondition failed: %v", e.err)
}

func (e FailedPrecondition) Unwrap() error {
	return e.err
}

func IsPreconditionFailed(err error) bool {
	var errPreconditionFailed FailedPrecondition
	return errors.As(err, &errPreconditionFailed)
}
