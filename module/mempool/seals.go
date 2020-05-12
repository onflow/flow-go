// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Seals represents a concurrency-safe memory pool for block seals.
type Seals interface {

	// Has checks whether the block seal with the given hash is currently in
	// the memory pool.
	Has(sealID flow.Identifier) bool

	// Add will add the given block seal to the memory pool. It will return
	// false if it was already in the mempool.
	Add(seal *flow.Seal) bool

	// Rem will remove the given block seal from the memory pool; it will
	// will return true if the block seal was known and removed.
	Rem(sealID flow.Identifier) bool

	// ByID retrieve the block seal with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByID(sealID flow.Identifier) (*flow.Seal, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all block seals that are currently in the memory pool
	// as a slice.
	All() []*flow.Seal

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
