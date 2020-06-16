package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IdentifierMap represents a concurrency-safe memory pool for mapping an identifier to a list of identifiers
type IdentifierMap interface {
	// Add will append the id to the list of identifier associated with key.
	Append(key, id flow.Identifier) bool

	// Rem removes the given key with all associated identifiers.
	Rem(key flow.Identifier) bool

	// Get returns list of all identifiers associated with key and True, if the key exists in the mempool.
	// Otherwise it returns nil and false.
	Get(key flow.Identifier) ([]flow.Identifier, bool)
}
