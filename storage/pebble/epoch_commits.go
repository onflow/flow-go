package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type EpochCommits struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.EpochCommit]
}

func NewEpochCommits(collector module.CacheMetrics, db *pebble.DB) *EpochCommits {

	store := func(id flow.Identifier, commit *flow.EpochCommit) func(operation.PebbleReaderWriter) error {
		return operation.OnlyWrite(operation.InsertEpochCommit(id, commit))
	}

	retrieve := func(id flow.Identifier) func(pebble.Reader) (*flow.EpochCommit, error) {
		return func(tx pebble.Reader) (*flow.EpochCommit, error) {
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

func (ec *EpochCommits) StoreTx(commit *flow.EpochCommit) func(operation.PebbleReaderWriter) error {
	return ec.cache.PutTx(commit.ID(), commit)
}

func (ec *EpochCommits) retrieveTx(commitID flow.Identifier) func(tx pebble.Reader) (*flow.EpochCommit, error) {
	return func(tx pebble.Reader) (*flow.EpochCommit, error) {
		val, err := ec.cache.Get(commitID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve EpochCommit event with id %x: %w", commitID, err)
		}
		return val, nil
	}
}

// ByID will return the EpochCommit event by its ID.
// Error returns:
// * storage.ErrNotFound if no EpochCommit with the ID exists
func (ec *EpochCommits) ByID(commitID flow.Identifier) (*flow.EpochCommit, error) {
	return ec.retrieveTx(commitID)(ec.db)
}
