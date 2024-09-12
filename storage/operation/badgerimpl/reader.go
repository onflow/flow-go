package badgerimpl

import (
	"errors"
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

	var value []byte
	err = item.Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
	if err != nil {
		return nil, nil, irrecoverable.NewExceptionf("could not load value: %w", err)
	}

	return value, noopCloser{}, nil
}

func (b dbReader) NewIter(startPrefix, endPrefix []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	return newBadgerIterator(b.db, startPrefix, endPrefix, ops), nil
}

// ToReader is a helper function to convert a *badger.DB to a Reader
func ToReader(db *badger.DB) storage.Reader {
	return dbReader{db}
}
