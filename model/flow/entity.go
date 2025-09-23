package flow

// Hashable structures provide a deterministic, collision resistant digest (i.e. hash) of 32 bits, which
// serves as an `Identifier` for this data structure. The protocol mandates that *all* fields must be covered
// by the hash, such that *any* modifications of the fields yield a different hash (collision resistance).
// Details:
//  1. The Hash is used to confirm integrity of a data structure by the recipient, when it is relayed via
//     a potentially byzantine intermediary. Only when the hash covers all fields in the data structure and
//     embedded structs (irrespective of whether the fields are exported or not), the recipient can be sure
//     that the structure has not been tempered with.
//  2. In some rare cases, there might be auxiliary information for a struct that has no influence on the
//     struct's behaviour whatsoever. Nevertheless, following the Principle of 'Safety By Default', it is
//     prohibited to include such auxiliary information in the struct while excluding it during hashing.
//     This rule exists, because repeatedly such auxiliary information was later used by protocol logic in
//     subsequent code revisions, but still excluded from the hash, hence creating a malleability vulnerability
//     in the protocol. Therefore, we forbid excluding any data in a struct from hashing - no exceptions!
//  3. Many structures (such as Block Headers, Execution Receipts, etc) contain signatures. Such signatures
//     must be included in the Hash (otherwise, a byzantine node could temper with the signature, without it
//     being attributable to them). For clarity and to avoid subtle security pitfalls, we recommend to
//     introduce dedicated "unsigned structs" (such as `UnsignedBlock`) which exclude the signature field.
//     In general, the "unsigned structs" are interim representations for node-internal usage only. From the
//     "unsigned struct" plus a signature over the "unsigned struct", the final data structure can be
//     constructed, which is byzantine-fault-tolerant and can be safely shared with other nodes.
//  4. For Hashable structures under construction, we recommend a builder. For auxiliary information, we
//     recommend the decorator patter. Though, the decorator must not expose the embedded struct's Hash
//     method, as this would be violating 2.
type Hashable interface {
	// Hash returns a deterministic, collision resistant digest (i.e. hash) of 32 bits, which
	// serves as an `Identifier` for this data structure.
	// The protocol mandates that *all* fields must be covered
	// by the hash, such that *any* modifications of the fields yield a different hash (collision resistance).
	Hash() Identifier
}

func EntitiesToIDs[T Hashable](entities []T) []Identifier {
	ids := make([]Identifier, 0, len(entities))
	for _, entity := range entities {
		ids = append(ids, entity.Hash())
	}
	return ids
}

// Deduplicate entities in a slice by the ID method
// The original order of the entities is preserved.
func Deduplicate[T Hashable](entities []T) []T {
	if entities == nil {
		return nil
	}

	seen := make(map[Identifier]struct{}, len(entities))
	result := make([]T, 0, len(entities))

	for _, entity := range entities {
		id := entity.Hash()
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, entity)
	}

	return result
}

// EntityRequest is a request for a set of entities, each keyed by an
// identifier. The relationship between the identifiers and the entity is not
// specified here. In the typical case, the identifier is simply the ID of the
// entity being requested, but more complex identifier-entity relationships can
// be used as well.
type EntityRequest struct {
	Nonce     uint64
	EntityIDs []Identifier
}

// EntityResponse is a response to an entity request, containing a set of
// serialized entities and the identifiers used to request them. The returned
// entity set may be empty or incomplete.
type EntityResponse struct {
	Nonce     uint64
	EntityIDs []Identifier
	Blobs     [][]byte
}
