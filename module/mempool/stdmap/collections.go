// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections implements a mempool storing collections.
type Collections struct {
	*Backend
}

// NewCollections creates a new memory pool for collection.
func NewCollections(limit uint) (*Collections, error) {
	c := &Collections{
		Backend: NewBackend(WithLimit(limit)),
	}
	return c, nil
}

// Add adds a collection to the mempool.
func (c *Collections) Add(coll *flow.Collection) bool {
	added := c.Backend.Add(coll)
	return added
}

// Rem removes a collection by ID from memory
func (c *Collections) Rem(collID flow.Identifier) bool {
	ok := c.Backend.Rem(collID)
	return ok
}

// ByID returns the collection with the given ID from the mempool.
func (c *Collections) ByID(collID flow.Identifier) (*flow.Collection, bool) {
	entity, exists := c.Backend.ByID(collID)
	if !exists {
		return nil, false
	}
	coll := entity.(*flow.Collection)
	return coll, true
}

// All returns all collections from the mempool.
func (c *Collections) All() []*flow.Collection {
	entities := c.Backend.All()
	colls := make([]*flow.Collection, 0, len(entities))
	for _, entity := range entities {
		colls = append(colls, entity.(*flow.Collection))
	}
	return colls
}
