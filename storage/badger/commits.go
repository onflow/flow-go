package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Commits struct {
	db    *badger.DB
	cache *Cache
}

func NewCommits(collector module.CacheMetrics, db *badger.DB) *Commits {

	store := func(blockID flow.Identifier, commit interface{}) error {
		return db.Update(operation.IndexStateCommitment(blockID, commit.(flow.StateCommitment)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var commit flow.StateCommitment
		err := db.View(operation.LookupStateCommitment(blockID, &commit))
		return commit, err
	}

	c := &Commits{
		db:    db,
		cache: newCache(collector, withStore(store), withRetrieve(retrieve)),
	}

	return c
}

func (c *Commits) Store(blockID flow.Identifier, commit flow.StateCommitment) error {
	return c.cache.Put(blockID, commit)
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	commit, err := c.cache.Get(blockID)
	if err != nil {
		return nil, err
	}
	return commit.(flow.StateCommitment), nil
}
