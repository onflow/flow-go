package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// StoreTx allows us to store a new block, including its payload & header, as part of a DB transaction, while
	// still going through the caching layer.
	// Deprecated: to be removed alongside Badger DB
	StoreTx(block *flow.Block) func(*transaction.Tx) error

	// BatchStore stores a valid block in a batch.
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, block *flow.Block) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available for finalized blocks.
	//
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found for the given height
	ByHeight(height uint64) (*flow.Block, error)

	// ByView returns the block with the given view. It is only available for certified blocks.
	// certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
	// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
	// even for non-finalized blocks.
	// Expected errors during normal operations:
	//   - `storage.ErrNotFound` if no certified block is known at given view.
	//
	// TODO: this method is not available until next spork (mainnet27) or a migration that builds the index.
	// ByView(view uint64) (*flow.Header, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was
	// included in. This should not be called when finalizing a block
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
