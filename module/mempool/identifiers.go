package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Identifiers represents a concurrency-safe memory pool for identifiers
type Identifiers interface {
	// Has checks whether the mempool has the identifier
	Has(id flow.Identifier) bool

	// Add will add the given identifier to the memory pool or it will error if
	// the identifier is already in the memory pool.
	Add(id flow.Identifier) error

	// Rem removes the given identifier
	Rem(id flow.Identifier) bool

	// All will retrieve all identifiers that are currently in the memory pool
	// as an IdentityList
	All() flow.IdentifierList
}
