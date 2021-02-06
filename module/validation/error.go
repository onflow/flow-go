package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type MissingPreviousResultError struct {
	err error
}

func NewMissingPreviousResultError(previousResultID flow.Identifier) error {
	return MissingPreviousResultError{
		err: fmt.Errorf("receipt's previous result is missing: %v", previousResultID),
	}
}

func (e MissingPreviousResultError) Unwrap() error {
	return e.err
}

func (e MissingPreviousResultError) Error() string {
	return e.err.Error()
}

// IsMissingPreviousResultError returns whether the given error is an MissingPreviousResultError error
func IsMissingPreviousResultError(err error) bool {
	var errMissingPreviousResultError MissingPreviousResultError
	return errors.As(err, &errMissingPreviousResultError)
}
