package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type EpochStatuses struct {
	db    *badger.DB
	cache *Cache
}

// NewEpochStatuses ...
func NewEpochStatuses(collector module.CacheMetrics, db *badger.DB) *EpochStatuses {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		blockID := key.(flow.Identifier)
		status := val.(*flow.EpochStatus)
		return operation.InsertEpochStatus(blockID, status)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var status flow.EpochStatus
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochStatus(blockID, &status)(tx)
			return &status, err
		}
	}

	es := &EpochStatuses{
		db: db,
		cache: newCache(collector, metrics.ResourceEpochStatus,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return es
}

func (es *EpochStatuses) StoreTx(blockID flow.Identifier, status *flow.EpochStatus) func(tx *badger.Txn) error {
	return es.cache.Put(blockID, status)
}

func (es *EpochStatuses) retrieveTx(blockID flow.Identifier) func(tx *badger.Txn) (*flow.EpochStatus, error) {
	return func(tx *badger.Txn) (*flow.EpochStatus, error) {
		val, err := es.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.EpochStatus), nil
	}
}

func (es *EpochStatuses) ByBlockID(blockID flow.Identifier) (*flow.EpochStatus, error) {
	tx := es.db.NewTransaction(false)
	defer tx.Discard()
	return es.retrieveTx(blockID)(tx)
}
