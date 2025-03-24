package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// IdentifierMap represents a concurrency-safe memory pool for sets of Identifier (keyed by some Identifier).
type IdentifierMap struct {
	*Backend[flow.Identifier, map[flow.Identifier]struct{}]
}

// NewIdentifierMap creates a new memory pool for sets of Identifier (keyed by some Identifier).
func NewIdentifierMap(limit uint) *IdentifierMap {
	return &IdentifierMap{NewBackend(WithLimit[flow.Identifier, map[flow.Identifier]struct{}](limit))}
}

// Append will add the id to the set of identifiers associated with key.
// If the key does not exist, a new set containing only `id` is stored.
// If `id` already exists in the set stored under `key`, Append is a no-op.
func (i *IdentifierMap) Append(key, id flow.Identifier) {
	i.Backend.AdjustWithInit(key, func(stored map[flow.Identifier]struct{}) map[flow.Identifier]struct{} {
		stored[id] = struct{}{}
		return stored
	}, func() map[flow.Identifier]struct{} {
		return map[flow.Identifier]struct{}{id: {}}
	})
}

// Get returns the set of all identifiers associated with key and true, if the key exists in the mempool.
// The set is returned as an unordered list with no duplicates.
// Otherwise it returns nil and false.
func (i *IdentifierMap) Get(key flow.Identifier) (flow.IdentifierList, bool) {
	var ids flow.IdentifierList
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

		ids = make(flow.IdentifierList, 0, len(idsMap))
		for id := range idsMap {
			ids = append(ids, id)
		}

		return nil
	})

	if err != nil {
		return nil, false
	}
	return ids, true
}

// RemoveIdFromKey removes the id from the list of identifiers associated with key.
// If the list becomes empty, it also removes the key from the map.
func (i *IdentifierMap) RemoveIdFromKey(key, id flow.Identifier) error {
	err := i.Backend.Run(func(backData mempool.BackData[flow.Identifier, map[flow.Identifier]struct{}]) error {
		idsMap, ok := backData.Get(key)
		if !ok {
			// entity key has already been removed
			return nil
		}

		delete(idsMap, id) // mutates the map stored in backData
		if len(idsMap) == 0 {
			// if the set stored under the key is empty, remove the key
			if _, removed := backData.Remove(key); !removed {
				return fmt.Errorf("sanity check failed: race condition observed removing from identifier map (key=%x, id=%x)", key, id)
			}
		}

		return nil
	})

	return err
}

// Keys returns a list of all keys in the mempool.
func (i *IdentifierMap) Keys() (flow.IdentifierList, bool) {
	all := i.Backend.All()
	keys := make(flow.IdentifierList, 0, len(all))
	for key := range all {
		keys = append(keys, key)
	}
	return keys, true
}
