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
	cache *Cache[flow.Identifier, *flow.Index]
}

func NewIndex(collector module.CacheMetrics, db *badger.DB) *Index {

	store := func(blockID flow.Identifier, index *flow.Index) func(*transaction.Tx) error {
		return transaction.WithTx(procedure.InsertIndex(blockID, index))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (*flow.Index, error) {
		var index flow.Index
		return func(tx *badger.Txn) (*flow.Index, error) {
			err := procedure.RetrieveIndex(blockID, &index)(tx)
			return &index, err
		}
	}

	p := &Index{
		db: db,
		cache: newCache[flow.Identifier, *flow.Index](collector, metrics.ResourceIndex,
			withLimit[flow.Identifier, *flow.Index](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return p
}

func (i *Index) storeTx(blockID flow.Identifier, index *flow.Index) func(*transaction.Tx) error {
	return i.cache.PutTx(blockID, index)
}

func (i *Index) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Index, error) {
	return func(tx *badger.Txn) (*flow.Index, error) {
		val, err := i.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (i *Index) Store(blockID flow.Identifier, index *flow.Index) error {
	return operation.RetryOnConflictTx(i.db, transaction.Update, i.storeTx(blockID, index))
}

func (i *Index) ByBlockID(blockID flow.Identifier) (*flow.Index, error) {
	tx := i.db.NewTransaction(false)
	defer tx.Discard()
	return i.retrieveTx(blockID)(tx)
}
