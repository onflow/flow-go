package operation

import (
	"bytes"
	"fmt"

	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// Upsert will encode the given entity using msgpack and will insert the resulting
// binary data under the provided key.
// If the key already exists, the value will be overwritten.
// Error returns:
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func Upsert(key []byte, val interface{}) func(storage.Writer) error {
	return func(w storage.Writer) error {
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
}

// Remove removes the entity with the given key, if it exists. If it doesn't
// exist, this is a no-op.
// Error returns:
// * generic error in case of unexpected database error
func Remove(key []byte) func(storage.Writer) error {
	return func(w storage.Writer) error {
		err := w.Delete(key)
		if err != nil {
			return irrecoverable.NewExceptionf("could not delete item: %w", err)
		}
		return nil
	}
}

// RemoveByPrefix removes all keys with the given prefix
// Error returns:
// * generic error in case of unexpected database error
func RemoveByPrefix(reader storage.Reader, key []byte) func(storage.Writer) error {
	return RemoveByRange(reader, key, key)
}

// RemoveByRange removes all keys with a prefix that falls within the range [start, end], both inclusive.
// It returns error if endPrefix < startPrefix
// no other errors are expected during normal operation
func RemoveByRange(reader storage.Reader, startPrefix []byte, endPrefix []byte) func(storage.Writer) error {
	return func(w storage.Writer) error {
		if bytes.Compare(startPrefix, endPrefix) > 0 {
			return fmt.Errorf("startPrefix key must be less than or equal to endPrefix key")
		}
		err := w.DeleteByRange(reader, startPrefix, endPrefix)
		if err != nil {
			return irrecoverable.NewExceptionf("could not delete item: %w", err)
		}
		return nil
	}
}
