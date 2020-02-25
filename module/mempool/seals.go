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

	// Add will add the given block seal to the memory pool; it will error if
	// the block seal is already in the memory pool.
	Add(seal *flow.Seal) error

	// Rem will remove the given block seal from the memory pool; it will
	// will return true if the block seal was known and removed.
	Rem(sealID flow.Identifier) bool

	// ByID will retrieve the given block seal from the memory pool; it will
	// error if the block seal is not in the memory pool.
	ByID(sealID flow.Identifier) (*flow.Seal, error)

	// ByPreviousState will retrieve a block seal that has the given parent state.
	ByPreviousState(commit flow.StateCommitment) (*flow.Seal, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all block seals that are currently in the memory pool
	// as a slice.
	All() []*flow.Seal

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
