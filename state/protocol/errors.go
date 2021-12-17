package protocol

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
)

var (
	// ErrNoPreviousEpoch is a sentinel error returned when a previous epoch is
	// queried from a snapshot within the first epoch after the root block.
	ErrNoPreviousEpoch = fmt.Errorf("no previous epoch exists")

	// ErrNextEpochNotSetup is a sentinel error returned when the next epoch
	// has not been set up yet.
	ErrNextEpochNotSetup = fmt.Errorf("next epoch has not yet been set up")

	// ErrEpochNotCommitted is a sentinel error returned when the epoch has
	// not been committed and information is queried that is only accessible
	// in the EpochCommitted phase.
	ErrEpochNotCommitted = fmt.Errorf("queried info from EpochCommit event before it was emitted")
)

type IdentityNotFoundError struct {
	NodeID flow.Identifier
}

func (e IdentityNotFoundError) Error() string {
	return fmt.Sprintf("identity not found (%x)", e.NodeID)
}

func IsIdentityNotFound(err error) bool {
	var errIdentityNotFound IdentityNotFoundError
	return errors.As(err, &errIdentityNotFound)
}

type InvalidBlockTimestampError struct {
	err error
}

func (e InvalidBlockTimestampError) Unwrap() error {
	return e.err
}

func (e InvalidBlockTimestampError) Error() string {
	return e.err.Error()
}

func IsInvalidBlockTimestampError(err error) bool {
	var errInvalidTimestampError InvalidBlockTimestampError
	return errors.As(err, &errInvalidTimestampError)
}

func NewInvalidBlockTimestamp(msg string, args ...interface{}) error {
	return InvalidBlockTimestampError{
		err: fmt.Errorf(msg, args...),
	}
}

// InvalidServiceEventError indicates an invalid service event was processed.
type InvalidServiceEventError struct {
	err error
}

func (e InvalidServiceEventError) Unwrap() error {
	return e.err
}

func (e InvalidServiceEventError) Error() string {
	return e.err.Error()
}

func IsInvalidServiceEventError(err error) bool {
	var errInvalidServiceEventError InvalidServiceEventError
	return errors.As(err, &errInvalidServiceEventError)
}

// NewInvalidServiceEventError returns an invalid service event error. Since all invalid
// service events indicate an invalid extension, the service event error is wrapped in
// the invalid extension error at construction.
func NewInvalidServiceEventError(msg string, args ...interface{}) error {
	return state.NewInvalidExtensionErrorf(
		"cannot extend state with invalid service event: %w",
		InvalidServiceEventError{
			err: fmt.Errorf(msg, args...),
		},
	)
}
