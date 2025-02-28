package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// MutableBackData extends BackData by allowing modifications to stored data structures.
// Unlike BackData, this interface supports adjusting existing data structures, making it suitable for use cases
// where they do not have a cryptographic hash function.
//
// WARNING: Entities that are cryptographically protected, such as Entity objects tied to signatures or hashes,
// should not be modified. Use BackData instead to prevent unintended mutations.
type MutableBackData interface {
	BackData

	// Adjust adjusts the entity using the given function if the given identifier can be found.
	// Returns: 
	//    - Entity, true if the entity with the given ID was found. The returned Entity is the version after the update is applied.
	//    - nil, false if no entity with the given ID was found
	Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool)

	// AdjustWithInit adjusts the entity using the given function if the given identifier can be found. When the
	// entity is not found, it initializes the entity using the given init function and then applies the adjust function.
	// Args:
	// - entityID: the identifier of the entity to adjust.
	// - adjust: the function that adjusts the entity.
	// - init: the function that initializes the entity when it is not found.
	// Returns:
	//   - the adjusted entity.
	//   - a bool which indicates whether the entity was either added or adjusted.
	AdjustWithInit(entityID flow.Identifier, adjust func(flow.Entity) flow.Entity, init func() flow.Entity) (flow.Entity, bool)
}
