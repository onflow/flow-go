package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees implements the collections memory pool of the consensus nodes,
// used to store collection guarantees and to generate block payloads.
type Guarantees struct {
	*Backend
}

// NewGuarantees creates a new memory pool for collection guarantees.
func NewGuarantees(limit uint) (*Guarantees, error) {
	g := &Guarantees{
		Backend: NewBackend(WithLimit(limit)),
	}

	return g, nil
}

// Add adds a collection guarantee guarantee to the mempool.
func (g *Guarantees) Add(guarantee *flow.CollectionGuarantee) bool {
	return g.Backend.Add(guarantee)
}

// ByID returns the collection guarantee with the given ID from the mempool.
func (g *Guarantees) ByID(collID flow.Identifier) (*flow.CollectionGuarantee, bool) {
	entity, exists := g.Backend.ByID(collID)
	if !exists {
		return nil, false
	}
	guarantee := entity.(*flow.CollectionGuarantee)
	return guarantee, true
}

// All returns all collection guarantees from the mempool.
func (g *Guarantees) All() []*flow.CollectionGuarantee {
	entities := g.Backend.All()
	guarantees := make([]*flow.CollectionGuarantee, 0, len(entities))
	for _, entity := range entities {
		guarantees = append(guarantees, entity.(*flow.CollectionGuarantee))
	}
	return guarantees
}
