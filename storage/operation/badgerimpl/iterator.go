package badgerimpl

import (
	"bytes"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

type badgerIterator struct {
	iter       *badger.Iterator
	lowerBound []byte
	upperBound []byte
}

var _ storage.Iterator = (*badgerIterator)(nil)

func newBadgerIterator(db *badger.DB, startPrefix, endPrefix []byte, ops storage.IteratorOption) *badgerIterator {
	options := badger.DefaultIteratorOptions
	if ops.IterateKeyOnly {
		options.PrefetchValues = false
	}

	tx := db.NewTransaction(false)
	iter := tx.NewIterator(options)

	lowerBound, upperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)

	return &badgerIterator{
		iter:       iter,
		lowerBound: lowerBound,
		upperBound: upperBound,
	}
}

// First seeks to the smallest key greater than or equal to the given key.
func (i *badgerIterator) First() {
	i.iter.Seek(i.lowerBound)
}

// Valid returns whether the iterator is positioned at a valid key-value pair.
func (i *badgerIterator) Valid() bool {
	// Note: we didn't specify the iteration range with the badger IteratorOptions,
	// because the IterationOptions only allows us to specify a single prefix, whereas
	// we need to specify a range of prefixes. So we have to manually check the bounds here.
	// The First() method, which calls Seek(i.lowerBound), ensures the iteration starts from
	// the lowerBound, and the upperbound is checked here by first checking if it's
	// reaching the end of the iteration, then checking if the key is within the upperbound.

	// check if it's reaching the end of the iteration
	if !i.iter.Valid() {
		return false
	}

	// check if the key is within the upperbound (exclusive)
	key := i.iter.Item().Key()
	// note: for the boundary case,
	// upperBound is the exclusive upper bound, should not be included in the iteration,
	// so if key == upperBound, it's invalid, should return false.
	valid := bytes.Compare(key, i.upperBound) < 0
	return valid
}

// Next advances the iterator to the next key-value pair.
func (i *badgerIterator) Next() {
	i.iter.Next()
}

// IterItem returns the current key-value pair, or nil if done.
func (i *badgerIterator) IterItem() storage.IterItem {
	return i.iter.Item()
}

var _ storage.IterItem = (*badger.Item)(nil)

// Close closes the iterator. Iterator must be closed, otherwise it causes memory leak.
// No errors expected during normal operation
func (i *badgerIterator) Close() error {
	i.iter.Close()
	return nil
}
