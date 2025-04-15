package badgerimpl

import (
	"github.com/dgraph-io/badger/v4"

	"github.com/onflow/flow-go/storage"
)

func ToDB(db *badger.DB) storage.DB {
	return &dbStore{db: db}
}

type dbStore struct {
	db *badger.DB
}

var _ (storage.DB) = (*dbStore)(nil)

func (b *dbStore) Reader() storage.Reader {
	return dbReader{db: b.db}
}

func (b *dbStore) WithReaderBatchWriter(fn func(storage.ReaderBatchWriter) error) error {
	return WithReaderBatchWriter(b.db, fn)
}

func (b *dbStore) NewBatch() storage.Batch {
	return NewReaderBatchWriter(b.db)
}
