package operation

import (
	"bytes"
	"fmt"

	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// UpsertByKey will encode the given entity using msgpack and will insert the resulting
// binary data under the provided key.
// If the key already exists, the value will be overwritten.
// Error returns:
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func UpsertByKey(w storage.Writer, key []byte, val any) error {
	value, err := msgpack.Marshal(val)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to encode value: %w", err)
	}

	err = w.Set(key, value)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to store data: %w", err)
	}

	return nil
}

// Upserting returns a functor, whose execution will append the given key-value-pair to the provided
// storage writer (typically a pending batch of database writes).
func Upserting(key []byte, val any) func(storage.Writer) error {
	value, err := msgpack.Marshal(val)
	return func(w storage.Writer) error {
		if err != nil {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}

		err = w.Set(key, value)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

// RemoveByKey removes the entity with the given key, if it exists. If it doesn't
// exist, this is a no-op.
// Error returns:
// * generic error in case of unexpected database error
func RemoveByKey(w storage.Writer, key []byte) error {
	err := w.Delete(key)
	if err != nil {
		return irrecoverable.NewExceptionf("could not delete item: %w", err)
	}
	return nil
}

// RemoveByKeyPrefix removes all keys with the given prefix
// Error returns:
// * generic error in case of unexpected database error
func RemoveByKeyPrefix(reader storage.Reader, w storage.Writer, prefix []byte) error {
	return RemoveByKeyRange(reader, w, prefix, prefix)
}

// RemoveByKeyRange removes all keys with a prefix that falls within the range [start, end], both inclusive.
// It returns error if endPrefix < startPrefix
// no other errors are expected during normal operation
func RemoveByKeyRange(reader storage.Reader, w storage.Writer, startPrefix []byte, endPrefix []byte) error {
	if bytes.Compare(startPrefix, endPrefix) > 0 {
		return fmt.Errorf("startPrefix key must be less than or equal to endPrefix key")
	}
	err := w.DeleteByRange(reader, startPrefix, endPrefix)
	if err != nil {
		return irrecoverable.NewExceptionf("could not delete item: %w", err)
	}
	return nil
}
