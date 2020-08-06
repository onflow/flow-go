package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type EpochCommits struct {
	db    *badger.DB
	cache *Cache
}

func NewEpochCommits(collector module.CacheMetrics, db *badger.DB) *EpochCommits {

	store := func(key interface{}, val interface{}) func(*badger.Txn) error {
		counter := key.(uint64)
		commit := val.(*flow.EpochCommit)
		return operation.InsertEpochCommit(counter, commit)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		counter := key.(uint64)
		var commit flow.EpochCommit
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochCommit(counter, &commit)(tx)
			return &commit, err
		}
	}

	ec := &EpochCommits{
		db: db,
		cache: newCache(collector,
			withLimit(16),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceSeal)),
	}

	return ec
}

func (ec *EpochCommits) StoreTx(commit *flow.EpochCommit) func(tx *badger.Txn) error {
	return ec.cache.Put(commit.Counter, commit)
}

func (ec *EpochCommits) retrieveTx(counter uint64) func(tx *badger.Txn) (*flow.EpochCommit, error) {
	return func(tx *badger.Txn) (*flow.EpochCommit, error) {
		val, err := ec.cache.Get(counter)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.EpochCommit), nil
	}
}

func (ec *EpochCommits) Store(counter uint64, commit *flow.EpochCommit) error {
	return operation.RetryOnConflict(ec.db.Update, ec.StoreTx(commit))
}

func (ec *EpochCommits) ByCounter(counter uint64) (*flow.EpochCommit, error) {
	tx := ec.db.NewTransaction(false)
	defer tx.Discard()
	return ec.retrieveTx(counter)(tx)
}
