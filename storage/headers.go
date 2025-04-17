package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Headers represents persistent storage for blocks.
type Headers interface {

	// Store will store a header.
	// Error returns:
	//   - storage.ErrAlreadyExists if a header for the given blockID already exists in the database.
	//   - generic error in case of unexpected failure from the database layer or encoding failure.
	Store(proposal *flow.Proposal) error

	// ByBlockID returns the header with the given ID. It is available for finalized blocks and those pending finalization.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) (*flow.Header, error)

	// ByHeight returns the block with the given number. It is only available for finalized blocks.
	// Error returns:
	//  - ErrNotFound if no finalized block is known at the given height
	ByHeight(height uint64) (*flow.Header, error)

	// Exists returns true if a header with the given ID has been stored.
	// CAUTION: this method is not backed by a cache and therefore comparatively slow!
	// No errors are expected during normal operation.
	Exists(blockID flow.Identifier) (bool, error)

	// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
	// version of `ByHeight` that skips retrieving the block. Expected errors during normal operations:
	//  - storage.ErrNotFound if no finalized block is known at given height
	BlockIDByHeight(height uint64) (flow.Identifier, error)

	// ByParentID finds all children for the given parent block. The returned headers
	// might be unfinalized; if there is more than one, at least one of them has to
	// be unfinalized.
	// CAUTION: this method is not backed by a cache and therefore comparatively slow!
	ByParentID(parentID flow.Identifier) ([]*flow.Header, error)

	// ProposalByBlockID returns the header with the given ID, along with the corresponding proposer signature.
	// It is available for finalized blocks and those pending finalization.
	// Error returns:
	//  - ErrNotFound if no block header or proposer signature with the given blockID exists
	ProposalByBlockID(blockID flow.Identifier) (*flow.Proposal, error)
}
