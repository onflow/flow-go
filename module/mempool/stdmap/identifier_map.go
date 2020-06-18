package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/model"
)

// IdentifierMap represents a concurrency-safe memory pool for IdMapEntity.
type IdentifierMap struct {
	*Backend
}

// NewIdentifierMap creates a new memory pool for IdMapEntity.
func NewIdentifierMap(limit uint) (*IdentifierMap, error) {
	i := &IdentifierMap{
		Backend: NewBackend(WithLimit(limit)),
	}
	return i, nil
}

// Add will append the id to the list of identifier associated with key.
func (i *IdentifierMap) Append(key, id flow.Identifier) error {
	ids, ok := i.Get(key)
	if !ok {
		// no record with key is available in the mempool,
		// initializes ids.
		ids = make([]flow.Identifier, 0)
	} else {
		// removes map entry associated with key for update
		ok = i.Backend.Rem(key)
		if !ok {
			return fmt.Errorf("could not remove key from backend: %x", key)
		}
	}

	// appends id to the ids list
	ids = append(ids, id)

	// adds the new ids list associated with key to mempool
	mapEntity := model.IdMapEntity{
		Key: key,
		IDs: ids,
	}

	ok = i.Backend.Add(mapEntity)
	if !ok {
		return fmt.Errorf("could not add updated entity to backend, key: %x", key)
	}

	return nil
}

// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
// Otherwise it returns nil and false.
func (i *IdentifierMap) Get(key flow.Identifier) ([]flow.Identifier, bool) {
	entity, ok := i.Backend.ByID(key)
	if !ok {
		return nil, false
	}

	mapEntity, ok := entity.(model.IdMapEntity)
	if !ok {
		return nil, false
	}

	return mapEntity.IDs, true
}

// Rem removes the given key with all associated identifiers.
func (i *IdentifierMap) Rem(id flow.Identifier) bool {
	return i.Backend.Rem(id)
}
