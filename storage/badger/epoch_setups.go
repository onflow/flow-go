package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type EpochSetups struct {
	db    *badger.DB
	cache *Cache
}

// NewEpochSetups instantiates a new EpochSetups storage.
func NewEpochSetups(collector module.CacheMetrics, db *badger.DB) *EpochSetups {

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		id := key.(flow.Identifier)
		setup := val.(*flow.EpochSetup)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertEpochSetup(id, setup)))
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		id := key.(flow.Identifier)
		var setup flow.EpochSetup
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochSetup(id, &setup)(tx)
			return &setup, err
		}
	}

	es := &EpochSetups{
		db: db,
		cache: newCache(collector, metrics.ResourceEpochSetup,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return es
}

func (es *EpochSetups) StoreTx(setup *flow.EpochSetup) func(tx *transaction.Tx) error {
	return es.cache.PutTx(setup.ID(), setup)
}

func (es *EpochSetups) retrieveTx(setupID flow.Identifier) func(tx *badger.Txn) (*flow.EpochSetup, error) {
	return func(tx *badger.Txn) (*flow.EpochSetup, error) {
		val, err := es.cache.Get(setupID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.EpochSetup), nil
	}
}

func (es *EpochSetups) ByID(setupID flow.Identifier) (*flow.EpochSetup, error) {
	tx := es.db.NewTransaction(false)
	defer tx.Discard()
	return es.retrieveTx(setupID)(tx)
}
