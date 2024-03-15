package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type EpochCommits struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.EpochCommit]
}

func NewEpochCommits(collector module.CacheMetrics, db *badger.DB) *EpochCommits {

	store := func(id flow.Identifier, commit *flow.EpochCommit) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertEpochCommit(id, commit)))
	}

	retrieve := func(id flow.Identifier) func(*badger.Txn) (*flow.EpochCommit, error) {
		return func(tx *badger.Txn) (*flow.EpochCommit, error) {
			var commit flow.EpochCommit
			err := operation.RetrieveEpochCommit(id, &commit)(tx)
			return &commit, err
		}
	}

	ec := &EpochCommits{
		db: db,
		cache: newCache[flow.Identifier, *flow.EpochCommit](collector, metrics.ResourceEpochCommit,
			withLimit[flow.Identifier, *flow.EpochCommit](4*flow.DefaultTransactionExpiry),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return ec
}

func (ec *EpochCommits) StoreTx(commit *flow.EpochCommit) func(*transaction.Tx) error {
	return ec.cache.PutTx(commit.ID(), commit)
}

func (ec *EpochCommits) retrieveTx(commitID flow.Identifier) func(tx *badger.Txn) (*flow.EpochCommit, error) {
	return func(tx *badger.Txn) (*flow.EpochCommit, error) {
		val, err := ec.cache.Get(commitID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve EpochCommit event with id %x: %w", commitID, err)
		}
		return val, nil
	}
}

// TODO: can we remove this method? Its not contained in the interface.
func (ec *EpochCommits) Store(commit *flow.EpochCommit) error {
	return operation.RetryOnConflictTx(ec.db, transaction.Update, ec.StoreTx(commit))
}

// ByID will return the EpochCommit event by its ID.
// Error returns:
// * storage.ErrNotFound if no EpochCommit with the ID exists
func (ec *EpochCommits) ByID(commitID flow.Identifier) (*flow.EpochCommit, error) {
	tx := ec.db.NewTransaction(false)
	defer tx.Discard()
	return ec.retrieveTx(commitID)(tx)
}
