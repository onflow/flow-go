package state

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownSnapshotReference indicates that the reference point for a queried
	// snapshot cannot be resolved. The reference point is either a height above the
	// finalized boundary, or a block ID that does not exist in the state.
	ErrUnknownSnapshotReference = errors.New("reference block of the snapshot is not resolvable")
)

// InvalidExtensionError is an error for invalid extension of the state. An invalid
// extension is distinct from outdated or unverifiable extensions, in that it indicates
// a malicious input.
type InvalidExtensionError struct {
	error
}

func NewInvalidExtensionErrorf(msg string, args ...any) error {
	return InvalidExtensionError{
		error: fmt.Errorf(msg, args...),
	}
}

func (e InvalidExtensionError) Unwrap() error {
	return e.error
}

// IsInvalidExtensionError returns whether the given error is an InvalidExtensionError error
func IsInvalidExtensionError(err error) bool {
	return errors.As(err, &InvalidExtensionError{})
}

// OutdatedExtensionError is an error for the extension of the state being outdated.
// Being outdated doesn't mean it's invalid or not.
// Knowing whether an outdated extension is an invalid extension or not would
// take more state queries.
type OutdatedExtensionError struct {
	error
}

func NewOutdatedExtensionErrorf(msg string, args ...any) error {
	return OutdatedExtensionError{
		error: fmt.Errorf(msg, args...),
	}
}

func (e OutdatedExtensionError) Unwrap() error {
	return e.error
}

func IsOutdatedExtensionError(err error) bool {
	return errors.As(err, &OutdatedExtensionError{})
}

// UnverifiableExtensionError represents a state extension (block) which cannot be
// verified at the moment. For example, it does not connect to the finalized state,
// or an entity referenced within the payload is unknown.
// Unlike InvalidExtensionError, this error is only used when the failure CANNOT be
// attributed to a malicious input, therefore this error can be treated as a benign failure.
type UnverifiableExtensionError struct {
	error
}

func NewUnverifiableExtensionError(msg string, args ...any) error {
	return UnverifiableExtensionError{
		error: fmt.Errorf(msg, args...),
	}
}

func (e UnverifiableExtensionError) Unwrap() error {
	return e.error
}

func IsUnverifiableExtensionError(err error) bool {
	var errUnverifiableExtensionError UnverifiableExtensionError
	return errors.As(err, &errUnverifiableExtensionError)
}
