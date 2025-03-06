package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Headers represents persistent storage for blocks.
type Headers interface {

	// Store persists a new header.
	// Expected errors during normal operation:
	//   - storage.ErrAlreadyExists if the header has already been persisted
	Store(header *flow.Header) error

	// ByBlockID returns the header with the given ID. It is available for finalized and ambiguous blocks.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) (*flow.Header, error)

	// ByHeight returns the block with the given number. It is only available for finalized blocks.
	ByHeight(height uint64) (*flow.Header, error)

	// ByView returns the block with the given view. It is only available for certified blocks.
	// Certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
	// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
	// oven for non-finalized blocks.
	//
	// TODO: this method is not available until next spork (mainnet27) or a migration that builds the index.
	// ByView(view uint64) (*flow.Header, error)

	// Exists returns true if a header with the given ID has been stored.
	// No errors are expected during normal operation.
	Exists(blockID flow.Identifier) (bool, error)

	// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
	// version of `ByHeight` that skips retrieving the block. Expected errors during normal operations:
	//  - storage.ErrNotFound if no finalized block is known at given height
	BlockIDByHeight(height uint64) (flow.Identifier, error)

	// ByParentID finds all children for the given parent block. The returned headers
	// might be unfinalized; if there is more than one, at least one of them has to
	// be unfinalized.
	ByParentID(parentID flow.Identifier) ([]*flow.Header, error)
}
