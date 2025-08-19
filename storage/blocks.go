package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
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

<<<<<<< HEAD
	// BatchStore stores a valid block in a batch.
	BatchStore(lctx lockctx.Proof, rw ReaderBatchWriter, block *flow.Block) error

	// ByID returns the block with the given hash. It is available for
	// finalized and pending blocks.
	// Expected errors during normal operations:
	// - storage.ErrNotFound if no block is found
=======
	// Store will atomically store a block with all its dependencies.
	//
	// Error returns:
	//   - storage.ErrAlreadyExists if the blockID already exists in the database.
	//   - generic error in case of unexpected failure from the database layer or encoding failure.
	Store(proposal *flow.Proposal) error

	// StoreTx allows us to store a new block, including its payload & header,
	// as part of a DB transaction, while still going through the caching layer.
	//
	// Error returns:
	//   - storage.ErrAlreadyExists if the blockID already exists in the database.
	//   - generic error in case of unexpected failure from the database layer or encoding failure.
	StoreTx(proposal *flow.Proposal) func(*transaction.Tx) error

	// ByID returns the block with the given hash. It is available for all incorporated blocks (validated blocks
	// that have been appended to any of the known forks) no matter whether the block has been finalized or not.
	//
	// Error returns:
	//   - storage.ErrNotFound if no block with the corresponding ID was found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
>>>>>>> feature/malleability
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ProposalByID returns the block with the given ID, along with the proposer's signature on it.
	// It is available for all incorporated blocks (validated blocks that have been appended to any
	// of the known forks) no matter whether the block has been finalized or not.
	//
	// Error returns:
	//   - storage.ErrNotFound if no block with the corresponding ID was found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ProposalByID(blockID flow.Identifier) (*flow.Proposal, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	//
	// Error returns:
	//   - storage.ErrNotFound if no block for the corresponding height was found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ByHeight(height uint64) (*flow.Block, error)

<<<<<<< HEAD
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
=======
	// ProposalByHeight returns the block at the given height, along with the proposer's
	// signature on it. It is only available for finalized blocks.
	//
	// Error returns:
	//   - storage.ErrNotFound if no block proposal for the corresponding height was found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ProposalByHeight(height uint64) (*flow.Proposal, error)

	// ByCollectionID returns the block for the given collection ID.
	// This method is only available for collections included in finalized blocks.
	// While consensus nodes verify that collections are not repeated within the same fork,
	// each different fork can contain a recent collection once. Therefore, we must wait for
	// finality.
	// CAUTION: this method is not backed by a cache and therefore comparatively slow!
	//
	// Error returns:
	//   - storage.ErrNotFound if the collection ID was not found
	//   - generic error in case of unexpected failure from the database layer, or failure
	//     to decode an existing database value
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)

	// IndexBlockForCollectionGuarantees is only to be called for *finalized* blocks. For each
	// collection ID, it stores the blockID as the block containing this collection.
	// While consensus nodes verify that collections are not repeated within the same fork,
	// each different fork can contain a recent collection once. Therefore, we must wait for
	// finality.
	//
	// Error returns:
	//   - generic error in case of unexpected failure from the database layer or encoding failure.
	IndexBlockForCollectionGuarantees(blockID flow.Identifier, collIDs []flow.Identifier) error
>>>>>>> feature/malleability
}
