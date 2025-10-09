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

type Functor func(lockctx.Proof, storage.ReaderBatchWriter) error

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

func HoldingLock(lockID string) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		if !lctx.HoldsLock(lockID) {
			return fmt.Errorf("missing required lock: %s", lockID)
		}
		return nil
	}
}

func WrapError(wrapMsg string, fn Functor) Functor {
	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		err := fn(lctx, rw)
		if err != nil {
			return fmt.Errorf("%s: %w", wrapMsg, err)
		}
		return nil
	}
}

// Overwriting returns a functor, whose execution will append the given key-value-pair to the provided
// storage writer (typically a pending batch of database writes).
func Overwriting(key []byte, val interface{}) Functor {
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
