package rest

import "net/http"

// StatusError provides custom error with http status.
type StatusError interface {
	error
	Status() int
}

// RestError is implementation of status error.
type RestError struct {
	status      int
	userMessage string
	err         error
}

// NewRestError creates an error returned to user with provided status
// user displayed message and internal error
func NewRestError(status int, msg string, err error) *RestError {
	return &RestError{
		status:      status,
		userMessage: msg,
		err:         err,
	}
}

// NewNotFoundError creates a new not found rest error.
func NewNotFoundError(msg string, err error) *RestError {
	return &RestError{
		status:      http.StatusNotFound,
		userMessage: msg,
		err:         err,
	}
}

// NewBadRequestError creates a new bad request rest error.
func NewBadRequestError(msg string, err error) *RestError {
	return &RestError{
		status:      http.StatusBadRequest,
		userMessage: msg,
		err:         err,
	}
}

// Status returns error http status code.
func (e *RestError) Status() int {
	return e.status
}

func (e *RestError) Error() string {
	return e.err.Error()
}
