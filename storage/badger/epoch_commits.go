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
		id := key.(flow.Identifier)
		commit := val.(*flow.EpochCommit)
		return operation.InsertEpochCommit(id, commit)
	}

	retrieve := func(key interface{}) func(*badger.Txn) (interface{}, error) {
		id := key.(flow.Identifier)
		var commit flow.EpochCommit
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveEpochCommit(id, &commit)(tx)
			return &commit, err
		}
	}

	ec := &EpochCommits{
		db: db,
		cache: newCache(collector,
			withLimit(4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceEpochCommit)),
	}

	return ec
}

func (ec *EpochCommits) StoreTx(commit *flow.EpochCommit) func(tx *badger.Txn) error {
	return ec.cache.Put(commit.ID(), commit)
}

func (ec *EpochCommits) retrieveTx(commitID flow.Identifier) func(tx *badger.Txn) (*flow.EpochCommit, error) {
	return func(tx *badger.Txn) (*flow.EpochCommit, error) {
		val, err := ec.cache.Get(commitID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.EpochCommit), nil
	}
}

// TODO: can we remove this method? Its not contained in the interface.
func (ec *EpochCommits) Store(commit *flow.EpochCommit) error {
	return operation.RetryOnConflict(ec.db.Update, ec.StoreTx(commit))
}

func (ec *EpochCommits) ByID(commitID flow.Identifier) (*flow.EpochCommit, error) {
	tx := ec.db.NewTransaction(false)
	defer tx.Discard()
	return ec.retrieveTx(commitID)(tx)
}
