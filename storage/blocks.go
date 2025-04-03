package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// Store will atomically store a block with all its dependencies.
	Store(block *flow.BlockProposal) error

	// StoreTx allows us to store a new block, including its payload & header, as part of a DB transaction, while
	// still going through the caching layer.
	StoreTx(block *flow.BlockProposal) func(*transaction.Tx) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ProposalByID returns the block with the given ID, along with the proposer's signature on it.
	// It is available for finalized and ambiguous blocks.
	ProposalByID(blockID flow.Identifier) (*flow.BlockProposal, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ProposalByHeight returns the block with proposer signature at the given height.
	// It is only available for finalized blocks.
	ProposalByHeight(height uint64) (*flow.BlockProposal, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections indexes the block each collection was
	// included in.
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
