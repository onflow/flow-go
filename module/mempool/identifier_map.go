package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// IdentifierMap represents a concurrency-safe memory pool for mapping an identifier to a list of identifiers
type IdentifierMap interface {
	// Append will append the id to the list of identifiers associated with key.
	Append(key, id flow.Identifier)

	// Remove removes the given key with all associated identifiers.
	Remove(key flow.Identifier) bool

	// RemoveIdFromKey removes the id from the list of identifiers associated with key.
	// If the list becomes empty, it also removes the key from the map.
	RemoveIdFromKey(key, id flow.Identifier) error

	// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
	// Otherwise it returns nil and false.
	Get(key flow.Identifier) (flow.IdentifierList, bool)

	// Has returns true if the key exists in the map, i.e., there is at least an id
	// attached to it.
	Has(key flow.Identifier) bool

	// Keys returns a list of all keys in the mempool
	Keys() (flow.IdentifierList, bool)

	// Size returns number of IdMapEntities in mempool
	Size() uint
}
