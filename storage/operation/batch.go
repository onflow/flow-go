package operation

import (
	"bytes"
	"fmt"

	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type CheckFunc func(key []byte) bool

// createFunc returns a pointer to an initialized entity that we can potentially
// decode the next value into during a badger DB iteration.
type CreateFunc func() interface{}

// handleFunc is a function that starts the processing of the current key-value
// pair during a badger iteration. It should be called after the key was checked
// and the entity was decoded.
// No errors are expected during normal operation. Any errors will halt the iteration.
type HandleFunc func() error
type IterationFunc func() (CheckFunc, CreateFunc, HandleFunc)

func Iterate(start []byte, end []byte, iterFunc IterationFunc, opt storage.IteratorOption) func(storage.Reader) error {
	return func(r storage.Reader) error {

		if len(start) == 0 {
			return fmt.Errorf("start prefix is empty")
		}

		if len(end) == 0 {
			return fmt.Errorf("end prefix is empty")
		}

		// Reverse iteration is not supported by pebble
		if bytes.Compare(start, end) > 0 {
			return fmt.Errorf("start key must be less than or equal to end key")
		}

		it, err := r.NewIter(start, end, opt)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		for it.SeekGE(); it.Valid(); it.Next() {
			item := it.IterItem()
			key := item.Key()

			// initialize processing functions for iteration
			check, create, handle := iterFunc()

			// check if we should process the item at all
			ok := check(key)
			if !ok {
				continue
			}

			err := item.Value(func(val []byte) error {

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
}

func Traverse(prefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) func(storage.Reader) error {
	return Iterate(prefix, PrefixUpperBound(prefix), iterFunc, opt)
}

// PrefixUpperBound returns a key K such that all possible keys beginning with the input prefix
// sort lower than K according to the byte-wise lexicographic key ordering used by Pebble.
// This is used to define an upper bound for iteration, when we want to iterate over
// all keys beginning with a given prefix.
// referred to https://pkg.go.dev/github.com/cockroachdb/pebble#example-Iterator-PrefixIteration
func PrefixUpperBound(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		// increment the bytes by 1
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // no upper-bound
}
