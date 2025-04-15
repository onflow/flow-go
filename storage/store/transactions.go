package store

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Transactions ...
type Transactions struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.TransactionBody]
}

var _ storage.Transactions = &Transactions{}

// NewTransactions ...
func NewTransactions(cacheMetrics module.CacheMetrics, db storage.DB) *Transactions {
	store := func(rw storage.ReaderBatchWriter, txID flow.Identifier, flowTX *flow.TransactionBody) error {
		return operation.UpsertTransaction(rw.Writer(), txID, flowTX)
	}

	retrieve := func(r storage.Reader, txID flow.Identifier) (*flow.TransactionBody, error) {
		var flowTx flow.TransactionBody
		err := operation.RetrieveTransaction(r, txID, &flowTx)
		return &flowTx, err
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

func (t *Transactions) StoreByID(flowTxID flow.Identifier, flowTx *flow.TransactionBody) error {
	return t.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return t.storeTx(rw, flowTxID, flowTx)
	})
}

func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return t.StoreByID(flowTx.ID(), flowTx)
}

func (t *Transactions) storeTx(rw storage.ReaderBatchWriter, flowTxID flow.Identifier, flowTx *flow.TransactionBody) error {
	return t.cache.PutTx(rw, flowTxID, flowTx)
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	return t.cache.Get(t.db.Reader(), txID)
}
