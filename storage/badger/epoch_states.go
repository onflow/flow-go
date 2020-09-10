package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type EpochStates struct {
	db    *badger.DB
	cache *Cache
}

func NewEpochStates(collector module.CacheMetrics, db *badger.DB) *EpochStates {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		id := key.(flow.Identifier)
		state := val.(*flow.EpochState)
		return operation.InsertEpochState(id, state)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		id := key.(flow.Identifier)
		var state flow.EpochState
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochState(id, &state)(tx)
			return &state, err
		}
	}

	es := &EpochStates{
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
func (es *EpochStates) Store(blockID flow.Identifier, state *flow.EpochState) error {
	return operation.RetryOnConflict(es.db.Update, es.StoreTx(blockID, state))
}

func (es *EpochStates) StoreTx(blockID flow.Identifier, state *flow.EpochState) func(tx *badger.Txn) error {
	return es.cache.Put(blockID, state)
}

func (es *EpochStates) retrieveTx(blockID flow.Identifier) func(tx *badger.Txn) (*flow.EpochState, error) {
	return func(tx *badger.Txn) (*flow.EpochState, error) {
		val, err := es.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.EpochState), nil
	}
}

func (es *EpochStates) ByBlockID(blockID flow.Identifier) (*flow.EpochState, error) {
	tx := es.db.NewTransaction(false)
	defer tx.Discard()
	return es.retrieveTx(blockID)(tx)
}
