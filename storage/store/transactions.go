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
	db      storage.DB
	readers []storage.Reader
	cache   *Cache[flow.Identifier, *flow.TransactionBody]
}

var _ storage.Transactions = (*Transactions)(nil)

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

	remove := func(rw storage.ReaderBatchWriter, txID flow.Identifier) error {
		return operation.RemoveTransaction(rw.Writer(), txID)
	}

	t := &Transactions{
		db:      db,
		readers: []storage.Reader{db.Reader()},
		cache: newCache(cacheMetrics, metrics.ResourceTransaction,
			withLimit[flow.Identifier, *flow.TransactionBody](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRemove[flow.Identifier, *flow.TransactionBody](remove),
			withRetrieve(retrieve),
		),
	}

	return t
}

func (t *Transactions) AddReader(reader storage.Reader) {
	t.readers = append(t.readers, reader)
}

func (t *Transactions) Store(flowTx *flow.TransactionBody) error {
	return t.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return t.storeTx(rw, flowTx)
	})
}

func (t *Transactions) storeTx(rw storage.ReaderBatchWriter, flowTx *flow.TransactionBody) error {
	return t.cache.PutTx(rw, flowTx.ID(), flowTx)
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	return t.cache.Get(t.reader(), txID)
}

// RemoveBatch removes a transaction by fingerprint.
func (t *Transactions) RemoveBatch(rw storage.ReaderBatchWriter, txID flow.Identifier) error {
	return t.cache.RemoveTx(rw, txID)
}

func (t *Transactions) reader() storage.Reader {
	return operation.NewMultiReader(t.readers...)
}
