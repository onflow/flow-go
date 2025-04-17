package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks provides persistent storage for blocks.
//
// Conceptually, blocks must always be signed by the proposer. Once a block is certified (i.e.
// received votes from a supermajority of consensus participants, in their aggregated form
// represented by the Quorum Certificate [QC]), the proposer's signature is included in the QC
// and does not need to be provided individually anymore. Therefore, from the protocol perspective,
// the proper data structures are either a block proposal (including the proposer's signature) or
// a certified block (including a QC for the block).
type Blocks interface {

	// Store will atomically store a block with all its dependencies.
	Store(proposal *flow.BlockProposal) error

	// StoreTx allows us to store a new block, including its payload & header,
	// as part of a DB transaction, while still going through the caching layer.
	StoreTx(proposal *flow.BlockProposal) func(*transaction.Tx) error

	// ByID returns the block with the given hash. It is available for all incorporated blocks (validated blocks
	// that have been appended to any of the known forks) no matter whether the block has been finalized or not.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ProposalByID returns the block with the given ID, along with the proposer's signature on it.
	// It is available for all incorporated blocks (validated blocks that have been appended to any
	// of the known forks) no matter whether the block has been finalized or not.
	ProposalByID(blockID flow.Identifier) (*flow.BlockProposal, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ProposalByHeight returns the block at the given height, along with the proposer's
	// signature on it. It is only available for finalized blocks.
	ProposalByHeight(height uint64) (*flow.BlockProposal, error)

	// ByCollectionID returns the block for the given collection ID.
	// This method is only available for collections included in finalized blocks.
	// While consensus nodes verify that collections are not repeated within the same fork,
	// each different fork can contain a recent collection once. Therefore, we must wait for
	// finality.
	// CAUTION: this method is not backed by a cache and therefore comparatively slow!
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollections is only to be called for *finalized* blocks. For each
	// collection ID, it stores the blockID as the block containing this collection.
	// While consensus nodes verify that collections are not repeated within the same fork,
	// each different fork can contain a recent collection once. Therefore, we must wait for
	// finality.
	IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error
}
