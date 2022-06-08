// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Seals represents persistent storage for seals.
type Seals interface {

	// Store inserts the seal.
	Store(seal *flow.Seal) error

	// ByID retrieves the seal by the collection
	// fingerprint.
	ByID(sealID flow.Identifier) (*flow.Seal, error)

	// ByBlockID retrieves the last seal in the chain of seals for the block.
	ByBlockID(sealedID flow.Identifier) (*flow.Seal, error)

	// FinalizedSealForBlock retrieves the finalized seal for the given block ID.
	// Returns storage.ErrNotFound if blockID is unknown or no _finalized_ seal
	// is known for the block.
	FinalizedSealForBlock(blockID flow.Identifier) (*flow.Seal, error)
}
