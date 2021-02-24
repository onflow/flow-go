package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type viewRange struct {
	first uint64
	last  uint64
}

type EpochSetups struct {
	db     *badger.DB
	cache  *Cache
	lookup map[uint64]*viewRange // used to track the first and last view per epoch
}

func NewEpochSetups(collector module.CacheMetrics, db *badger.DB) *EpochSetups {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		id := key.(flow.Identifier)
		setup := val.(*flow.EpochSetup)
		return operation.InsertEpochSetup(id, setup)
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
		cache: newCache(collector,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceEpochSetup)),
		lookup: make(map[uint64]*viewRange),
	}

	return es
}

func (es *EpochSetups) StoreTx(setup *flow.EpochSetup) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := es.cache.Put(setup.ID(), setup)(tx)
		if err != nil {
			return err
		}
		es.lookup[setup.Counter] = &viewRange{first: setup.FirstView, last: setup.FinalView}
		return nil
	}
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

func (es *EpochSetups) CounterByView(view uint64) (uint64, error) {
	for c, vr := range es.lookup {
		if view >= vr.first && view <= vr.last {
			return c, nil
		}
	}
	return 0, fmt.Errorf("no epoch found for view %d", view)
}
