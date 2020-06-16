package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Commits struct {
	db    *badger.DB
	cache *Cache
}

func NewCommits(collector module.CacheMetrics, db *badger.DB) *Commits {

	store := func(blockID flow.Identifier, v interface{}) func(tx *badger.Txn) error {
		commit := v.(flow.StateCommitment)
		return operation.SkipDuplicates(operation.IndexStateCommitment(blockID, commit))
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (interface{}, error) {
		var commit flow.StateCommitment
		return func(tx *badger.Txn) (interface{}, error) {
			err := db.View(operation.LookupStateCommitment(blockID, &commit))
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
		v, err := c.cache.Get(blockID)(tx)
		if err != nil {
			return nil, err
		}
		return v.(flow.StateCommitment), nil
	}
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return operation.RetryOnConflict(c.db.Update, c.storeTx(blockID, commit))
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	tx := c.db.NewTransaction(false)
	defer tx.Discard()
	return c.retrieveTx(blockID)(tx)
}
