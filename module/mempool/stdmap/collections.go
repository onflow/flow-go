package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Collections implements a mempool storing collections.
type Collections struct {
	*Backend[flow.Identifier, *flow.Collection]
}

// NewCollections creates a new memory pool for collection.
func NewCollections(limit uint) (*Collections, error) {
	c := &Collections{
		Backend: NewBackend[flow.Identifier, *flow.Collection](WithLimit[flow.Identifier, *flow.Collection](limit)),
	}
	return c, nil
}

// Add adds a collection to the mempool.
func (c *Collections) Add(coll *flow.Collection) bool {
	added := c.Backend.Add(coll.ID(), coll)
	return added
}

// Remove removes a collection by ID from memory
func (c *Collections) Remove(collID flow.Identifier) bool {
	ok := c.Backend.Remove(collID)
	return ok
}

// ByID returns the collection with the given ID from the mempool.
func (c *Collections) ByID(collID flow.Identifier) (*flow.Collection, bool) {
	coll, exists := c.Backend.ByID(collID)
	if !exists {
		return nil, false
	}
	return coll, true
}

// All returns all collections from the mempool.
func (c *Collections) All() []*flow.Collection {
	entities := c.Backend.All()
	colls := make([]*flow.Collection, 0, len(entities))
	for _, coll := range entities {
		colls = append(colls, coll)
	}
	return colls
}
