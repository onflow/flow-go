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
	// finalized and pending blocks.
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available for finalized blocks.
	//
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found for the given height
	ByHeight(height uint64) (*flow.Block, error)

	// ByCollectionID returns the block for the given collection ID.
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
