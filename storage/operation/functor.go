// Package operation provides functional programming utilities for database operations
// with lock context support. It defines functors that can be composed to create
// complex database operations while ensuring proper lock acquisition and error handling.
package operation

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/merr"
)

// Functor represents a database operation that requires a lock context proof
// and a batch writer. It encapsulates the pattern of acquiring locks before
// performing database operations and ensures proper error handling.
type Functor func(lockctx.Proof, storage.ReaderBatchWriter) error

// BindFunctors composes multiple functors into a single functor that executes
// them sequentially. This enables functional composition of database operations.
// If any bound functor returns an error, that error is passed through unchanged.
// This utility is error-agnostic: callers are responsible for checking and documenting expected errors at the higher level.
func BindFunctors(functors ...Functor) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		for _, fn := range functors {
			err := fn(lctx, rw)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// CheckHoldsLockFunctor creates a functor that validates the lock context holds the specified lock.
// This is used as a guard to ensure operations are only performed when the required lock is held.
// Returns an exception if the lock is not held, otherwise returns nil.
func CheckHoldsLockFunctor(lockID string) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		if !lctx.HoldsLock(lockID) {
			return fmt.Errorf("missing required lock: %s", lockID)
		}
		return nil
	}
}

// WrapError creates a functor that wraps any error returned by the provided functor
// with additional context. This is useful for providing more descriptive error messages
// when composing complex operations.
func WrapError(wrapMsg string, fn Functor) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		err := fn(lctx, rw)
		if err != nil {
			return fmt.Errorf("%s: %w", wrapMsg, err)
		}
		return nil
	}
}

// UpsertFunctor returns a functor that overwrites a key-value pair in the storage.
// The value is serialized using msgpack encoding. If the key already exists,
// the value will be overwritten without any checks.
//
// This is typically used for operations where we want to update existing data
// or where we don't care about potential conflicts.
func UpsertFunctor(key []byte, val interface{}) Functor {
	value, err := msgpack.Marshal(val)
	if err != nil {
		return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		err = rw.Writer().Set(key, value)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

// UpsertMulFunctor returns a functor that overwrites multiple key-value pairs in the storage.
// The values are serialized using msgpack encoding. If any of the keys already exist,
// the values will be overwritten without any checks.
//
// This is the batch version of UpsertFunctor, useful for operations where we want to
// update multiple entries atomically or where we don't care about potential conflicts.
// The keys and vals slices must have the same length.
func UpsertMulFunctor(keys [][]byte, vals []any) Functor {
	if len(keys) != len(vals) {
		return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
			return irrecoverable.NewExceptionf("keys and vals length mismatch: %d vs %d", len(keys), len(vals))
		}
	}

	values := make([][]byte, len(vals))
	for i, val := range vals {
		value, err := msgpack.Marshal(val)
		if err != nil {
			return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
				return irrecoverable.NewExceptionf("failed to encode value at index %d: %w", i, err)
			}
		}
		values[i] = value
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		for i, key := range keys {
			err := rw.Writer().Set(key, values[i])
			if err != nil {
				return irrecoverable.NewExceptionf("failed to store data at index %d: %w", i, err)
			}
		}

		return nil
	}
}

// InsertingWithExistenceCheck returns a functor that inserts a key-value pair
// only if the key does not already exist. If the key exists, it returns
// storage.ErrAlreadyExists error.
//
// This is used for operations where we want to ensure uniqueness and prevent
// accidental overwrites of existing data.
func InsertingWithExistenceCheck(key []byte, val interface{}) Functor {
	value, err := msgpack.Marshal(val)
	if err != nil {
		return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		exist, err := KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to check for existing key: %w", err)
		}

		if exist {
			return fmt.Errorf("attempting to insert existing key: %x: %w", key, storage.ErrAlreadyExists)
		}

		err = rw.Writer().Set(key, value)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

// InsertingWithMismatchCheck returns a functor that inserts a key-value pair
// with conflict detection. If the key already exists, it compares the existing
// value with the new value. If they differ, it returns storage.ErrDataMismatch.
// If they are the same, the operation succeeds without modification.
//
// This is used for operations where we want to ensure data consistency and
// detect potential race conditions or conflicting updates.
func InsertingWithMismatchCheck(key []byte, val interface{}) Functor {
	value, err := msgpack.Marshal(val)
	if err != nil {
		return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) (errToReturn error) {
		existing, closer, err := rw.GlobalReader().Get(key)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return irrecoverable.NewExceptionf("could not load existing data when inserting new data: %w", err)
			}

			// no existing data stored under this key, proceed with insert
			err = rw.Writer().Set(key, value)
			if err != nil {
				return irrecoverable.NewExceptionf("failed to store data: %w", err)
			}

			return nil
		}

		defer func() {
			errToReturn = merr.CloseAndMergeError(closer, errToReturn)
		}()

		if !bytes.Equal(existing, value) {
			return fmt.Errorf("attempting to insert existing key with different value: %x: %w", key, storage.ErrDataMismatch)
		}

		return nil
	}
}

// InsertingMulWithMismatchCheck returns a functor that inserts multiple key-value pairs
// with conflict detection. For each key, if it already exists, it compares the existing
// value with the new value. If they differ, it returns storage.ErrDataMismatch.
// If they are the same, the operation succeeds without modification.
//
// This is the batch version of InsertingWithMismatchCheck, useful for operations where
// we want to ensure data consistency and detect potential race conditions or conflicting
// updates across multiple entries atomically. The keys and vals slices must have the same length.
func InsertingMulWithMismatchCheck(keys [][]byte, vals []any) Functor {
	if len(keys) != len(vals) {
		return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
			return irrecoverable.NewExceptionf("keys and vals length mismatch: %d vs %d", len(keys), len(vals))
		}
	}

	values := make([][]byte, len(vals))
	for i, val := range vals {
		value, err := msgpack.Marshal(val)
		if err != nil {
			return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
				return irrecoverable.NewExceptionf("failed to encode value at index %d: %w", i, err)
			}
		}
		values[i] = value
	}

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) (errToReturn error) {
		for i, key := range keys {
			existing, closer, err := rw.GlobalReader().Get(key)
			if err != nil {
				if !errors.Is(err, storage.ErrNotFound) {
					return irrecoverable.NewExceptionf("could not load existing data when inserting new data at index %d: %x: %w",
						i, key, err)
				}

				// no existing data stored under this key, proceed with insert
				err = rw.Writer().Set(key, values[i])
				if err != nil {
					return irrecoverable.NewExceptionf("failed to store data at index %d: %x: %w", i, key, err)
				}

				continue
			}

			errToReturn = merr.CloseAndMergeError(closer, errToReturn)
			if errToReturn != nil {
				return errToReturn
			}

			if !bytes.Equal(existing, values[i]) {
				return fmt.Errorf("attempting to insert existing key with different value at index %d: %x: %w",
					i, key, storage.ErrDataMismatch)
			}
		}

		return nil
	}

}

// OnCommitSucceedFunctor returns a functor that registers a callback to be executed
// when the database transaction commits successfully. The callback is executed after
// the transaction is committed but before the batch writer is closed.
//
// This is useful for operations that need to perform additional actions (like notifications
// or cache updates) only after the database changes are permanently stored.
func OnCommitSucceedFunctor(callback func()) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		storage.OnCommitSucceed(rw, callback)
		return nil
	}
}

// NoOpFunctor returns a functor that performs no operation and always succeeds.
// This is useful as a placeholder in functor compositions or when a conditional
// operation needs to be skipped without affecting the overall composition.
func NoOpFunctor() Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		return nil
	}
}
