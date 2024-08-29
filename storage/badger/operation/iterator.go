package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

type badgerIterator struct {
	iter       *badger.Iterator
	lowerBound []byte
	upperBound []byte
}

var _ storage.Iterator = (*badgerIterator)(nil)

func StartEndPrefixToLowerUpperBound(start, end []byte) (lowerBound, upperBound []byte) {
	// TODO:
	return nil, nil
}

func newBadgerIterator(db *badger.DB, start, end []byte, ops storage.IteratorOption) *badgerIterator {
	options := badger.DefaultIteratorOptions
	if ops.IterateKeyOnly {
		options.PrefetchValues = false
	}

	tx := db.NewTransaction(false)
	iter := tx.NewIterator(options)

	lowerBound, upperBound := StartEndPrefixToLowerUpperBound(start, end)

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
	return i.iter.Valid()
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
