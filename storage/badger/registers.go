package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type Registers struct {
	db    *badger.DB
	cache *Cache[flow.RegisterID, *flow.RegisterEntry]
}

var _ storage.Registers = &Registers{}

func NewRegisters(cacheMetrics module.CacheMetrics, db *badger.DB) *Registers {
	store := func(ID flow.RegisterID, entry *flow.RegisterEntry) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertRegister(ID, entry)))
	}

	retrieve := func(ID flow.RegisterID) func(tx *badger.Txn) (*flow.RegisterEntry, error) {
		return func(tx *badger.Txn) (*flow.RegisterEntry, error) {
			var entry flow.RegisterEntry
			err := operation.RetrieveRegister(ID, &entry)(tx)
			return &entry, err
		}
	}

	return &Registers{
		db: db,
		cache: newCache[flow.RegisterID, *flow.RegisterEntry](cacheMetrics, metrics.ResourceTransaction,
			withLimit[flow.RegisterID, *flow.RegisterEntry](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

func (r *Registers) Store(ID flow.RegisterID, entry *flow.RegisterEntry) error {
	return operation.RetryOnConflictTx(r.db, transaction.Update, r.cache.PutTx(ID, entry))
}

func (r *Registers) ByID(ID flow.RegisterID) (*flow.RegisterEntry, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.retrieveRegister(ID)(tx)
}

func (r *Registers) retrieveRegister(ID flow.RegisterID) func(tx *badger.Txn) (*flow.RegisterEntry, error) {
	return func(tx *badger.Txn) (*flow.RegisterEntry, error) {
		val, err := r.cache.Get(ID)(tx)
		if err != nil {
			return nil, err
		}

		return val, nil
	}
}
