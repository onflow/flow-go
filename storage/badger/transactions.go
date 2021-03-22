package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// Transactions ...
type Transactions struct {
	db    *badger.DB
	cache *Cache
}

// NewTransactions ...
func NewTransactions(cacheMetrics module.CacheMetrics, db *badger.DB) *Transactions {
	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		txID := key.(flow.Identifier)
		flowTx := val.(*flow.TransactionBody)
		return operation.SkipDuplicates(operation.InsertTransaction(txID, flowTx))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		txID := key.(flow.Identifier)
		var flowTx flow.TransactionBody
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveTransaction(txID, &flowTx)(tx)
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

// Store ...
func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return operation.RetryOnConflict(t.db.Update, t.storeTx(flowTx))
}

// ByID ...
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	return t.retrieveTx(txID)(tx)
}

func (t *Transactions) storeTx(flowTx *flow.TransactionBody) func(*badger.Txn) error {
	return t.cache.Put(flowTx.ID(), flowTx)
}

func (t *Transactions) retrieveTx(txID flow.Identifier) func(*badger.Txn) (*flow.TransactionBody, error) {
	return func(tx *badger.Txn) (*flow.TransactionBody, error) {
		val, err := t.cache.Get(txID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.TransactionBody), err
	}
}
