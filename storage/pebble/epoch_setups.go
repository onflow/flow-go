package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type EpochSetups struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.EpochSetup]
}

// NewEpochSetups instantiates a new EpochSetups storage.
func NewEpochSetups(collector module.CacheMetrics, db *pebble.DB) *EpochSetups {

	store := func(id flow.Identifier, setup *flow.EpochSetup) func(operation.PebbleReaderWriter) error {
		return operation.OnlyWrite(operation.InsertEpochSetup(id, setup))
	}

	retrieve := func(id flow.Identifier) func(pebble.Reader) (*flow.EpochSetup, error) {
		return func(tx pebble.Reader) (*flow.EpochSetup, error) {
			var setup flow.EpochSetup
			err := operation.RetrieveEpochSetup(id, &setup)(tx)
			return &setup, err
		}
	}

	es := &EpochSetups{
		db: db,
		cache: newCache(collector, metrics.ResourceEpochSetup,
			withLimit[flow.Identifier, *flow.EpochSetup](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return es
}

func (es *EpochSetups) StoreTx(setup *flow.EpochSetup) func(operation.PebbleReaderWriter) error {
	return es.cache.PutTx(setup.ID(), setup)
}

func (es *EpochSetups) retrieveTx(setupID flow.Identifier) func(tx pebble.Reader) (*flow.EpochSetup, error) {
	return func(tx pebble.Reader) (*flow.EpochSetup, error) {
		val, err := es.cache.Get(setupID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// ByID will return the EpochSetup event by its ID.
// Error returns:
// * storage.ErrNotFound if no EpochSetup with the ID exists
func (es *EpochSetups) ByID(setupID flow.Identifier) (*flow.EpochSetup, error) {
	return es.retrieveTx(setupID)(es.db)
}
