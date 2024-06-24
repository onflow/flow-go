package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type Commits struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, flow.StateCommitment]
}

var _ storage.Commits = (*Commits)(nil)

func NewCommits(collector module.CacheMetrics, db *pebble.DB) *Commits {

	store := func(blockID flow.Identifier, commit flow.StateCommitment) func(rw pebble.Writer) error {
		return operation.IndexStateCommitment(blockID, commit)
	}

	retrieve := func(blockID flow.Identifier) func(tx pebble.Reader) (flow.StateCommitment, error) {
		return func(tx pebble.Reader) (flow.StateCommitment, error) {
			var commit flow.StateCommitment
			err := operation.LookupStateCommitment(blockID, &commit)(tx)
			return commit, err
		}
	}

	c := &Commits{
		db: db,
		cache: newCache(collector, metrics.ResourceCommit,
			withLimit[flow.Identifier, flow.StateCommitment](1000),
			withStore(store),
			withRetrieve(retrieve),
		),
	}

	return c
}

func (c *Commits) retrieveTx(blockID flow.Identifier) func(tx pebble.Reader) (flow.StateCommitment, error) {
	return func(tx pebble.Reader) (flow.StateCommitment, error) {
		val, err := c.cache.Get(blockID)(tx)
		if err != nil {
			return flow.DummyStateCommitment, err
		}
		return val, nil
	}
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return c.cache.PutTx(blockID, commit)(c.db)
}

// BatchStore stores Commit keyed by blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If pebble unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (c *Commits) BatchStore(blockID flow.Identifier, commit flow.StateCommitment, batch storage.BatchStorage) error {
	// we can't cache while using batches, as it's unknown at this point when, and if
	// the batch will be committed. Cache will be populated on read however.
	writeBatch := batch.GetWriter()
	return operation.IndexStateCommitment(blockID, commit)(operation.NewBatchWriter(writeBatch))
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return c.retrieveTx(blockID)(c.db)
}

func (c *Commits) RemoveByBlockID(blockID flow.Identifier) error {
	return operation.RemoveStateCommitment(blockID)(c.db)
}

// BatchRemoveByBlockID removes Commit keyed by blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If pebble unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (c *Commits) BatchRemoveByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return operation.RemoveStateCommitment(blockID)(operation.NewBatchWriter(writeBatch))
}
