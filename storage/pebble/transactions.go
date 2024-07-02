package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// Transactions ...
type Transactions struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.TransactionBody]
}

// NewTransactions ...
func NewTransactions(cacheMetrics module.CacheMetrics, db *pebble.DB) *Transactions {
	store := func(txID flow.Identifier, flowTX *flow.TransactionBody) func(pebble.Writer) error {
		return operation.InsertTransaction(txID, flowTX)
	}

	retrieve := func(txID flow.Identifier) func(tx pebble.Reader) (*flow.TransactionBody, error) {
		return func(tx pebble.Reader) (*flow.TransactionBody, error) {
			var flowTx flow.TransactionBody
			err := operation.RetrieveTransaction(txID, &flowTx)(tx)
			return &flowTx, err
		}
	}

	t := &Transactions{
		db: db,
		cache: newCache(cacheMetrics, metrics.ResourceTransaction,
			withLimit[flow.Identifier, *flow.TransactionBody](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return t
}

// Store ...
func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return t.storeTx(flowTx)(t.db)
}

// ByID ...
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	return t.retrieveTx(txID)(t.db)
}

func (t *Transactions) storeTx(flowTx *flow.TransactionBody) func(pebble.Writer) error {
	return t.cache.PutTx(flowTx.ID(), flowTx)
}

func (t *Transactions) retrieveTx(txID flow.Identifier) func(pebble.Reader) (*flow.TransactionBody, error) {
	return func(tx pebble.Reader) (*flow.TransactionBody, error) {
		val, err := t.cache.Get(txID)(tx)
		if err != nil {
			return nil, err
		}
		return val, err
	}
}
