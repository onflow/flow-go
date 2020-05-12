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
	g := &Collections{
		Backend: NewBackend(WithLimit(limit)),
	}

	return g, nil
}

// Add adds a collection to the mempool.
func (g *Collections) Add(coll *flow.Collection) bool {
	return g.Backend.Add(coll)
}

// ByID returns the collection with the given ID from the mempool.
func (g *Collections) ByID(collID flow.Identifier) (*flow.Collection, bool) {
	entity, exists := g.Backend.ByID(collID)
	if !exists {
		return nil, false
	}
	coll := entity.(*flow.Collection)
	return coll, true
}

// All returns all collections from the mempool.
func (g *Collections) All() []*flow.Collection {
	entities := g.Backend.All()
	colls := make([]*flow.Collection, 0, len(entities))
	for _, entity := range entities {
		colls = append(colls, entity.(*flow.Collection))
	}
	return colls
}
