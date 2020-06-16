package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/model"
)

// Identifiers represents a concurrency-safe memory pool for IDs.
type IdentifierMap struct {
	*Backend
}

// NewIdentifiers creates a new memory pool for identifiers.
func NewIdentifierMap(limit uint) (*IdentifierMap, error) {
	i := &IdentifierMap{
		Backend: NewBackend(WithLimit(limit)),
	}
	return i, nil
}

// Add will add the given identifier to the memory pool or it will error if
// the identifier is already in the memory pool.
func (i *IdentifierMap) Add(key, id flow.Identifier) bool {
	// wraps ID around an ID entity to be stored in the mempool
	idEntity := &model.IdEntity{
		Id: id,
	}
	return i.Backend.Add(idEntity)
}

// Has checks whether the mempool has the identifier
func (i *IdentifierMap) Get(id flow.Identifier) ([]flow.Identifier, bool) {
	return i.Backend.Has(id)
}

// Rem removes the given identifier from the memory pool; it will
// return true if the identifier was known and removed.
func (i *IdentifierMap) Rem(id flow.Identifier) bool {
	return i.Backend.Rem(id)
}
