package badgerimpl

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

type badgerSeeker struct {
	db *badger.DB
}

var _ storage.Seeker = (*badgerSeeker)(nil)

func newBadgerSeeker(db *badger.DB) *badgerSeeker {
	return &badgerSeeker{db: db}
}

// SeekLE (seek less than or equal) returns the largest key in lexicographical
// order within inclusive range of [startPrefix, key].
// This function returns an error if specified key is less than startPrefix.
// This function returns storage.ErrNotFound if a key that matches
// the specified criteria is not found.
func (i *badgerSeeker) SeekLE(startPrefix, key []byte) ([]byte, error) {
	if bytes.Compare(key, startPrefix) < 0 {
		return nil, errors.New("key must be greater than or equal to startPrefix key")
	}

	options := badger.DefaultIteratorOptions
	options.PrefetchValues = false
	options.Reverse = true

	tx := i.db.NewTransaction(false)
	iter := tx.NewIterator(options)
	defer func() {
		iter.Close()
		tx.Discard()
	}()

	// Seek seeks to given key or largest key less than the given key because we are iterating backwards.
	iter.Seek(key)

	// Check if we reach the end of the iteration.
	if !iter.Valid() {
		return nil, storage.ErrNotFound
	}

	item := iter.Item()

	// Check if returned key is less than startPrefix.
	if bytes.Compare(item.Key(), startPrefix) < 0 {
		return nil, storage.ErrNotFound
	}

	return item.KeyCopy(nil), nil
}
