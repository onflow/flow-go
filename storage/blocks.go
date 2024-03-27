package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// Store will atomically store a block with all its dependencies.
	Store(block *flow.Block) error

	// StoreTx allows us to store a new block, including its payload & header, as part of a DB transaction, while
	// still going through the caching layer.
	StoreTx(block *flow.Block) func(*transaction.Tx) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was
	// included in.
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error

	// InsertLastFullBlockHeightIfNotExists inserts the FullBlockHeight index if it does not already exist.
	// Calling this function multiple times is a no-op and returns no expected errors.
	InsertLastFullBlockHeightIfNotExists(height uint64) error

	// UpdateLastFullBlockHeight updates the FullBlockHeight index
	// The FullBlockHeight index indicates that block for which all collections have been received
	UpdateLastFullBlockHeight(height uint64) error

	// GetLastFullBlockHeight retrieves the FullBlockHeight
	GetLastFullBlockHeight() (height uint64, err error)
}
