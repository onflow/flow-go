package dkg

import (
	"errors"
	"fmt"
)

// InvalidStateTransitionError happens when an invalid DKG state transition is
// attempted.
type InvalidStateTransitionError struct {
	From State
	To   State
}

// NewInvalidStateTransitionError creates a new InvalidStateTransitionError
// between the specified states.
func NewInvalidStateTransitionError(from State, to State) InvalidStateTransitionError {
	return InvalidStateTransitionError{
		From: from,
		To:   to,
	}
}

func (e InvalidStateTransitionError) Error() string {
	return fmt.Sprintf("Invalid DKG state transition from %s to %s", e.From, e.To)
}

func IsInvalidStateTransitionError(err error) bool {
	var errInvalidStateTransition InvalidStateTransitionError
	return errors.As(err, &errInvalidStateTransition)
}
