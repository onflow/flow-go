package store

import (
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

func NewCommits(collector module.CacheMetrics, db storage.DB) *Commits {

	store := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, commit flow.StateCommitment) error {
		return operation.IndexStateCommitment(rw.Writer(), blockID, commit)
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) (flow.StateCommitment, error) {
		var commit flow.StateCommitment
		err := operation.LookupStateCommitment(r, blockID, &commit)
		return commit, err
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

func (c *Commits) storeTx(rw storage.ReaderBatchWriter, blockID flow.Identifier, commit flow.StateCommitment) error {
	return c.cache.PutTx(rw, blockID, commit)
}

func (c *Commits) retrieveTx(r storage.Reader, blockID flow.Identifier) (flow.StateCommitment, error) {
	val, err := c.cache.Get(r, blockID)
	if err != nil {
		return flow.DummyStateCommitment, err
	}
	return val, nil
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return c.storeTx(rw, blockID, commit)
	})
}

// BatchStore stores Commit keyed by blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (c *Commits) BatchStore(blockID flow.Identifier, commit flow.StateCommitment, rw storage.ReaderBatchWriter) error {
	// we can't cache while using batches, as it's unknown at this point when, and if
	// the batch will be committed. Cache will be populated on read however.
	return operation.IndexStateCommitment(rw.Writer(), blockID, commit)
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return c.retrieveTx(c.db.Reader(), blockID)
}

func (c *Commits) RemoveByBlockID(blockID flow.Identifier) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveStateCommitment(rw.Writer(), blockID)
	})
}

// BatchRemoveByBlockID removes Commit keyed by blockID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (c *Commits) BatchRemoveByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return operation.RemoveStateCommitment(rw.Writer(), blockID)
}
