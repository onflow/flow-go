package internal

import "github.com/onflow/flow-go/model/flow"

// WrappedEntity is a wrapper around a flow.Entity that allows overriding the ID.
// The has 2 main use cases:
//   - when the ID is expensive to compute, we can pre-compute it and use it for the cache
//   - when caching an entity using a different ID than what's returned by ID(). For example, if there
//     is a 1:1 mapping between a block and an entity, we can use the block ID as the cache key.
type WrappedEntity struct {
	flow.Entity
	id flow.Identifier
}

var _ flow.Entity = (*WrappedEntity)(nil)

// NewWrappedEntity creates a new WrappedEntity
func NewWrappedEntity(id flow.Identifier, entity flow.Entity) *WrappedEntity {
	return &WrappedEntity{
		Entity: entity,
		id:     id,
	}
}

// ID returns the cached ID of the wrapped entity
func (w WrappedEntity) ID() flow.Identifier {
	return w.id
}

// Checksum returns th cached ID of the wrapped entity
func (w WrappedEntity) Checksum() flow.Identifier {
	return w.id
}
