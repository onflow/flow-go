package common

import (
	"context"
	"errors"
	"net/http"

	"github.com/onflow/flow-go/access"
)

// StatusError provides custom error with http status.
type StatusError interface {
	error                // this is the actual error that occured
	Status() int         // the HTTP status code to return
	UserMessage() string // the error message to return to the client
}

// NewRestError creates an error returned to user with provided status
// user displayed message and internal error
func NewRestError(status int, msg string, err error) *Error {
	return &Error{
		status:      status,
		userMessage: msg,
		err:         err,
	}
}

// NewNotFoundError creates a new not found rest error.
func NewNotFoundError(msg string, err error) *Error {
	return &Error{
		status:      http.StatusNotFound,
		userMessage: msg,
		err:         err,
	}
}

// NewBadRequestError creates a new bad request rest error.
func NewBadRequestError(err error) *Error {
	return &Error{
		status:      http.StatusBadRequest,
		userMessage: err.Error(),
		err:         err,
	}
}

// Error is implementation of status error.
type Error struct {
	status      int
	userMessage string
	err         error
}

func (e *Error) UserMessage() string {
	return e.userMessage
}

// Status returns error http status code.
func (e *Error) Status() int {
	return e.status
}

func (e *Error) Error() string {
	return e.err.Error()
}

// ErrorToStatusError converts an Access API error into a grpc status error. The input may either
// be a status.Error already, or an access sentinel error.
func ErrorToStatusError(err error) StatusError {
	if err == nil {
		return nil
	}

	var converted StatusError
	if errors.As(err, &converted) {
		return converted
	}

	switch {
	case access.IsInvalidRequest(err):
		return NewBadRequestError(err)
	case access.IsDataNotFound(err):
		return NewNotFoundError(err.Error(), err)
	case access.IsPreconditionFailed(err):
		return NewRestError(http.StatusPreconditionFailed, err.Error(), err)
	case access.IsOutOfRangeError(err):
		return NewNotFoundError(err.Error(), err)
	case access.IsInternalError(err):
		return NewRestError(http.StatusInternalServerError, err.Error(), err)
	case errors.Is(err, context.Canceled):
		return NewRestError(http.StatusRequestTimeout, "Request canceled", err)
	case errors.Is(err, context.DeadlineExceeded):
		return NewRestError(http.StatusRequestTimeout, "Request deadline exceeded", err)
	default:
		return NewRestError(http.StatusInternalServerError, err.Error(), err)
	}
}
