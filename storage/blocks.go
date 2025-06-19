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

	// BatchStoreWithStoringResults stores multiple blocks as a batch.
	// The additional storingResults parameter helps verify that each receipt in the block
	// refers to a known result. This check is essential during bootstrapping
	// when multiple blocks are stored together in a batch.
	BatchStoreWithStoringResults(lctx lockctx.Proof, rw ReaderBatchWriter, block *flow.Block, storingResults map[flow.Identifier]*flow.ExecutionResult) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was
	// included in. This should not be called when finalizing a block
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
