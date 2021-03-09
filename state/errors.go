package state

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidExtensionError is an error for invalid extension of the state
type InvalidExtensionError struct {
	err error
}

func NewInvalidExtensionError(msg string) error {
	return NewInvalidExtensionErrorf(msg)
}

func NewInvalidExtensionErrorf(msg string, args ...interface{}) error {
	return InvalidExtensionError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidExtensionError) Unwrap() error {
	return e.err
}

func (e InvalidExtensionError) Error() string {
	return e.err.Error()
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
	err error
}

func NewOutdatedExtensionError(msg string) error {
	return NewOutdatedExtensionErrorf(msg)
}

func NewOutdatedExtensionErrorf(msg string, args ...interface{}) error {
	return OutdatedExtensionError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e OutdatedExtensionError) Unwrap() error {
	return e.err
}

func (e OutdatedExtensionError) Error() string {
	return e.err.Error()
}

func IsOutdatedExtensionError(err error) bool {
	return errors.As(err, &OutdatedExtensionError{})
}

// NoValidChildBlockError is a sentinel error when the case where a certain block has
// no valid child.
type NoValidChildBlockError struct {
	err error
}

func NewNoValidChildBlockError(msg string) error {
	return NoValidChildBlockError{
		err: fmt.Errorf(msg),
	}
}

func NewNoValidChildBlockErrorf(msg string, args ...interface{}) error {
	return NewNoValidChildBlockError(fmt.Sprintf(msg, args...))
}

func (e NoValidChildBlockError) Unwrap() error {
	return e.err
}

func (e NoValidChildBlockError) Error() string {
	return e.err.Error()
}

func IsNoValidChildBlockError(err error) bool {
	return errors.As(err, &NoValidChildBlockError{})
}

// UnvalidatedBlockQueryError is an error returned when we query a block by ID
// which has not been marked valid by HotStuff. This means that we aren't yet
// sure whether the block contains a valid quorum certificate. As a rule, the
// protocol state returns an error for these blocks rather than returning a
// state which may be invalid.
type UnvalidatedBlockQueryError struct {
	BlockID flow.Identifier
}

func (e UnvalidatedBlockQueryError) Error() string {
	return fmt.Sprintf("invalid query for unvalidated block (id=%x)", e.BlockID)
}

func NewUnvalidatedBlockQueryError(blockID flow.Identifier) error {
	return UnvalidatedBlockQueryError{BlockID: blockID}
}

func IsUnvalidatedBlockQueryError(err error) bool {
	return errors.As(err, &UnvalidatedBlockQueryError{})
}
