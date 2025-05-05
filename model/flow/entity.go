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

// idCache is a utility for caching the ID (canonical hash) of a Flow entity.
// The entity should define an UncachedID method, which computes the ID without caching.
// Upon construction, the entity should construct a cache using newIDCache,
// passing in the UncachedID method as the single parameter.
// Entities should use a literal (not pointer) field for the cache, as the
// cache already includes only reference-typed fields and can be safely copied.
//
// idCache is safe for concurrent use by multiple goroutines.
//
// CAUTION: idCache is a write-once, never-invalidated cache for a derived field.
// This means that instances of entities that use idCache must be immutable after construction.
// These entities should opt-in to the structwrite linter to enforce most classes of immutability.
type idCache struct {
	id        *atomic.Pointer[Identifier]
	computeID func() Identifier
}

// newIDCache returns a new idCache.
// Caller is responsible for ensure that computeID always returns the same ID
// (should be a deterministic hashing function over an immutable data structure).
func newIDCache(computeID func() Identifier) idCache {
	return idCache{
		id:        new(atomic.Pointer[Identifier]),
		computeID: computeID,
	}
}

// EncodeRLP overrides RLP encoding when this type is encoded.
// This implementation results in an empty encoding of 0 bytes.
// This is useful when the cache is included as a field of a struct,
// so that any cached value does not impact the encoding of the struct.
func (cache idCache) EncodeRLP(w io.Writer) error {
	return nil
}

// getID computes the ID or returns a cached version if the ID has ever been computed before.
// In the typical path where the ID is cached, we simply read and return the atomic.
// If the ID is not yet cached, we compute it, then attempt to store it to the atomic.
// The ID is stored using CAS so it is stored only once.
// This function may panic if two goroutines compute inconsistent IDs.
func (cache idCache) getID() Identifier {
	// if ID is already computed and cached, return it
	v := cache.id.Load()
	if v != nil {
		return *v
	}

	// compute the ID and attempt to store it
	computedID := cache.computeID()
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
