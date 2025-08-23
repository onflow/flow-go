package store

import (
	"github.com/jordanschalm/lockctx"

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
	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
		return procedure.InsertIndex(lctx, rw, blockID, index)
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
			withStoreWithLock(storeWithLock),
			withRetrieve(retrieve)),
	}

	return p
}

func (i *Index) storeTx(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, index *flow.Index) error {
	return i.cache.PutWithLockTx(lctx, rw, blockID, index)
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	val, err := i.cache.Get(i.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return val, nil
}
