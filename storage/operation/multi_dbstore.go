package operation

import "github.com/onflow/flow-go/storage"

type multiDBStore struct {
	rwStore storage.DB // primary read and write store
	r       storage.DB // secondary read store
}

var _ (storage.DB) = (*multiDBStore)(nil)

// NewMultiDBStore returns a DB store that consists of a primary
// read-and-write store, and a secondary read-only store.
func NewMultiDBStore(rwStore storage.DB, rStore storage.DB) storage.DB {
	return &multiDBStore{
		rwStore: rwStore,
		r:       rStore,
	}
}

func (b *multiDBStore) Reader() storage.Reader {
	return NewMultiReader(b.rwStore.Reader(), b.r.Reader())
}

func (b *multiDBStore) WithReaderBatchWriter(fn func(storage.ReaderBatchWriter) error) error {
	return b.rwStore.WithReaderBatchWriter(fn)
}

func (b *multiDBStore) NewBatch() storage.Batch {
	return b.rwStore.NewBatch()
}
