// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Index implements a simple read-only payload storage around a badger DB.
type Index struct {
	db    *badger.DB
	cache *Cache
}

func NewIndex(collector module.CacheMetrics, db *badger.DB) *Index {

	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		blockID := key.(flow.Identifier)
		index := val.(*flow.Index)
		return procedure.InsertIndex(blockID, index)
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var index flow.Index
		return func(tx *badger.Txn) (interface{}, error) {
			err := procedure.RetrieveIndex(blockID, &index)(tx)
			return &index, err
		}
	}

	p := &Index{
		db: db,
		cache: newCache(collector, metrics.ResourceIndex,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return p
}

func (i *Index) storeTx(blockID flow.Identifier, index *flow.Index) func(*badger.Txn) error {
	return i.cache.Put(blockID, index)
}

func (i *Index) storeTxn(blockID flow.Identifier, index *flow.Index) func(*transaction.Tx) error {
	return i.cache.PutTxn(blockID, index)
}

func (i *Index) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Index, error) {
	return func(tx *badger.Txn) (*flow.Index, error) {
		val, err := i.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.Index), nil
	}
}

func (i *Index) Store(blockID flow.Identifier, index *flow.Index) error {
	return operation.RetryOnConflict(i.db.Update, i.storeTx(blockID, index))
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	tx := i.db.NewTransaction(false)
	defer tx.Discard()
	return i.retrieveTx(blockID)(tx)
}
