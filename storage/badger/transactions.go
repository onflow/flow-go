package badger

import (
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

	store := func(txID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		flowTx := v.(*flow.TransactionBody)
		return operation.SkipDuplicates(operation.InsertTransaction(txID, flowTx))
	}

	retrieve := func(txID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
		var flowTx flow.TransactionBody
		return func(tx *badger.Txn) (interface{}, error) {
			err := db.View(operation.RetrieveTransaction(txID, &flowTx))
			return &flowTx, err
		}
	}

	t := &Transactions{
		db: db,
		cache: newCache(cacheMetrics,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceTransaction)),
	}

	return t
}

func (t *Transactions) storeTx(flowTx *flow.TransactionBody) func(*badger.Txn) error {
	return t.cache.Put(flowTx.ID(), flowTx)
}

func (t *Transactions) retrieveTx(txID flow.Identifier) func(*badger.Txn) (*flow.TransactionBody, error) {
	return func(tx *badger.Txn) (*flow.TransactionBody, error) {
		v, err := t.cache.Get(txID)(tx)
		if err != nil {
			return nil, err
		}
		return v.(*flow.TransactionBody), err
	}
}

func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return operation.RetryOnConflict(t.db.Update, t.storeTx(flowTx))
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	return t.retrieveTx(txID)(t.db.NewTransaction(false))
}
