package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type UnverifiableError struct {
	err error
}

func NewUnverifiableError(previousResultID flow.Identifier) error {
	return UnverifiableError{
		err: fmt.Errorf("receipt's previous result is missing: %v", previousResultID),
	}
}

func (e UnverifiableError) Unwrap() error {
	return e.err
}

func (e UnverifiableError) Error() string {
	return e.err.Error()
}

// IsUnverifiableError returns whether the given error is an UnverifiableError error
func IsUnverifiableError(err error) bool {
	var errUnverifiableError UnverifiableError
	return errors.As(err, &errUnverifiableError)
}
