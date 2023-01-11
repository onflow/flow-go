package admin

import (
	"errors"
	"fmt"
)

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
