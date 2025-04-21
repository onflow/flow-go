package operation

import (
	"bytes"
	"errors"
	"io"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/noop"
)

type multiReader struct {
	readers []storage.Reader
}

var _ storage.Reader = (*multiReader)(nil)

// NewMultiReader returns a Reader that consists of multiple readers
// in the provided order.  Readers are read sequentially until
// - a reader succeeds or
// - a reader returns an error that is not ErrNotFound
// If all readers return ErrNotFound, Reader.Get will return ErrNotFound.
func NewMultiReader(readers ...storage.Reader) storage.Reader {
	if len(readers) == 1 {
		return readers[0]
	}
	return &multiReader{readers: readers}
}

// Get gets the value for the given key from one of the readers.
// Readers are read sequentially until
// - a reader succeeds or
// - a reader returns an error that is not ErrNotFound
// If all readers return ErrNotFound, Get will return ErrNotFound.
// Other errors are exceptions.
//
// The caller should not modify the contents of the returned slice, but it is
// safe to modify the contents of the argument after Get returns. The
// returned slice will remain valid until the returned Closer is closed.
// when err == nil, the caller MUST call closer.Close() or a memory leak will occur.
func (b *multiReader) Get(key []byte) (value []byte, closer io.Closer, err error) {
	for _, r := range b.readers {
		value, closer, err = r.Get(key)
		if err == nil || !errors.Is(err, storage.ErrNotFound) {
			return
		}
	}
	return nil, noop.Closer{}, storage.ErrNotFound
}

// NewIter returns a new Iterator for the given key prefix range [startPrefix, endPrefix], both inclusive.
// Specifically, all keys that meet ANY of the following conditions are included in the iteration:
//   - have a prefix equal to startPrefix OR
//   - have a prefix equal to the endPrefix OR
//   - have a prefix that is lexicographically between startPrefix and endPrefix
//
// Returned new iterator consists of multiple iterators in reverse order from underlying readers.
// For example, the first iterator is created from the last underlying reader.
// This is to ensure that legacy databases are iterated first to preserve key orders.
//
// NewIter returns error if the startPrefix key is greater than the endPrefix key.
// No errors are expected during normal operation.
func (b *multiReader) NewIter(startPrefix, endPrefix []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	if bytes.Compare(startPrefix, endPrefix) > 0 {
		return nil, errors.New("startPrefix key must be less than or equal to endPrefix key")
	}

	// Create iterators from readers in reverse order
	// because we want to iterate legacy databases first
	// to preserve key orders.
	iterators := make([]storage.Iterator, len(b.readers))
	for i, r := range b.readers {
		iterator, err := r.NewIter(startPrefix, endPrefix, ops)
		if err != nil {
			return nil, err
		}
		iterators[len(b.readers)-1-i] = iterator
	}

	return NewMultiIterator(iterators...)
}

func (b *multiReader) NewSeeker() storage.Seeker {
	panic("not implemented")
}
