package badgerimpl

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

func ToDB(db *badger.DB) storage.DB {
	return &dbStore{db: db}
}

type dbStore struct {
	db *badger.DB
}

func (b *dbStore) Reader() storage.Reader {
	return dbReader{db: b.db}
}

func (b *dbStore) WithReaderBatchWriter(fn func(storage.BaseReaderBatchWriter) error) error {
	return WithReaderBatchWriter(b.db, fn)
}
