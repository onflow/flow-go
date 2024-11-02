package pebbleimpl

import (
	"errors"
	"io"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type dbReader struct {
	db *pebble.DB
}

var _ storage.Reader = (*dbReader)(nil)

type noopCloser struct{}

var _ io.Closer = (*noopCloser)(nil)

func (noopCloser) Close() error { return nil }

// Get gets the value for the given key. It returns ErrNotFound if the DB
// does not contain the key.
// other errors are exceptions
//
// The caller should not modify the contents of the returned slice, but it is
// safe to modify the contents of the argument after Get returns. The
// returned slice will remain valid until the returned Closer is closed. On
// success, the caller MUST call closer.Close() or a memory leak will occur.
func (b dbReader) Get(key []byte) ([]byte, io.Closer, error) {
	value, closer, err := b.db.Get(key)

	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil, storage.ErrNotFound
		}

		// exception while checking for the key
		return nil, nil, irrecoverable.NewExceptionf("could not load data: %w", err)
	}

	return value, closer, nil
}

// NewIter returns a new Iterator for the given key prefix range [startPrefix, endPrefix], both inclusive.
// Specifically, all keys that meet ANY of the following conditions are included in the iteration:
//   - have a prefix equal to startPrefix OR
//   - have a prefix equal to the endPrefix OR
//   - have a prefix that is lexicographically between startPrefix and endPrefix
func (b dbReader) NewIter(startPrefix, endPrefix []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	return newPebbleIterator(b.db, startPrefix, endPrefix, ops)
}

// ToReader is a helper function to convert a *pebble.DB to a Reader
func ToReader(db *pebble.DB) storage.Reader {
	return dbReader{db}
}
