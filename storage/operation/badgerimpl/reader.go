package badgerimpl

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type dbReader struct {
	db *badger.DB
}

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
	tx := b.db.NewTransaction(false)
	defer tx.Discard()

	item, err := tx.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, nil, storage.ErrNotFound
		}
		return nil, nil, irrecoverable.NewExceptionf("could not load data: %w", err)
	}

	value, err := item.ValueCopy(nil)
	if err != nil {
		return nil, nil, irrecoverable.NewExceptionf("could not load value: %w", err)
	}

	return value, noopCloser{}, nil
}

// NewIter returns a new Iterator for the given key prefix range [startPrefix, endPrefix], both inclusive.
// Specifically, all keys that meet ANY of the following conditions are included in the iteration:
//   - have a prefix equal to startPrefix OR
//   - have a prefix equal to the endPrefix OR
//   - have a prefix that is lexicographically between startPrefix and endPrefix
//
// it returns error if the startPrefix key is greater than the endPrefix key
// no errors are expected during normal operation
func (b dbReader) NewIter(startPrefix, endPrefix []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	if bytes.Compare(startPrefix, endPrefix) > 0 {
		return nil, fmt.Errorf("startPrefix key must be less than or equal to endPrefix key")
	}

	return newBadgerIterator(b.db, startPrefix, endPrefix, ops), nil
}

// ToReader is a helper function to convert a *badger.DB to a Reader
func ToReader(db *badger.DB) storage.Reader {
	return dbReader{db}
}
