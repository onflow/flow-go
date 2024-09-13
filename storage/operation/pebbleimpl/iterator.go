package pebbleimpl

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

func (i *pebbleIterator) Close() error {
	return i.iter.Close()
}
