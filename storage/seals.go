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

	// HighestInFork retrieves the highest seal that was included in the
	// fork up to (and including) the given blockID.
	// This method should return
	//   - a seal for any block known to the node.
	//   - storage.ErrNotFound if blockID is unknown.
	HighestInFork(blockID flow.Identifier) (*flow.Seal, error)

	// FinalizedSealForBlock retrieves the finalized seal for the given block ID.
	// Returns storage.ErrNotFound if blockID is unknown or no _finalized_ seal
	// is known for the block.
	FinalizedSealForBlock(blockID flow.Identifier) (*flow.Seal, error)
}
