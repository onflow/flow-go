package pops

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

func (b dbReader) NewIter(startPrefix, endPrefix []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	return newPebbleIterator(b.db, startPrefix, endPrefix, ops)
}

// ToReader is a helper function to convert a *pebble.DB to a Reader
func ToReader(db *pebble.DB) storage.Reader {
	return dbReader{db}
}
