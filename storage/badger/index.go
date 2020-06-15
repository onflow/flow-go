// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Index implements a simple read-only payload storage around a badger DB.
type Index struct {
	db    *badger.DB
	cache *Cache
}

func NewIndex(collector module.CacheMetrics, db *badger.DB) *Index {

	store := func(blockID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		index := v.(*flow.Index)
		return procedure.InsertIndex(blockID, index)
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
		var index flow.Index
		return func(tx *badger.Txn) (interface{}, error) {
			err := procedure.RetrieveIndex(blockID, &index)(tx)
			return &index, err
		}
	}

	p := &Index{
		db: db,
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceIndex)),
	}

	return p
}

func (i *Index) storeTx(blockID flow.Identifier, index *flow.Index) func(*badger.Txn) error {
	return i.cache.Put(blockID, index)
}

func (i *Index) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Index, error) {
	return func(tx *badger.Txn) (*flow.Index, error) {
		v, err := i.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return v.(*flow.Index), nil
	}
}

func (i *Index) Store(blockID flow.Identifier, index *flow.Index) error {
	return operation.RetryOnConflict(i.db.Update, i.storeTx(blockID, index))
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	return i.retrieveTx(blockID)(i.db.NewTransaction(false))
}
