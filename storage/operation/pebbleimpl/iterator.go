package pebbleimpl

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

type pebbleIterator struct {
	*pebble.Iterator
}

var _ storage.Iterator = (*pebbleIterator)(nil)

func newPebbleIterator(reader pebble.Reader, startPrefix, endPrefix []byte, ops storage.IteratorOption) (*pebbleIterator, error) {
	lowerBound, upperBound, hasUpperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)

	options := pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	}

	// setting UpperBound to nil if there is no upper bound
	if !hasUpperBound {
		options.UpperBound = nil
	}

	iter, err := reader.NewIter(&options)
	if err != nil {
		return nil, fmt.Errorf("can not create iterator: %w", err)
	}

	return &pebbleIterator{
		iter,
	}, nil
}

// IterItem returns the current key-value pair, or nil if done.
func (i *pebbleIterator) IterItem() storage.IterItem {
	return pebbleIterItem{i.Iterator}
}

// Next seeks to the smallest key greater than or equal to the given key.
func (i *pebbleIterator) Next() {
	i.Iterator.Next()
}

type pebbleIterItem struct {
	*pebble.Iterator
}

var _ storage.IterItem = (*pebbleIterItem)(nil)

// KeyCopy returns a copy of the key of the item, writing it to dst slice.
func (i pebbleIterItem) KeyCopy(dst []byte) []byte {
	return append(dst[:0], i.Key()...)
}

func (i pebbleIterItem) Value(fn func([]byte) error) error {
	val, err := i.ValueAndErr()
	if err != nil {
		return err
	}

	return fn(val)
}
