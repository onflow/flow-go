package operation

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// CheckFunc is a function that checks if the value should be read and decoded.
// return (true, nil) to read the value and pass it to the CreateFunc and HandleFunc for decoding
// return (false, nil) to skip reading the value
// return (false, err) if running into any error, the iteration should be stopped.
// when making a CheckFunc to be used in the IterationFunc to iterate over the keys, a sentinel error
// can be defined and checked to stop the iteration early, such as finding the first key that match
// certain condition.
type CheckFunc func(key []byte) (bool, error)

// CreateFunc returns a pointer to an initialized entity that we can potentially
// decode the next value into during a badger DB iteration.
type CreateFunc func() interface{}

// HandleFunc is a function that starts the processing of the current key-value
// pair during a badger iteration. It should be called after the key was checked
// and the entity was decoded.
// No errors are expected during normal operation. Any errors will halt the iteration.
type HandleFunc func() error
type IterationFunc func() (CheckFunc, CreateFunc, HandleFunc)

// IterateKeysByPrefixRange will iterate over all entries in the database, where the key starts with a prefixes in
// the range [startPrefix, endPrefix] (both inclusive). We require that startPrefix <= endPrefix (otherwise this
// function errors). On every such key, the `check` function is called. If `check` errors, iteration is aborted.
// In other words, error returned by the iteration functions will be propagated to the caller.
// No errors expected during normal operations.
func IterateKeysByPrefixRange(r storage.Reader, startPrefix []byte, endPrefix []byte, check func(key []byte) error) error {
	return IterateKeys(r, startPrefix, endPrefix, func() (CheckFunc, CreateFunc, HandleFunc) {
		return func(key []byte) (bool, error) {
			err := check(key)
			if err != nil {
				return false, err
			}
			return false, nil
		}, nil, nil
	}, storage.IteratorOption{BadgerIterateKeyOnly: true})
}

// IterateKeys will iterate over all entries in the database, where the key starts with a prefixes in
// the range [startPrefix, endPrefix] (both inclusive).
// No errors expected during normal operations.
func IterateKeys(r storage.Reader, startPrefix []byte, endPrefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) error {
	if len(startPrefix) == 0 {
		return fmt.Errorf("startPrefix prefix is empty")
	}

	if len(endPrefix) == 0 {
		return fmt.Errorf("endPrefix prefix is empty")
	}

	// Reverse iteration is not supported by pebble
	if bytes.Compare(startPrefix, endPrefix) > 0 {
		return fmt.Errorf("startPrefix key must be less than or equal to endPrefix key")
	}

	it, err := r.NewIter(startPrefix, endPrefix, opt)
	if err != nil {
		return fmt.Errorf("can not create iterator: %w", err)
	}
	defer it.Close()

	for it.First(); it.Valid(); it.Next() {
		item := it.IterItem()
		key := item.Key()

		// initialize processing functions for iteration
		check, create, handle := iterFunc()

		keyCopy := make([]byte, len(key))

		// The underlying database may re-use and modify the backing memory of the returned key.
		// Tor safety we proactively make a copy before passing the key to the upper layer.
		copy(keyCopy, key)

		// check if we should process the item at all
		shouldReadValue, err := check(keyCopy)
		if err != nil {
			return err
		}
		if !shouldReadValue { // skip reading value
			continue
		}

		err = item.Value(func(val []byte) error {

			// decode into the entity
			entity := create()
			err = msgpack.Unmarshal(val, entity)
			if err != nil {
				return irrecoverable.NewExceptionf("could not decode entity: %w", err)
			}

			// process the entity
			err = handle()
			if err != nil {
				return fmt.Errorf("could not handle entity: %w", err)
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("could not process value: %w", err)
		}
	}

	return nil
}

// Traverse will iterate over all keys with the given prefix
// error returned by the iteration functions will be propagated to the caller.
// No other errors are expected during normal operation.
func TraverseByPrefix(r storage.Reader, prefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) error {
	return IterateKeys(r, prefix, prefix, iterFunc, opt)
}

// KeyExists returns true if a key exists in the database.
// When this returned function is executed (and only then), it will write into the `keyExists` whether
// the key exists.
// No errors are expected during normal operation.
func KeyExists(r storage.Reader, key []byte) (exist bool, errExit error) {
	_, closer, err := r.Get(key)
	if err != nil {
		// the key does not exist in the database
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		// exception while checking for the key
		return false, irrecoverable.NewExceptionf("could not load data: %w", err)
	}
	defer func() {
		errExit = closer.Close()
	}()

	// the key does exist in the database
	return true, nil
}

// RetrieveByKey will retrieve the binary data under the given key from the database
// and decode it into the given entity. The provided entity needs to be a
// pointer to an initialized entity of the correct type.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func RetrieveByKey(r storage.Reader, key []byte, entity interface{}) (errExit error) {
	val, closer, err := r.Get(key)
	if err != nil {
		return err
	}

	defer func() {
		errExit = closer.Close()
	}()

	err = msgpack.Unmarshal(val, entity)
	if err != nil {
		return irrecoverable.NewExceptionf("could not decode entity: %w", err)
	}
	return nil
}

// FindHighestAtOrBelowByPrefix is for database entries that are indexed by block height. It is suitable to search
// keys with the format prefix` + `height` (where "+" denotes concatenation of binary strings). The height
// is encoded as Big-Endian (entries with numerically smaller height have lexicographically smaller key).
// The function finds the *highest* key with the given prefix and height equal to or below the given height.
func FindHighestAtOrBelowByPrefix(r storage.Reader, prefix []byte, height uint64, entity interface{}) (errExit error) {
	if len(prefix) == 0 {
		return fmt.Errorf("prefix must not be empty")
	}

	key := append(prefix, EncodeKeyPart(height)...)
	it, err := r.NewIter(prefix, key, storage.DefaultIteratorOptions())
	if err != nil {
		return fmt.Errorf("can not create iterator: %w", err)
	}
	defer func() {
		errExit = it.Close()
	}()

	var highestKey []byte

	// find highest value below the given height
	for it.First(); it.Valid(); it.Next() {
		// copy the key to avoid the underlying slices of the key
		// being modified by the Next() call
		highestKey = it.IterItem().KeyCopy(highestKey)
	}

	if len(highestKey) == 0 {
		return storage.ErrNotFound
	}

	// read the value of the highest key
	val, closer, err := r.Get(highestKey)
	if err != nil {
		return err
	}

	defer func() {
		errExit = closer.Close()
	}()

	err = msgpack.Unmarshal(val, entity)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to decode value: %w", err)
	}

	return nil
}
