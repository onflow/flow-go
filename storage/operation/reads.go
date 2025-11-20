package operation

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/merr"
)

// IterationFunc is a callback function that will be called on each key-value pair during the iteration.
// The key is copied and passed to the function, so key can be modified or retained after iteration.
// The `getValue` function can be called to retrieve the value of the current key and decode value into destVal object.
// The caller can return (true, nil) to stop the iteration early.
type IterationFunc func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error)

// IterateKeysByPrefixRange will iterate over all entries in the database, where the key starts with a prefixes in
// the range [startPrefix, endPrefix] (both inclusive). We require that startPrefix <= endPrefix (otherwise this
// function errors). On every such key, the `check` function is called. If `check` errors, iteration is aborted.
// In other words, error returned by the iteration functions will be propagated to the caller.
// No errors expected during normal operations.
func IterateKeysByPrefixRange(r storage.Reader, startPrefix []byte, endPrefix []byte, check func(key []byte) error) error {
	iterFunc := func(key []byte, getValue func(destVal any) error) (bail bool, err error) {
		err = check(key)
		if err != nil {
			return true, err
		}
		return false, nil
	}
	return IterateKeys(r, startPrefix, endPrefix, iterFunc, storage.IteratorOption{BadgerIterateKeyOnly: true})
}

// IterateKeys will iterate over all entries in the database, where the key starts with a prefixes in
// the range [startPrefix, endPrefix] (both inclusive).
// No errors expected during normal operations.
func IterateKeys(r storage.Reader, startPrefix []byte, endPrefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) (errToReturn error) {
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
	defer func() {
		errToReturn = merr.CloseAndMergeError(it, errToReturn)
	}()

	for it.First(); it.Valid(); it.Next() {
		item := it.IterItem()
		key := item.Key()

		keyCopy := make([]byte, len(key))

		// The underlying database may re-use and modify the backing memory of the returned key.
		// Tor safety we proactively make a copy before passing the key to the upper layer.
		copy(keyCopy, key)

		// check if we should process the item at all
		bail, err := iterFunc(keyCopy, func(destVal any) error {
			return item.Value(func(val []byte) error {
				return msgpack.Unmarshal(val, destVal)
			})
		})
		if err != nil {
			return err
		}
		if bail {
			return nil
		}
	}

	return nil
}

// TraverseByPrefix will iterate over all keys with the given prefix
// error returned by the iteration functions will be propagated to the caller.
// No other errors are expected during normal operation.
func TraverseByPrefix(r storage.Reader, prefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) error {
	return IterateKeys(r, prefix, prefix, iterFunc, opt)
}

// KeyOnlyIterateFunc returns an IterationFunc that only iterates over keys
func KeyOnlyIterateFunc(fn func(key []byte) error) IterationFunc {
	return func(key []byte, _ func(destVal any) error) (bail bool, err error) {
		err = fn(key)
		if err != nil {
			return true, err
		}
		return false, nil
	}
}

// KeyExists returns true if a key exists in the database.
// When this returned function is executed (and only then), it will write into the `keyExists` whether
// the key exists.
// No errors are expected during normal operation.
func KeyExists(r storage.Reader, key []byte) (exist bool, errToReturn error) {
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
		errToReturn = merr.CloseAndMergeError(closer, errToReturn)
	}()

	// the key does exist in the database
	return true, nil
}

// RetrieveByKey will retrieve the binary data under the given key from the database
// and decode it into the given entity. The provided entity needs to be a
// pointer to an initialized entity of the correct type.
// Error returns:
//   - [storage.ErrNotFound] if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func RetrieveByKey(r storage.Reader, key []byte, entity any) (errToReturn error) {
	val, closer, err := r.Get(key)
	if err != nil {
		return err
	}

	defer func() {
		errToReturn = merr.CloseAndMergeError(closer, errToReturn)
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
func FindHighestAtOrBelowByPrefix(r storage.Reader, prefix []byte, height uint64, entity any) (errToReturn error) {
	if len(prefix) == 0 {
		return fmt.Errorf("prefix must not be empty")
	}

	key := append(prefix, EncodeKeyPart(height)...)

	seeker := r.NewSeeker()

	// Seek the highest key equal or below to the given key within the [prefix, key] prefix range
	highestKey, err := seeker.SeekLE(prefix, key)
	if err != nil {
		if err == storage.ErrNotFound {
			return storage.ErrNotFound
		}
		return fmt.Errorf("can not seek height %d: %w", height, err)
	}

	// read the value of the highest key
	val, closer, err := r.Get(highestKey)
	if err != nil {
		return err
	}

	defer func() {
		errToReturn = merr.CloseAndMergeError(closer, errToReturn)
	}()

	err = msgpack.Unmarshal(val, entity)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to decode value: %w", err)
	}

	return nil
}

// CommonPrefix returns common prefix of startPrefix and endPrefix.
// The common prefix is used to narrow down the SSTables that
// BadgerDB's iterator picks up.
func CommonPrefix(startPrefix, endPrefix []byte) []byte {
	commonPrefixMaxLength := min(
		len(startPrefix),
		len(endPrefix),
	)

	commonPrefixLength := commonPrefixMaxLength
	for i := range commonPrefixMaxLength {
		if startPrefix[i] != endPrefix[i] {
			commonPrefixLength = i
			break
		}
	}

	if commonPrefixLength == 0 {
		return nil
	}
	return slices.Clone(startPrefix[:commonPrefixLength])
}
