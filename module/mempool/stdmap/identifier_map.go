package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/model"
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
	return i.Backend.Run(func(backdata mempool.BackData) error {
		var ids map[flow.Identifier]struct{}
		entity, ok := backdata.ByID(key)
		if !ok {
			// no record with key is available in the mempool,
			// initializes ids.
			ids = make(map[flow.Identifier]struct{})
		} else {
			idMapEntity, ok := entity.(model.IdMapEntity)
			if !ok {
				return fmt.Errorf("could not assert entity to IdMapEntity")
			}

			ids = idMapEntity.IDs
			if _, ok := ids[id]; ok {
				// id is already associated with the key
				// no need to append
				return nil
			}

			// removes map entry associated with key for update
			if _, removed := backdata.Rem(key); !removed {
				return fmt.Errorf("potential race condition on removing from identifier map")
			}
		}

		// appends id to the ids list
		ids[id] = struct{}{}

		// adds the new ids list associated with key to mempool
		idMapEntity := model.IdMapEntity{
			Key: key,
			IDs: ids,
		}

		if added := backdata.Add(key, idMapEntity); !added {
			return fmt.Errorf("potential race condition on adding to identifier map")
		}

		return nil
	})
}

// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
// Otherwise it returns nil and false.
func (i *IdentifierMap) Get(key flow.Identifier) ([]flow.Identifier, bool) {
	ids := make([]flow.Identifier, 0)
	err := i.Run(func(backdata mempool.BackData) error {
		entity, ok := backdata.ByID(key)
		if !ok {
			return fmt.Errorf("could not retrieve key from backend")
		}

		mapEntity, ok := entity.(model.IdMapEntity)
		if !ok {
			return fmt.Errorf("could not assert entity as IdMapEntity")
		}

		for id := range mapEntity.IDs {
			ids = append(ids, id)
		}

		return nil
	})

	if err != nil {
		return nil, false
	}
	return ids, true
}

// Has returns true if the key exists in the map, i.e., there is at least an id
// attached to it.
func (i *IdentifierMap) Has(key flow.Identifier) bool {
	return i.Backend.Has(key)
}

// Rem removes the given key with all associated identifiers.
func (i *IdentifierMap) Rem(key flow.Identifier) bool {
	return i.Backend.Rem(key)
}

// RemIdFromKey removes the id from the list of identifiers associated with key.
// If the list becomes empty, it also removes the key from the map.
func (i *IdentifierMap) RemIdFromKey(key, id flow.Identifier) error {
	err := i.Backend.Run(func(backdata mempool.BackData) error {
		// var ids map[flow.Identifier]struct{}
		entity, ok := backdata.ByID(key)
		if !ok {
			// entity key has already been removed
			return nil
		}

		idMapEntity, ok := entity.(model.IdMapEntity)
		if !ok {
			return fmt.Errorf("could not assert entity to IdMapEntity")
		}

		if _, ok := idMapEntity.IDs[id]; !ok {
			// id has already been removed from the key map
			return nil
		}

		// removes map entry associated with key for update
		if _, removed := backdata.Rem(key); !removed {
			return fmt.Errorf("potential race condition on removing from identifier map")
		}

		// removes id from the secondary map of the key
		delete(idMapEntity.IDs, id)

		if len(idMapEntity.IDs) == 0 {
			// all ids related to key are removed, so there is no need
			// to add key back to the idMapEntity
			return nil
		}

		// adds the new ids list associated with key to mempool
		idMapEntity = model.IdMapEntity{
			Key: key,
			IDs: idMapEntity.IDs,
		}

		if added := backdata.Add(key, idMapEntity); !added {
			return fmt.Errorf("potential race condition on adding to identifier map")
		}

		return nil
	})

	return err
}

// Size returns number of IdMapEntities in mempool
func (i *IdentifierMap) Size() uint {
	return i.Backend.Size()
}

// Keys returns a list of all keys in the mempool
func (i *IdentifierMap) Keys() ([]flow.Identifier, bool) {
	entities := i.Backend.All()
	keys := make([]flow.Identifier, 0)
	for _, entity := range entities {
		idMapEntity, ok := entity.(model.IdMapEntity)
		if !ok {
			return nil, false
		}
		keys = append(keys, idMapEntity.Key)
	}
	return keys, true
}
