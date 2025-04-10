package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/storage/operation"
)

type EpochSetups struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.EpochSetup]
}

// NewEpochSetups instantiates a new EpochSetups storage.
func NewEpochSetups(collector module.CacheMetrics, db storage.DB) *EpochSetups {

	store := func(rw storage.ReaderBatchWriter, id flow.Identifier, setup *flow.EpochSetup) error {
		return operation.InsertEpochSetup(rw.Writer(), id, setup)
	}

	retrieve := func(r storage.Reader, id flow.Identifier) (*flow.EpochSetup, error) {
		var setup flow.EpochSetup
		err := operation.RetrieveEpochSetup(r, id, &setup)
		return &setup, err
	}

	es := &EpochSetups{
		db: db,
		cache: newCache[flow.Identifier, *flow.EpochSetup](collector, metrics.ResourceEpochSetup,
			withLimit[flow.Identifier, *flow.EpochSetup](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return es
}

func (es *EpochSetups) StoreTx(setup *flow.EpochSetup) func(tx *transaction.Tx) error {
	panic("not implemented")
}

func (es *EpochSetups) BatchStore(rw storage.ReaderBatchWriter, setup *flow.EpochSetup) error {
	return es.cache.PutTx(rw, setup.ID(), setup)
}

func (es *EpochSetups) retrieveTx(setupID flow.Identifier) (*flow.EpochSetup, error) {
	val, err := es.cache.Get(es.db.Reader(), setupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve EpochSetup event with id %x: %w", setupID, err)
	}
	return val, nil
}

// ByID will return the EpochSetup event by its ID.
// Error returns:
// * storage.ErrNotFound if no EpochSetup with the ID exists
func (es *EpochSetups) ByID(setupID flow.Identifier) (*flow.EpochSetup, error) {
	return es.retrieveTx(setupID)
}
