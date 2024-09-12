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

func (i *badgerIterator) SeekGE() {
	i.iter.Seek(i.lowerBound)
}

func (i *badgerIterator) Valid() bool {
	// if it's beyond the upper bound, it's invalid
	if !i.iter.Valid() {
		return false
	}
	key := i.iter.Item().Key()
	// "< 0" means the upperBound is exclusive
	valid := bytes.Compare(key, i.upperBound) < 0
	return valid
}

func (i *badgerIterator) Next() {
	i.iter.Next()
}

func (i *badgerIterator) IterItem() storage.IterItem {
	return i.iter.Item()
}

var _ storage.IterItem = (*badger.Item)(nil)

func (i *badgerIterator) Close() error {
	i.iter.Close()
	return nil
}
