package signature

import (
	"errors"
)

var (
	ErrInvalidFormat      = errors.New("invalid signature format")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")

	ErrInvalidInputs    = errors.New("invalid inputs")
	ErrDuplicatedSigner = errors.New("duplicated signer")
)

// ErrInvalidInputs is returned when an API receives invalid inputs.
type ErrInvalidInputs struct {
	message string
}

// newInvalidInputsError constructs a new InvalidInputsError
func newErrInvalidInputs(msg string, args ...interface{}) error {
	return &ErrInvalidInputs{message: fmt.Sprintf(msg, args...)}
}

func (e ErrInvalidInputs) Error() string {
	return e.message
}
