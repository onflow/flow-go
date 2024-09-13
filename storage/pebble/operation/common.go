package operation

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type ReaderBatchWriter struct {
	db        *pebble.DB
	batch     *pebble.Batch
	callbacks []func(error)
}

var _ storage.PebbleReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) ReaderWriter() (pebble.Reader, pebble.Writer) {
	return b.db, b.batch
}

func (b *ReaderBatchWriter) IndexedBatch() *pebble.Batch {
	return b.batch
}

func (b *ReaderBatchWriter) Commit() error {
	err := b.batch.Commit(nil)

	for _, callback := range b.callbacks {
		callback(err)
	}

	return err
}

func (b *ReaderBatchWriter) AddCallback(callback func(error)) {
	b.callbacks = append(b.callbacks, callback)
}

func NewPebbleReaderBatchWriterWithBatch(db *pebble.DB, batch *pebble.Batch) *ReaderBatchWriter {
	return &ReaderBatchWriter{
		db:        db,
		batch:     batch,
		callbacks: make([]func(error), 0),
	}
}

func NewPebbleReaderBatchWriter(db *pebble.DB) *ReaderBatchWriter {
	return &ReaderBatchWriter{
		db:    db,
		batch: db.NewIndexedBatch(),
	}
}

func WithReaderBatchWriter(db *pebble.DB, fn func(storage.PebbleReaderBatchWriter) error) error {
	batch := NewPebbleReaderBatchWriter(db)
	err := fn(batch)
	if err != nil {
		return err
	}
	err = batch.Commit()
	if err != nil {
		return err
	}

	return nil
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

// iterate iterates over a range of keys defined by a start and end key. The
// start key may be higher than the end key, in which case we iterate in
// reverse order.
//
// The iteration range uses prefix-wise semantics. Specifically, all keys that
// meet ANY of the following conditions are included in the iteration:
//   - have a prefix equal to the start key OR
//   - have a prefix equal to the end key OR
//   - have a prefix that is lexicographically between start and end
//
// On each iteration, it will call the iteration function to initialize
// functions specific to processing the given key-value pair.
//
// TODO: this function is unbounded – pass context.Context to this or calling functions to allow timing functions out.
// No errors are expected during normal operation. Any errors returned by the
// provided handleFunc will be propagated back to the caller of iterate.
func iterate(start []byte, end []byte, iteration iterationFunc, prefetchValues bool) func(pebble.Reader) error {
	return func(tx pebble.Reader) error {
		// Reverse iteration is not supported by pebble
		if bytes.Compare(start, end) > 0 {
			return fmt.Errorf("start key must be less than or equal to end key")
		}

		// initialize the default options and comparison modifier for iteration
		maxEndWithPrefix := append(end, ffBytes...)

		options := pebble.IterOptions{
			LowerBound: start,
			UpperBound: maxEndWithPrefix,
		}

		// In order to satisfy this function's prefix-wise inclusion semantics,
		// we append 0xff bytes to the largest of start and end.
		// This ensures Badger will seek to the largest key with that prefix
		// for reverse iteration, thus including all keys with a prefix matching
		// the starting key. It also enables us to detect boundary conditions by
		// simple lexicographic comparison (ie. bytes.Compare) rather than
		// explicitly comparing prefixes.

		// If start is bigger than end, we have a backwards iteration:
		// 1) We set the reverse option on the iterator, so we step through all
		//    the keys backwards. This modifies the behaviour of Seek to go to
		//    the first key that is less than or equal to the start key (as
		//    opposed to greater than or equal in a regular iteration).
		// 2) In order to satisfy this function's prefix-wise inclusion semantics,
		//    we append a 0xff-byte suffix to the start key so the seek will go
		// to the right place.
		// 3) For a regular iteration, we break the loop upon hitting the first
		//    item that has a key higher than the end prefix. In order to reverse
		//    this, we use a modifier for the comparison that reverses the check
		//    and makes it stop upon the first item lower than the end prefix.
		// for forward iteration, add the 0xff-bytes suffix to the end
		// prefix, to ensure we include all keys with that prefix before
		// finishing.
		it, err := tx.NewIter(&options)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		for it.SeekGE(start); it.Valid(); it.Next() {
			key := it.Key()

			// initialize processing functions for iteration
			check, create, handle := iteration()

			// check if we should process the item at all
			ok := check(key)
			if !ok {
				continue
			}

			// when prefetchValues is false, we skip loading the value
			if !prefetchValues {
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

var ffBytes = bytes.Repeat([]byte{0xFF}, 32)

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

		it, err := r.NewIter(&pebble.IterOptions{
			LowerBound: prefix,
			UpperBound: append(prefix, ffBytes...),
		})

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
// note: DeleteRange takes [start, end) (inclusion on start, exclusive on end),
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

// O(N) performance
func findHighestAtOrBelow(
	prefix []byte,
	height uint64,
	entity interface{},
) func(pebble.Reader) error {
	return func(r pebble.Reader) error {
		if len(prefix) == 0 {
			return fmt.Errorf("prefix must not be empty")
		}

		key := append(prefix, b(height)...)
		it, err := r.NewIter(&pebble.IterOptions{
			UpperBound: append(key, ffBytes...),
		})
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		var highestKey []byte
		// find highest value below the given height
		for it.SeekGE(prefix); it.Valid(); it.Next() {
			highestKey = it.Key()
		}

		if len(highestKey) == 0 {
			return storage.ErrNotFound
		}

		// read the value of the highest key
		val, closer, err := r.Get(highestKey)
		if err != nil {
			return convertNotFoundError(err)
		}

		defer closer.Close()

		err = msgpack.Unmarshal(val, entity)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to decode value: %w", err)
		}

		return nil
	}
}

func BatchUpdate(db *pebble.DB, fn func(tx pebble.Writer) error) error {
	batch := db.NewIndexedBatch()
	err := fn(batch)
	if err != nil {
		return err
	}
	return batch.Commit(nil)
}
