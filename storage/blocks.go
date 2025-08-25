package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// BatchStore stores a valid block in a batch.
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, block *flow.Block) error

	// ByID returns the block with the given hash. It is available for
	// finalized and pending blocks.
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available for finalized blocks.
	//
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found for the given height
	ByHeight(height uint64) (*flow.Block, error)

	// ByView returns the block with the given view. It is only available for certified blocks.
	// Certified blocks are the blocks that have received a QC. Hotstuff guarantees that for each view,
	// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
	// even for non-finalized blocks.
	// Expected errors during normal operations:
	//   - `storage.ErrNotFound` if no certified block is known at given view.
	//
	// TODO: this method is not available until next spork (mainnet27) or a migration that builds the index.
	// ByView(view uint64) (*flow.Header, error)

	// ByCollectionID returns the *finalized** block that contains the collection with the given ID.
	//
	// Expected errors during normal operations:
	// - storage.ErrNotFound if finalized block is known that contains the collection
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was included in.
	// CAUTION: a collection can be included in multiple *unfinalized* blocks. However, the implementation
	// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
	// *and* only in the absence of byzantine collector clusters (which the mature protocol must tolerate).
	// Hence, this function should be treated as a temporary solution, which requires generalization
	// (one-to-many mapping) for soft finality and the mature protocol.
	//
	// No errors expected during normal operation.
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
