package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Transactions struct {
	db    *badger.DB
	cache *Cache
}

func NewTransactions(cacheMetrics module.CacheMetrics, db *badger.DB) *Transactions {

	t := Transactions{db: db}

	store := func(txID flow.Identifier, tx interface{}) error {
		return operation.RetryOnConflict(t.db.Update, t.storeTx(tx.(*flow.TransactionBody)))
	}

	retrieve := func(txID flow.Identifier) (interface{}, error) {
		var tx flow.TransactionBody
		err := db.View(operation.RetrieveTransaction(txID, &tx))
		return &tx, err
	}

	t.cache = newCache(cacheMetrics,
		withLimit(flow.DefaultTransactionExpiry+100),
		withStore(store),
		withRetrieve(retrieve),
		withResource(metrics.ResourceTransaction))

	return &t
}

func (t *Transactions) Store(tx *flow.TransactionBody) error {
	return t.cache.Put(tx.ID(), tx)
}

func (t *Transactions) storeTx(tx *flow.TransactionBody) func(*badger.Txn) error {
	return func(btx *badger.Txn) error {
		return operation.SkipDuplicates(operation.InsertTransaction(tx))(btx)
	}
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	tx, err := t.cache.Get(txID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve transaction: %w", err)
	}
	return tx.(*flow.TransactionBody), nil
}
