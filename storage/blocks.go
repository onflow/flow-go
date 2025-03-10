package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// Store persists a block with all its dependencies.
	// Expected errors during normal operation:
	//   - storage.ErrAlreadyExists if the block has already been persisted
	Store(block *flow.Block) error

	// StoreTx allows us to store a new block, including its payload & header, as part of a DB transaction, while
	// still going through the caching layer.
	// Expected errors during normal operation:
	//   - storage.ErrAlreadyExists if the block has already been persisted
	StoreTx(block *flow.Block) func(*transaction.Tx) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ByView returns the block with the given view. It is only available for certified blocks.
	// certified blocks are the blocks that have received QC.
	//
	// TODO: this method is not available until next spork (mainnet27) or a migration that builds the index.
	// ByView(view uint64) (*flow.Header, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was
	// included in.
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
