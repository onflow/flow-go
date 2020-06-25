package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Identifiers represents a concurrency-safe memory pool for identifiers
type Identifiers interface {
	// Has checks whether the mempool has the identifier
	Has(id flow.Identifier) bool

	// Add will add the given identifier to the memory pool. It will return
	// false if it was already in the mempool.
	Add(id flow.Identifier) bool

	// Rem removes the given identifier
	Rem(id flow.Identifier) bool

	// Size returns total number of identifiers in mempool
	Size() uint

	// All will retrieve all identifiers that are currently in the memory pool
	// as an IdentityList
	All() flow.IdentifierList
}
