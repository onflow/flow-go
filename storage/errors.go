package storage

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var (
	// ErrNotFound is returned when a retrieved key does not exist in the database.
	// Note: there is another not found error: badger.ErrKeyNotFound. The difference between
	// badger.ErrKeyNotFound and storage.ErrNotFound is that:
	// badger.ErrKeyNotFound is the error returned by the badger API.
	// Modules in storage/badger and storage/badger/operation package both
	// return storage.ErrNotFound for not found error
	ErrNotFound = errors.New("key not found")

	// ErrAlreadyExists is returned when an insert attempts to set the value
	// for a key that already exists. Inserts may only occur once per key,
	// updates may overwrite an existing key without returning an error.
	ErrAlreadyExists = errors.New("key already exists")

	// ErrDataMismatch is returned when a repeatable insert operation attempts
	// to insert a different value for the same key.
	ErrDataMismatch = errors.New("data for key is different")

	// ErrHeightNotIndexed is returned when data that is indexed sequentially is queried by a given block height
	// and that data is unavailable.
	ErrHeightNotIndexed = errors.New("data for block height not available")

	// ErrNotBootstrapped is returned when the database has not been bootstrapped.
	ErrNotBootstrapped = errors.New("pebble database not bootstrapped")

	// ErrInvalidQuery is returned when parameters passed to a read query are invalid (e.g., startHeight > endHeight).
	ErrInvalidQuery = errors.New("invalid query")
)

// InvalidDKGStateTransitionError is a sentinel error that is returned in case an invalid state transition is attempted.
type InvalidDKGStateTransitionError struct {
	err  error
	From flow.DKGState
	To   flow.DKGState
}

func (e InvalidDKGStateTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition from %s to %s: %s", e.From.String(), e.To.String(), e.err.Error())
}

func IsInvalidDKGStateTransitionError(err error) bool {
	var e InvalidDKGStateTransitionError
	return errors.As(err, &e)
}

// NewInvalidDKGStateTransitionErrorf constructs a new InvalidDKGStateTransitionError error with a formatted message.
func NewInvalidDKGStateTransitionErrorf(from, to flow.DKGState, msg string, args ...any) error {
	return InvalidDKGStateTransitionError{
		From: from,
		To:   to,
		err:  fmt.Errorf(msg, args...),
	}
}
