package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type EpochStatuses struct {
	db    *badger.DB
	cache *Cache
}

func NewEpochStatuses(collector module.CacheMetrics, db *badger.DB) *EpochStatuses {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		id := key.(flow.Identifier)
		state := val.(*flow.EpochStatus)
		return operation.InsertEpochStatus(id, state)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		id := key.(flow.Identifier)
		var state flow.EpochStatus
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochStatus(id, &state)(tx)
			return &state, err
		}
	}

	es := &EpochStatuses{
		db: db,
		cache: newCache(collector,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceEpochState)),
	}

	return es
}

// TODO: can we remove this method? Its not contained in the interface.
func (es *EpochStatuses) Store(blockID flow.Identifier, state *flow.EpochStatus) error {
	return operation.RetryOnConflict(es.db.Update, es.StoreTx(blockID, state))
}

func (es *EpochStatuses) StoreTx(blockID flow.Identifier, state *flow.EpochStatus) func(tx *badger.Txn) error {
	return es.cache.Put(blockID, state)
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
