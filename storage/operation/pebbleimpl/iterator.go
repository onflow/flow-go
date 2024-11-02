package pebbleimpl

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

type pebbleIterator struct {
	iter *pebble.Iterator
}

var _ storage.Iterator = (*pebbleIterator)(nil)

func newPebbleIterator(reader pebble.Reader, startPrefix, endPrefix []byte, ops storage.IteratorOption) (*pebbleIterator, error) {
	lowerBound, upperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)

	options := pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	}

	iter, err := reader.NewIter(&options)
	if err != nil {
		return nil, fmt.Errorf("can not create iterator: %w", err)
	}

	return &pebbleIterator{
		iter: iter,
	}, nil
}

// First seeks to the smallest key greater than or equal to the given key.
func (i *pebbleIterator) First() {
	i.iter.First()
}

// Valid returns whether the iterator is positioned at a valid key-value pair.
func (i *pebbleIterator) Valid() bool {
	return i.iter.Valid()
}

// Next advances the iterator to the next key-value pair.
func (i *pebbleIterator) Next() {
	i.iter.Next()
}

// IterItem returns the current key-value pair, or nil if done.
func (i *pebbleIterator) IterItem() storage.IterItem {
	return pebbleIterItem{iter: i.iter}
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

// Close closes the iterator. Iterator must be closed, otherwise it causes memory leak.
// No errors expected during normal operation
func (i *pebbleIterator) Close() error {
	return i.iter.Close()
}
