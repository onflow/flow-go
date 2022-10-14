package admin

import (
	"errors"
	"fmt"
)

// ErrValidatorReqDataFormat is returned when an admin request is not in the expected format.
// For example, if the input is a JSON number, but the command expects a map, type conversion
// of the input will fail.
var ErrValidatorReqDataFormat = NewInvalidAdminReqErrorf("invalid request format")

// InvalidAdminReqError indicates that an admin request has failed validation, and
// the request will not be processed. All Validator functions must return this error
// if the
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
	return NewInvalidAdminReqErrorf("invalid value for '%s': %s. Got: %v", field, msg, actualVal)
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
