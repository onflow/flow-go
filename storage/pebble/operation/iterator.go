package operation

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

type pebbleIterator struct {
	iter       *pebble.Iterator
	lowerBound []byte
}

var _ storage.Iterator = (*pebbleIterator)(nil)

func StartEndPrefixToLowerUpperBound(start, end []byte) (lowerBound, upperBound []byte) {
	// LowerBound specifies the smallest key to iterate and it's inclusive.
	// UpperBound specifies the largest key to iterate and it's exclusive (not inclusive)
	// in order to match all keys prefixed with the `end` bytes, we increment the bytes of end by 1,
	// for instance, to iterate keys between "hello" and "world",
	// we use "hello" as LowerBound, "worle" as UpperBound, so that "world", "world1", "worldffff...ffff"
	// will all be included.
	return start, prefixUpperBound(end)
}

func newPebbleIterator(reader pebble.Reader, start, end []byte, ops storage.IteratorOption) (*pebbleIterator, error) {
	lowerBound, upperBound := StartEndPrefixToLowerUpperBound(start, end)

	options := pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	}

	iter, err := reader.NewIter(&options)
	if err != nil {
		return nil, fmt.Errorf("can not create iterator: %w", err)
	}

	return &pebbleIterator{
		iter:       iter,
		lowerBound: lowerBound,
	}, nil
}

func (i *pebbleIterator) SeekGE() {
	i.iter.SeekGE(i.lowerBound)
}

func (i *pebbleIterator) Valid() bool {
	return i.iter.Valid()
}

func (i *pebbleIterator) Next() {
	i.iter.Next()
}

func (i *pebbleIterator) IterItem() storage.IterItem {
	return pebbleIterItem{}
}

type pebbleIterItem struct {
	iter *pebble.Iterator
}

var _ storage.IterItem = (*pebbleIterItem)(nil)

func (i pebbleIterItem) Key() []byte {
	return i.iter.Key()
}

func (i pebbleIterItem) Value(fn func([]byte) error) error {
	val, err := i.iter.ValueAndErr()
	if err != nil {
		return err
	}

	return fn(val)
}

func (i *pebbleIterator) Close() error {
	return i.iter.Close()
}

// prefixUpperBound returns a key K such that all possible keys beginning with the input prefix
// sort lower than K according to the byte-wise lexicographic key ordering used by Pebble.
// This is used to define an upper bound for iteration, when we want to iterate over
// all keys beginning with a given prefix.
// referred to https://pkg.go.dev/github.com/cockroachdb/pebble#example-Iterator-PrefixIteration
func prefixUpperBound(prefix []byte) []byte {
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
