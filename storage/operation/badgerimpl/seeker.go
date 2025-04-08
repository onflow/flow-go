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

// SeekLE (seek less than or equal) returns given key if present.  Otherwise,
// it returns the largest key that is less than the given key within startPrefix
// and endPrefix.  Keys are ordered in lexicographical order.
// This function returns error if given key is outside range of startPrefix and endPrefix.
func (i *badgerSeeker) SeekLE(startPrefix, endPrefix []byte, key []byte) ([]byte, bool, error) {
	options := badger.DefaultIteratorOptions
	options.PrefetchValues = false
	options.Reverse = true

	lowerBound, _, hasUpperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)

	if bytes.Compare(key, startPrefix) < 0 {
		return nil, false, errors.New("key must be greater than or equal to startPrefix key")
	}

	if hasUpperBound && bytes.Compare(key, endPrefix) > 0 {
		return nil, false, errors.New("key must be less than or equal to endPrefix key")
	}

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
		return nil, false, nil
	}

	// Check if returned key is less than lowerBound.
	if bytes.Compare(iter.Item().Key(), lowerBound) < 0 {
		return nil, false, nil
	}

	return iter.Item().KeyCopy(nil), true, nil
}
