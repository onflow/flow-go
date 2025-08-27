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

// EntityRequest is the trusted internal representation of a request
// for one or more entities identified by their unique identifiers.
//
// This type is derived from the untrusted network-level message
// (messages.EntityRequest) during decoding. Construction of this type
// is only allowed through validation logic, ensuring the request
// contains structurally valid data.
type EntityRequest struct {
	Nonce     uint64
	EntityIDs []Identifier
}

// EntityResponse is the trusted internal representation of a response
// to an EntityRequest. It contains the set of serialized entities and
// the identifiers corresponding to each entity.
//
// This type is created from the untrusted network-level message
// (messages.EntityResponse) during decoding and validation, ensuring
// that all returned data meets structural integrity requirements.
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []Identifier
	Blobs     [][]byte
}
