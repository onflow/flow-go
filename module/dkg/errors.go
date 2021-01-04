package dkg

import (
	"errors"
	"fmt"
)

type InvalidStateTransitionError struct {
	From State
	To   State
}

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
