package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// IdentifierMap represents a concurrency-safe memory pool for a list of other identifiers.
type IdentifierMap struct {
	*Backend[flow.Identifier, map[flow.Identifier]struct{}]
}

// NewIdentifierMap creates a new memory pool for a list of identifiers.
func NewIdentifierMap(limit uint) (*IdentifierMap, error) {
	i := &IdentifierMap{
		Backend: NewBackend[flow.Identifier, map[flow.Identifier]struct{}](
			WithLimit[flow.Identifier, map[flow.Identifier]struct{}](limit),
		),
	}
	return i, nil
}

// Append will append the id to the list of identifiers associated with key.
func (i *IdentifierMap) Append(key, id flow.Identifier) error {
	return i.Backend.Run(func(backdata mempool.BackData[flow.Identifier, map[flow.Identifier]struct{}]) error {
		idsMap, ok := backdata.Get(key)
		if !ok {
			// no record with key is available in the mempool,
			// initializes ids.
			idsMap = make(map[flow.Identifier]struct{})
		} else {
			if _, ok := idsMap[id]; ok {
				// id is already associated with the key
				// no need to append
				return nil
			}

			// removes map entry associated with key for update
			if _, removed := backdata.Remove(key); !removed {
				return fmt.Errorf("potential race condition on removing from identifier map")
			}
		}

		// appends id to the ids list
		idsMap[id] = struct{}{}

		if added := backdata.Add(key, idsMap); !added {
			return fmt.Errorf("potential race condition on adding to identifier map")
		}

		return nil
	})
}

// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
// Otherwise it returns nil and false.
func (i *IdentifierMap) Get(key flow.Identifier) (flow.IdentifierList, bool) {
	ids := make(flow.IdentifierList, 0)
	// we need to perform a blocking operation since we are dealing with a reference object. Since our mempool
	// changes the map itself in other operations we need to ensure that all changes to the map are done in mutually
	// exclusive way, otherwise we risk observing a race condition. This is exactly why we perform `Get` and
	// transformation  of the map in critical section since if the goroutine will be suspended in between those two operations
	// we are potentially concurrently accessing the map.
	err := i.Run(func(backdata mempool.BackData[flow.Identifier, map[flow.Identifier]struct{}]) error {
		idsMap, ok := backdata.Get(key)
		if !ok {
			return fmt.Errorf("could not retrieve key from backend")
		}

		for id, _ := range idsMap {
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

// Remove removes the given key with all associated identifiers.
func (i *IdentifierMap) Remove(key flow.Identifier) bool {
	return i.Backend.Remove(key)
}

// RemoveIdFromKey removes the id from the list of identifiers associated with key.
// If the list becomes empty, it also removes the key from the map.
func (i *IdentifierMap) RemoveIdFromKey(key, id flow.Identifier) error {
	err := i.Backend.Run(func(backdata mempool.BackData[flow.Identifier, map[flow.Identifier]struct{}]) error {
		idsMap, ok := backdata.Get(key)
		if !ok {
			// entity key has already been removed
			return nil
		}

		if _, ok := idsMap[id]; !ok {
			// id has already been removed from the key map
			return nil
		}

		// removes map entry associated with key for update
		if _, removed := backdata.Remove(key); !removed {
			return fmt.Errorf("potential race condition on removing from identifier map")
		}

		// removes id from the secondary map of the key
		delete(idsMap, id)

		if len(idsMap) == 0 {
			// all ids related to key are removed, so there is no need
			// to add key back to the idMapEntity
			return nil
		}

		if added := backdata.Add(key, idsMap); !added {
			return fmt.Errorf("potential race condition on adding to identifier map")
		}

		return nil
	})

	return err
}

// Size returns number of a lists of identifiers in the mempool
func (i *IdentifierMap) Size() uint {
	return i.Backend.Size()
}

// Keys returns a list of all keys in the mempool
func (i *IdentifierMap) Keys() (flow.IdentifierList, bool) {
	all := i.Backend.All()
	keys := make(flow.IdentifierList, 0, len(all))
	for key, _ := range all {
		keys = append(keys, key)
	}
	return keys, true
}
