package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees implements the collections memory pool of the consensus nodes,
// used to store collection guarantees and to generate block payloads.
type Guarantees struct {
	*Backend[flow.Identifier, *flow.CollectionGuarantee]
}

// NewGuarantees creates a new memory pool for collection guarantees.
func NewGuarantees(limit uint) (*Guarantees, error) {
	g := &Guarantees{
		Backend: NewBackend[flow.Identifier, *flow.CollectionGuarantee](WithLimit[flow.Identifier, *flow.CollectionGuarantee](limit)),
	}

	return g, nil
}

// Add adds a collection guarantee to the mempool.
func (g *Guarantees) Add(guarantee *flow.CollectionGuarantee) bool {
	return g.Backend.Add(guarantee.ID(), guarantee)
}

// ByID returns the collection guarantee with the given ID from the mempool.
func (g *Guarantees) ByID(collID flow.Identifier) (*flow.CollectionGuarantee, bool) {
	guarantee, exists := g.Backend.ByID(collID)
	if !exists {
		return nil, false
	}
	return guarantee, true
}

// All returns all collection guarantees from the mempool.
func (g *Guarantees) All() []*flow.CollectionGuarantee {
	entities := g.Backend.All()
	guarantees := make([]*flow.CollectionGuarantee, 0, len(entities))
	for _, guarantee := range entities {
		guarantees = append(guarantees, guarantee)
	}
	return guarantees
}
