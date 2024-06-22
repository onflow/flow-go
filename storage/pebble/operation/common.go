package operation

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type PebbleReaderWriter interface {
	pebble.Reader
	pebble.Writer
}

// insertNew requires the key does not exist
// Error returns: storage.ErrAlreadyExists
func insertNew(key []byte, val interface{}) func(PebbleReaderWriter) error {
	return func(rw PebbleReaderWriter) error {
		_, closer, err := rw.Get(key)
		if err == nil {
			defer closer.Close()
			return storage.ErrAlreadyExists
		}

		if !errors.Is(err, pebble.ErrNotFound) {
			return irrecoverable.NewExceptionf("could not check key: %w", err)
		}

		value, err := msgpack.Marshal(val)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}

		err = rw.Set(key, value, pebble.Sync)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

func insert(key []byte, val interface{}) func(pebble.Writer) error {
	return func(w pebble.Writer) error {
		value, err := msgpack.Marshal(val)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}

		err = w.Set(key, value, pebble.Sync)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

func retrieve(key []byte, sc interface{}) func(r pebble.Reader) error {
	return func(r pebble.Reader) error {
		val, closer, err := r.Get(key)
		if err != nil {
			return convertNotFoundError(err)
		}
		defer closer.Close()

		err = msgpack.Unmarshal(val, sc)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to decode value: %w", err)
		}
		return nil
	}
}

func exists(key []byte, keyExists *bool) func(r pebble.Reader) error {
	return func(r pebble.Reader) error {
		_, closer, err := r.Get(key)
		if err != nil {
			if errors.Is(err, pebble.ErrNotFound) {
				*keyExists = false
				return nil
			}

			// exception while checking for the key
			return irrecoverable.NewExceptionf("could not load data: %w", err)
		}
		*keyExists = true
		defer closer.Close()
		return nil
	}
}

// checkFunc is called during key iteration through the badger DB in order to
// check whether we should process the given key-value pair. It can be used to
// avoid loading the value if its not of interest, as well as storing the key
// for the current iteration step.
type checkFunc func(key []byte) bool

// createFunc returns a pointer to an initialized entity that we can potentially
// decode the next value into during a badger DB iteration.
type createFunc func() interface{}

// handleFunc is a function that starts the processing of the current key-value
// pair during a badger iteration. It should be called after the key was checked
// and the entity was decoded.
// No errors are expected during normal operation. Any errors will halt the iteration.
type handleFunc func() error

// iterationFunc is a function provided to our low-level iteration function that
// allows us to pass badger efficiencies across badger boundaries. By calling it
// for each iteration step, we can inject a function to check the key, a
// function to create the decode target and a function to process the current
// key-value pair. This a consumer of the API to decode when to skip the loading
// of values, the initialization of entities and the processing.
type iterationFunc func() (checkFunc, createFunc, handleFunc)

// update will encode the given entity with MsgPack and update the binary data
// under the given key in the badger DB. The key must already exist.
// Error returns:
//   - storage.ErrNotFound if the key does not already exist in the database.
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func update(key []byte, entity interface{}) func(PebbleReaderWriter) error {
	return func(w PebbleReaderWriter) error {

		// retrieve the item from the key-value store
		_, closer, err := w.Get(key)
		if errors.Is(err, pebble.ErrNotFound) {
			return storage.ErrNotFound
		}

		if err != nil {
			return irrecoverable.NewExceptionf("could not check key: %w", err)
		}
		err = closer.Close()
		if err != nil {
			return err
		}

		// serialize the entity data
		val, err := msgpack.Marshal(entity)
		if err != nil {
			return irrecoverable.NewExceptionf("could not encode entity: %w", err)
		}

		// persist the entity data into the DB
		err = w.Set(key, val, nil)
		if err != nil {
			return irrecoverable.NewExceptionf("could not replace data: %w", err)
		}

		return nil
	}
}

// remove removes the entity with the given key, if it exists. If it doesn't
// exist, this is a no-op.
// Error returns:
// * generic error in case of unexpected database error
func remove(key []byte) func(pebble.Writer) error {
	return func(w pebble.Writer) error {
		err := w.Delete(key, nil)
		if err != nil {
			return irrecoverable.NewExceptionf("could not delete item: %w", err)
		}
		return nil
	}
}

// traverse iterates over a range of keys defined by a prefix.
//
// The prefix must be shared by all keys in the iteration.
//
// On each iteration, it will call the iteration function to initialize
// functions specific to processing the given key-value pair.
func traverse(prefix []byte, iteration iterationFunc) func(pebble.Reader) error {
	return func(r pebble.Reader) error {
		if len(prefix) == 0 {
			return fmt.Errorf("prefix must not be empty")
		}

		// opts := badger.DefaultIteratorOptions
		// NOTE: this is an optimization only, it does not enforce that all
		// results in the iteration have this prefix.
		// opts.Prefix = prefix

		// TODO: add prefix to new iter, use `useL6Filter`
		it, err := r.NewIter(nil)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		// this is where we actually enforce that all results have the prefix
		for it.SeekGE(prefix); it.Valid(); it.Next() {

			// initialize processing functions for iteration
			check, create, handle := iteration()

			// check if we should process the item at all
			key := it.Key()

			ok := check(key)
			if !ok {
				continue
			}

			binaryValue, err := it.ValueAndErr()
			if err != nil {
				return fmt.Errorf("failed to get value: %w", err)
			}

			// preventing caller from modifying the iterator's value slices
			valueCopy := make([]byte, len(binaryValue))
			copy(valueCopy, binaryValue)

			entity := create()
			err = msgpack.Unmarshal(valueCopy, entity)
			if err != nil {
				return irrecoverable.NewExceptionf("could not decode entity: %w", err)
			}
			// process the entity
			err = handle()
			if err != nil {
				return fmt.Errorf("could not handle entity: %w", err)
			}

		}

		return nil
	}
}

// removeByPrefix removes all the entities if the prefix of the key matches the given prefix.
// if no key matches, this is a no-op
// No errors are expected during normal operation.
func removeByPrefix(prefix []byte) func(pebble.Writer) error {
	return func(tx pebble.Writer) error {
		start, end := getStartEndKeys(prefix)
		return tx.DeleteRange(start, end, nil)
	}
}

// getStartEndKeys calculates the start and end keys for a given prefix.
func getStartEndKeys(prefix []byte) (start, end []byte) {
	start = prefix

	// End key is the prefix with the last byte incremented by 1
	end = append([]byte{}, prefix...)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i]++
			break
		}
		end[i] = 0
		if i == 0 {
			end = append(end, 0)
		}
	}

	return start, end
}

func convertNotFoundError(err error) error {
	if errors.Is(err, pebble.ErrNotFound) {
		return storage.ErrNotFound
	}
	return err
}

// TODO: TOFIX
func findHighestAtOrBelow(
	prefix []byte,
	height uint64,
	entity interface{},
) func(pebble.Reader) error {
	return func(r pebble.Reader) error {
		if len(prefix) == 0 {
			return fmt.Errorf("prefix must not be empty")
		}

		// opts := badger.DefaultIteratorOptions
		// opts.Prefix = prefix
		// opts.Reverse = true

		it, err := r.NewIter(nil)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}

		it.SeekGE(append(prefix, b(height)...))

		if !it.Valid() {
			return storage.ErrNotFound
		}

		val, err := it.ValueAndErr()
		if err != nil {
			return fmt.Errorf("could not get value: %w", err)
		}

		err = msgpack.Unmarshal(val, entity)
		if err != nil {
			return fmt.Errorf("could not decode entity: %w", err)
		}

		return nil
	}
}

func BatchUpdate(db *pebble.DB, fn func(tx PebbleReaderWriter) error) error {
	batch := db.NewBatch()
	err := fn(batch)
	if err != nil {
		return err
	}
	return batch.Commit(nil)
}
