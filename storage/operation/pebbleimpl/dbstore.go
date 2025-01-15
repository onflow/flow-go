package pebbleimpl

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

func ToDB(db *pebble.DB) storage.DB {
	return &dbStore{db: db}
}

type dbStore struct {
	db *pebble.DB
}

func (b *dbStore) Reader() storage.Reader {
	return dbReader{db: b.db}
}

func (b *dbStore) WithReaderBatchWriter(fn func(storage.ReaderBatchWriter) error) error {
	return WithReaderBatchWriter(b.db, fn)
}

func (b *dbStore) NewBatch() storage.Batch {
	return NewReaderBatchWriter(b.db)
}
