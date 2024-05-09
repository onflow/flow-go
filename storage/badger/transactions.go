package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Transactions ...
type Transactions struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.TransactionBody]
}

// NewTransactions ...
func NewTransactions(cacheMetrics module.CacheMetrics, db *badger.DB) *Transactions {
	store := func(txID flow.Identifier, flowTX *flow.TransactionBody) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertTransaction(txID, flowTX)))
	}

	retrieve := func(txID flow.Identifier) func(tx *badger.Txn) (*flow.TransactionBody, error) {
		return func(tx *badger.Txn) (*flow.TransactionBody, error) {
			var flowTx flow.TransactionBody
			err := operation.RetrieveTransaction(txID, &flowTx)(tx)
			return &flowTx, err
		}
	}

	t := &Transactions{
		db: db,
		cache: newCache[flow.Identifier, *flow.TransactionBody](cacheMetrics, metrics.ResourceTransaction,
			withLimit[flow.Identifier, *flow.TransactionBody](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return t
}

// Store ...
func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return operation.RetryOnConflictTx(t.db, transaction.Update, t.storeTx(flowTx))
}

// ByID ...
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	return t.retrieveTx(txID)(tx)
}

func (t *Transactions) storeTx(flowTx *flow.TransactionBody) func(*transaction.Tx) error {
	return t.cache.PutTx(flowTx.ID(), flowTx)
}

func (t *Transactions) retrieveTx(txID flow.Identifier) func(*badger.Txn) (*flow.TransactionBody, error) {
	return func(tx *badger.Txn) (*flow.TransactionBody, error) {
		val, err := t.cache.Get(txID)(tx)
		if err != nil {
			return nil, err
		}
		return val, err
	}
}
