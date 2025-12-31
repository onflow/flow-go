package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Headers represents persistent storage for blocks.
type Headers interface {

	// ByBlockID returns the header with the given ID. It is available for finalized blocks and those pending finalization.
	// Error returns:
	//  - [storage.ErrNotFound] if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) (*flow.Header, error)

	// CertifiedByBlockID is a temporary extension for Archival nodes with truncated chains.
	// Returns the header with the given ID and in addition a QC for this header. If either
	// the header or QC is not found, an error is returned.
	// It is available for finalized blocks and those pending finalization.
	// Error returns:
	//  - [storage.ErrNotFound] no header with the given ID is known or QC for it is found.
	//  - [BeyondArchiveThresholdError] wrapping [storage.ErrNotFound] if and only if
	//    the block with `blockID` is stored but its view exceeds the archive threshold.
	CertifiedByBlockID(blockID flow.Identifier) (*flow.Header, *flow.QuorumCertificate, error)

	// ByHeight returns the block with the given number. It is only available for finalized blocks.
	// Error returns:
	//  - [storage.ErrNotFound] if no finalized block is known at the given height
	ByHeight(height uint64) (*flow.Header, error)

	// ByView returns the block with the given view. It is only available for certified blocks.
	// Certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
	// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
	// even for non-finalized blocks.
	//
	// Expected errors during normal operations:
	//   - [storage.ErrNotFound] if no certified block is known at given view.
	ByView(view uint64) (*flow.Header, error)

	// Exists returns true if a header with the given ID has been stored.
	// No errors are expected during normal operation.
	Exists(blockID flow.Identifier) (bool, error)

	// BlockIDByHeight returns the block ID that is finalized at the given height. It is an optimized
	// version of `ByHeight` that skips retrieving the block. Expected errors during normal operations:
	//  - [storage.ErrNotFound] if no finalized block is known at given height
	BlockIDByHeight(height uint64) (flow.Identifier, error)

	// ByParentID finds all children for the given parent block. The returned headers
	// might be unfinalized; if there is more than one, at least one of them has to
	// be unfinalized.
	// CAUTION: this method is not backed by a cache and therefore comparatively slow!
	//
	// Expected error returns during normal operations:
	//   - [storage.ErrNotFound] if no block with the given parentID is known
	ByParentID(parentID flow.Identifier) ([]*flow.Header, error)

	// ProposalByBlockID returns the header with the given ID, along with the corresponding proposer signature.
	// It is available for finalized blocks and those pending finalization.
	// Error returns:
	//  - [storage.ErrNotFound] if no block header or proposer signature with the given blockID exists
	ProposalByBlockID(blockID flow.Identifier) (*flow.ProposalHeader, error)
}
