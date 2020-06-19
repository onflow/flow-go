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

// Append will append the id to the list of identifiers associated with key.
func (i *IdentifierMap) Append(key, id flow.Identifier) error {
	return i.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		var ids map[flow.Identifier]struct{}
		entity, ok := backdata[key]
		if !ok {
			// no record with key is available in the mempool,
			// initializes ids.
			ids = make(map[flow.Identifier]struct{})
		} else {
			idMapEntity, ok := entity.(model.IdMapEntity)
			if !ok {
				return fmt.Errorf("could not assert entity to IdMapEntity")
			}
			// removes map entry associated with key for update
			delete(backdata, key)

			ids = idMapEntity.IDs
		}

		// appends id to the ids list
		ids[id] = struct{}{}

		// adds the new ids list associated with key to mempool
		idMapEntity := model.IdMapEntity{
			Key: key,
			IDs: ids,
		}

		backdata[key] = idMapEntity

		return nil
	})
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

	ids := make([]flow.Identifier, len(mapEntity.IDs))
	for id := range mapEntity.IDs {
		ids = append(ids, id)
	}

	return ids, true
}

// Rem removes the given key with all associated identifiers.
func (i *IdentifierMap) Rem(id flow.Identifier) bool {
	return i.Backend.Rem(id)
}
