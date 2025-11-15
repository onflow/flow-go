package store

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Commits struct {
	db    storage.DB
	cache *Cache[flow.Identifier, flow.StateCommitment]
}

var _ storage.Commits = (*Commits)(nil)

func NewCommits(collector module.CacheMetrics, db storage.DB) *Commits {

	retrieve := func(r storage.Reader, blockID flow.Identifier) (flow.StateCommitment, error) {
		var commit flow.StateCommitment
		err := operation.LookupStateCommitment(r, blockID, &commit)
		return commit, err
	}

	remove := func(rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
		return operation.RemoveStateCommitment(rw.Writer(), blockID)
	}

	c := &Commits{
		db: db,
		cache: newCache(collector, metrics.ResourceCommit,
			withLimit[flow.Identifier, flow.StateCommitment](1000),
			withRetrieve(retrieve),
			withRemove[flow.Identifier, flow.StateCommitment](remove),
		),
	}

	return c
}

func (c *Commits) retrieveTx(r storage.Reader, blockID flow.Identifier) (flow.StateCommitment, error) {
	val, err := c.cache.Get(r, blockID)
	if err != nil {
		return flow.DummyStateCommitment, err
	}
	return val, nil
}

// BatchStore stores a state commitment keyed by the blockID whose execution results in that state.
// The function ensures data integrity by first checking if a commitment already exists for the given block
// and rejecting overwrites with different values. This function is idempotent, i.e. repeated calls with the
// *initially* indexed value are no-ops.
//
// CAUTION:
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     The caller must acquire the [storage.LockIndexStateCommitment] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a *different* state commitment is already indexed for the same block ID
func (c *Commits) BatchStore(lctx lockctx.Proof, blockID flow.Identifier, commit flow.StateCommitment, rw storage.ReaderBatchWriter) error {
	// we can't cache while using batches, as it's unknown at this point when, and if
	// the batch will be committed. Cache will be populated on read however.
	return operation.IndexStateCommitment(lctx, rw, blockID, commit)
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return c.retrieveTx(c.db.Reader(), blockID)
}

func (c *Commits) RemoveByBlockID(blockID flow.Identifier) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return c.BatchRemoveByBlockID(blockID, rw)
	})
}

// BatchRemoveByBlockID removes Commit keyed by blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If the database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (c *Commits) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return c.cache.RemoveTx(rw, blockID)
}
