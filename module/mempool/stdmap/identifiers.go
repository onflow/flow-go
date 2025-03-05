package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Identifiers represents a concurrency-safe memory pool for IDs.
type Identifiers struct {
	*Backend[flow.Identifier, struct{}]
}

// NewIdentifiers creates a new memory pool for identifiers.
func NewIdentifiers(limit uint) (*Identifiers, error) {
	i := &Identifiers{
		Backend: NewBackend(WithLimit[flow.Identifier, struct{}](limit)),
	}

	return i, nil
}

// Add will add the given identifier to the memory pool or it will error if
// the identifier is already in the memory pool.
func (i *Identifiers) Add(id flow.Identifier) bool {
	return i.Backend.Add(id, struct{}{})
}

// Has checks whether the mempool has the identifier
func (i *Identifiers) Has(id flow.Identifier) bool {
	return i.Backend.Has(id)
}

// Remove removes the given identifier from the memory pool; it will
// return true if the identifier was known and removed.
func (i *Identifiers) Remove(id flow.Identifier) bool {
	return i.Backend.Remove(id)
}

// All returns all identifiers stored in the mempool
func (i *Identifiers) All() flow.IdentifierList {
	all := i.Backend.All()
	idEntities := make([]flow.Identifier, 0, len(all))
	for key, _ := range all {
		idEntities = append(idEntities, key)
	}
	return idEntities
}
