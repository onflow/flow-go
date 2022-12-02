package state

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
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

func NewInvalidExtensionError(msg string) error {
	return NewInvalidExtensionErrorf(msg)
}

func NewInvalidExtensionErrorf(msg string, args ...interface{}) error {
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

func NewOutdatedExtensionError(msg string) error {
	return NewOutdatedExtensionErrorf(msg)
}

func NewOutdatedExtensionErrorf(msg string, args ...interface{}) error {
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

func NewUnverifiableExtensionError(msg string, args ...interface{}) error {
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

// NoChildBlockError is returned where a certain block has no valid child.
// Since all blocks are validated before being inserted to the state, this is
// equivalent to have no stored child.
type NoChildBlockError struct {
	error
}

func NewNoChildBlockError(msg string) error {
	return NoChildBlockError{
		error: fmt.Errorf(msg),
	}
}

func NewNoChildBlockErrorf(msg string, args ...interface{}) error {
	return NewNoChildBlockError(fmt.Sprintf(msg, args...))
}

func (e NoChildBlockError) Unwrap() error {
	return e.error
}


func IsNoChildBlockError(err error) bool {
	return errors.As(err, &NoChildBlockError{})
}

// UnknownBlockError is a sentinel error indicating that a certain block
// has not been ingested yet.
type UnknownBlockError struct {
	blockID flow.Identifier
	error
}

// WrapAsUnknownBlockError wraps a given error as UnknownBlockError
func WrapAsUnknownBlockError(blockID flow.Identifier, err error) error {
	return UnknownBlockError{
		blockID: blockID,
		error:   fmt.Errorf("block %v has not been processed yet: %w", blockID, err),
	}
}

func NewUnknownBlockError(blockID flow.Identifier) error {
	return UnknownBlockError{
		blockID: blockID,
		error:   fmt.Errorf("block %v has not been processed yet", blockID),
	}
}

func (e UnknownBlockError) Unwrap() error { return e.error }

func IsUnknownBlockError(err error) bool {
	var e UnknownBlockError
	return errors.As(err, &e)
}
