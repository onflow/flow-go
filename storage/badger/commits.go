package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Commits struct {
	db    *badger.DB
	cache *Cache
}

func NewCommits(db *badger.DB) *Commits {

	store := func(blockID flow.Identifier, sealedID interface{}) error {
		return db.Update(operation.IndexSealedBlock(blockID, sealedID.(flow.Identifier)))
	}

	retrieve := func(blockID flow.Identifier) (interface{}, error) {
		var sealedID flow.Identifier
		err := db.View(operation.LookupSealedBlock(blockID, &sealedID))
		return sealedID, err
	}

	c := &Commits{
		db:    db,
		cache: newCache(withStore(store), withRetrieve(retrieve)),
	}

	return c
}

func (c *Commits) Store(blockID flow.Identifier, sealedID flow.Identifier) error {
	return c.cache.Put(blockID, sealedID)
}

func (c *Commits) ByBlockID(blockID flow.Identifier) (flow.Identifier, error) {
	sealedID, err := c.cache.Get(blockID)
	return sealedID.(flow.Identifier), err
}
