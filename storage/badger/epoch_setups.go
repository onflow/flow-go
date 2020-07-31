package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type EpochSetups struct {
	db    *badger.DB
	cache *Cache
}

func NewEpochSetups(collector module.CacheMetrics, db *badger.DB) *EpochSetups {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		counter := key.(uint64)
		setup := val.(*epoch.Setup)
		return operation.InsertEpochSetup(counter, setup)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		counter := key.(uint64)
		var setup epoch.Setup
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochSetup(counter, &setup)(tx)
			return &setup, err
		}
	}

	es := &EpochSetups{
		db: db,
		cache: newCache(collector,
			withLimit(16),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceSeal)),
	}

	return es
}

func (es *EpochSetups) StoreTx(setup *epoch.Setup) func(tx *badger.Txn) error {
	return es.cache.Put(setup.Counter, setup)
}

func (es *EpochSetups) retrieveTx(counter uint64) func(tx *badger.Txn) (*epoch.Setup, error) {
	return func(tx *badger.Txn) (*epoch.Setup, error) {
		val, err := es.cache.Get(counter)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*epoch.Setup), nil
	}
}

func (es *EpochSetups) Store(setup *epoch.Setup) error {
	return operation.RetryOnConflict(es.db.Update, es.StoreTx(setup))
}

func (es *EpochSetups) ByCounter(counter uint64) (*epoch.Setup, error) {
	tx := es.db.NewTransaction(false)
	defer tx.Discard()
	return es.retrieveTx(counter)(tx)
}
