package flow

import (
	"fmt"
	"io"
	"sync/atomic"
)

type IDEntity interface {
	// ID returns a unique id for this entity using a hash of the immutable
	// fields of the entity.
	ID() Identifier
}

// Entity defines how flow entities should be defined
// Entities are flat data structures holding multiple data fields.
// Entities don't include nested entities, they only include pointers to
// other entities. For example, they keep a slice of entity commits instead
// of keeping a slice of entity object itself. This simplifies storage, signature and validation
// of entities.
type Entity interface {
	IDEntity

	// Checksum returns a unique checksum for the entity, including the mutable
	// data such as signatures.
	Checksum() Identifier
}

func EntitiesToIDs[T Entity](entities []T) []Identifier {
	ids := make([]Identifier, 0, len(entities))
	for _, entity := range entities {
		ids = append(ids, entity.ID())
	}
	return ids
}

// Deduplicate entities in a slice by the ID method
// The original order of the entities is preserved.
func Deduplicate[T IDEntity](entities []T) []T {
	if entities == nil {
		return nil
	}

	seen := make(map[Identifier]struct{}, len(entities))
	result := make([]T, 0, len(entities))

	for _, entity := range entities {
		id := entity.ID()
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, entity)
	}

	return result
}

// TODO docs
type idCache struct {
	id *atomic.Pointer[Identifier]
}

func newIDCache() idCache {
	return idCache{
		id: new(atomic.Pointer[Identifier]),
	}
}

func (cache *idCache) EncodeRLP(w io.Writer) error {
	return nil
}

// getID ...
func (cache *idCache) getID(computeID func() Identifier) Identifier {
	// if ID is already computed and cached, return it
	v := cache.id.Load()
	if v != nil {
		return *v
	}

	// compute the ID and attempt to store it
	computedID := computeID()
	if cache.id.CompareAndSwap(nil, &computedID) {
		// we won the race and stored the value
		return computedID
	}
	// another goroutine stored it first - sanity check that values are consistent
	storedID := *cache.id.Load()
	if computedID != storedID {
		panic(fmt.Sprintf("idCache: multiple ID computations yielded inconsistent results: %x != %x", computedID, storedID))
	}
	return computedID
}
