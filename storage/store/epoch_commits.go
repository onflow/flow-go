package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type EpochCommits struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.EpochCommit]
}

func NewEpochCommits(collector module.CacheMetrics, db storage.DB) *EpochCommits {

	store := func(rw storage.ReaderBatchWriter, id flow.Identifier, commit *flow.EpochCommit) error {
		return operation.InsertEpochCommit(rw.Writer(), id, commit)
	}

	retrieve := func(r storage.Reader, id flow.Identifier) (*flow.EpochCommit, error) {
		var commit flow.EpochCommit
		err := operation.RetrieveEpochCommit(r, id, &commit)
		return &commit, err
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

func (ec *EpochCommits) BatchStore(rw storage.ReaderBatchWriter, commit *flow.EpochCommit) error {
	return ec.cache.PutTx(rw, commit.ID(), commit)
}

func (ec *EpochCommits) retrieveTx(commitID flow.Identifier) (*flow.EpochCommit, error) {
	val, err := ec.cache.Get(ec.db.Reader(), commitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve EpochCommit event with id %x: %w", commitID, err)
	}
	return val, nil
}

// ByID will return the EpochCommit event by its ID.
// Error returns:
// * storage.ErrNotFound if no EpochCommit with the ID exists
func (ec *EpochCommits) ByID(commitID flow.Identifier) (*flow.EpochCommit, error) {
	return ec.retrieveTx(commitID)
}
