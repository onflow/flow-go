package flow

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
func Deduplicate[T IDEntity](entities []T) []T {
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
