package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Commits struct {
	db    *badger.DB
	cache *Cache
}

func NewCommits(collector module.CacheMetrics, db *badger.DB) *Commits {

	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		blockID := key.(flow.Identifier)
		commit := val.(flow.StateCommitment)
		return operation.SkipDuplicates(operation.IndexStateCommitment(blockID, commit))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)
		var commit flow.StateCommitment
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.LookupStateCommitment(blockID, &commit)(tx)
			return commit, err
		}
	}

	c := &Commits{
		db: db,
		cache: newCache(collector,
			withLimit(100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceCommit),
		),
	}

	return c
}

func (c *Commits) storeTx(blockID flow.Identifier, commit flow.StateCommitment) func(tx *badger.Txn) error {
	return c.cache.Put(blockID, commit)
}

func (c *Commits) retrieveTx(blockID flow.Identifier) func(tx *badger.Txn) (flow.StateCommitment, error) {
	return func(tx *badger.Txn) (flow.StateCommitment, error) {
		val, err := c.cache.Get(blockID)(tx)
		if err != nil {
			return flow.DummyStateCommitment, err
		}
		return val.(flow.StateCommitment), nil
	}
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return operation.RetryOnConflict(c.db.Update, c.storeTx(blockID, commit))
}

func (c *Commits) BatchStore(blockID flow.Identifier, commit flow.StateCommitment, batch storage.BatchStorage) error {
	// we can't cache while using batches, as it's unknown at this point when, and if
	// the batch will be committed. Cache will be populated on read however.
	writeBatch := batch.GetWriter()
	return operation.BatchIndexStateCommitment(blockID, commit)(writeBatch)
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	tx := c.db.NewTransaction(false)
	defer tx.Discard()
	return c.retrieveTx(blockID)(tx)
}
