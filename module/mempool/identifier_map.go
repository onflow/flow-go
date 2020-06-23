package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// IdentifierMap represents a concurrency-safe memory pool for mapping an identifier to a list of identifiers
type IdentifierMap interface {
	// Append will append the id to the list of identifiers associated with key.
	Append(key, id flow.Identifier) (bool, error)

	// Rem removes the given key with all associated identifiers.
	Rem(key flow.Identifier) bool

	// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
	// Otherwise it returns nil and false.
	Get(key flow.Identifier) ([]flow.Identifier, bool)
}
