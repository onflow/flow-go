package store

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/procedure"
)

// Index implements a simple read-only payload storage around a badger DB.
type Index struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.Index]
}

func NewIndex(collector module.CacheMetrics, db storage.DB) *Index {
	indexing := &sync.Mutex{}

	store := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
		return procedure.InsertIndex(indexing, rw, blockID, index)
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) (*flow.Index, error) {
		var index flow.Index
		err := procedure.RetrieveIndex(r, blockID, &index)
		return &index, err
	}

	p := &Index{
		db: db,
		cache: newCache(collector, metrics.ResourceIndex,
			withLimit[flow.Identifier, *flow.Index](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return p
}

func (i *Index) storeTx(rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
	return i.cache.PutTx(rw, blockID, index)
}

func (i *Index) Store(blockID flow.Identifier, index *flow.Index) error {
	return i.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return i.storeTx(rw, blockID, index)
	})
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	val, err := i.cache.Get(i.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}
