package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Blocks implements a simple block storage around a badger DB.
type Blocks struct {
	db       storage.DB
	headers  *Headers
	payloads *Payloads
}

var _ storage.Blocks = (*Blocks)(nil)

// NewBlocks instantiates a storage abstraction combining [storage.Headers] and [storage.Payloads].
// Caching is implemented by the constituent parts.
func NewBlocks(db storage.DB, headers *Headers, payloads *Payloads) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

// BatchStore adds the provided block to the database write batch and populates all secondary storage indices
// (maps from the block ID to some block-related information).
//
// CAUTION: Under the hood, `BatchStore` performs some prior database reads, which must happen atomically with
// the subsequent database write in order to prevent accidental state corruption. Therefore, the caller must
// acquire [storage.LockInsertBlock] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
// - [storage.ErrAlreadyExists] if some block with the same ID has already been stored
// - [storage.ErrWrongChain] if the block is part of a different chain than this storage was initialized with
func (b *Blocks) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, proposal *flow.Proposal) error {
	blockID := proposal.Block.ID()
	err := b.headers.storeTx(lctx, rw, blockID, proposal.Block.ToHeader(), proposal.ProposerSigData)
	if err != nil {
		return fmt.Errorf("could not store header %v: %w", blockID, err)
	}
	err = b.payloads.storeTx(lctx, rw, blockID, &proposal.Block.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload: %w", err)
	}
	return nil
}

// retrieve returns the block with the given hash. It is available for
// finalized and pending blocks.
// Expected error returns during normal operations:
// - [storage.ErrNotFound] if no block is found
// - [storage.ErrWrongChain] if a block with that ID exists but on a different chain, such as a cluster chain
func (b *Blocks) retrieve(blockID flow.Identifier) (*flow.Block, error) {
	header, err := b.headers.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}
	payload, err := b.payloads.retrieveTx(blockID)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not retrieve payload for block %x: %w", blockID, err)
	}
	untrustedBlock := flow.UntrustedBlock{
		HeaderBody: header.HeaderBody,
		Payload:    *payload,
	}
	var block *flow.Block
	if header.ContainsParentQC() {
		block, err = flow.NewBlock(untrustedBlock)
		if err != nil {
			return nil, fmt.Errorf("could not construct block: %w", err)
		}
	} else {
		block, err = flow.NewRootBlock(untrustedBlock)
		if err != nil {
			return nil, fmt.Errorf("could not construct root block: %w", err)
		}
	}
	return block, nil
}

// retrieveProposal returns the proposal with the given block ID.
// It is available for finalized and pending blocks.
// Expected errors during normal operations:
// - [storage.ErrNotFound] if no block is found
// - [storage.ErrWrongChain] if a block with that ID exists but on a different chain, such as a cluster chain
func (b *Blocks) retrieveProposal(blockID flow.Identifier) (*flow.Proposal, error) {
	block, err := b.retrieve(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block body: %w", err)
	}
	sig, err := b.headers.sigs.retrieveTx(blockID)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not retrieve proposer signature for block %x: %w", blockID, err)
	}

	untrustedProposal := flow.UntrustedProposal{
		Block:           *block,
		ProposerSigData: sig,
	}
	var proposal *flow.Proposal
	if block.ContainsParentQC() {
		proposal, err = flow.NewProposal(untrustedProposal)
		if err != nil {
			return nil, fmt.Errorf("could not construct proposal: %w", err)
		}
	} else {
		proposal, err = flow.NewRootProposal(untrustedProposal)
		if err != nil {
			return nil, fmt.Errorf("could not construct root proposal: %w", err)
		}
	}

	return proposal, nil
}

// ByID returns the block with the given hash. It is available for all incorporated blocks (validated blocks
// that have been appended to any of the known forks) no matter whether the block has been finalized or not.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block with the corresponding ID was found
//   - [storage.ErrWrongChain] if a block with that ID exists but on a different chain, such as a cluster chain
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieve(blockID)
}

// ProposalByID returns the block with the given ID, along with the proposer's signature on it.
// It is available for all incorporated blocks (validated blocks that have been appended to any
// of the known forks) no matter whether the block has been finalized or not.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block with the corresponding ID was found
//   - [storage.ErrWrongChain] if a block with that ID exists but on a different chain, such as a cluster chain
func (b *Blocks) ProposalByID(blockID flow.Identifier) (*flow.Proposal, error) {
	return b.retrieveProposal(blockID)
}

// ByView returns the block with the given view. It is only available for certified blocks.
// certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
// even for non-finalized blocks.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block is known at given view.
func (b *Blocks) ByView(view uint64) (*flow.Block, error) {
	blockID, err := b.headers.BlockIDByView(view)
	if err != nil {
		return nil, err
	}
	block, err := b.ByID(blockID)
	if err != nil {
		// failure to retrieve a block that has been indexed indicates state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed block for view %d: %w", view, err)
	}
	return block, nil
}

// ProposalByView returns the block proposal with the given view. It is only available for certified blocks.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block is known at given view.
func (b *Blocks) ProposalByView(view uint64) (*flow.Proposal, error) {
	blockID, err := b.headers.BlockIDByView(view)
	if err != nil {
		return nil, err
	}
	proposal, err := b.retrieveProposal(blockID)
	if err != nil {
		// not being able to retrieve a proposal indexed by view indicates state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve proposal for view %d: %w", view, err)
	}
	return proposal, nil
}

// ByHeight returns the block at the given height. It is only available
// for finalized blocks.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block for the corresponding height was found
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	block, err := b.retrieve(blockID)
	if err != nil {
		// failure to retrieve a block that has been indexed indicates state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed block for height %d: %w", height, err)
	}
	return block, nil
}

// ProposalByHeight returns the block at the given height, along with the proposer's
// signature on it. It is only available for finalized blocks.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block proposal for the corresponding height was found
func (b *Blocks) ProposalByHeight(height uint64) (*flow.Proposal, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	proposal, err := b.retrieveProposal(blockID)
	if err != nil {
		// failure to retrieve a block that has been indexed indicates state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed proposal for height %d: %w", height, err)
	}
	return proposal, nil
}

// ByCollectionID returns the block for the given [flow.CollectionGuarantee] ID.
// This method is only available for collections included in finalized blocks.
// While consensus nodes verify that collections are not repeated within the same fork,
// each different fork can contain a recent collection once. Therefore, we must wait for
// finality.
// CAUTION: this method is not backed by a cache and therefore comparatively slow!
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if the collection ID was not found
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	blockID, err := b.BlockIDByCollectionID(collID)
	if err != nil {
		return nil, err
	}
	block, err := b.ByID(blockID)
	if err != nil {
		// failure to retrieve a block that has been indexed indicates state corruption
		return nil, irrecoverable.NewExceptionf("could not retrieve indexed block %x for collection id %x: %w", blockID, collID, err)
	}
	return block, nil
}

// BlockIDByCollectionID returns the block ID for the finalized block which includes the guarantee for the
// given collection (the collection guarantee such that `CollectionGuarantee.CollectionID == collID`).
// This function returns the finalized _consensus_ block including the specified collection, not the cluster
// block which defines the collection.
// NOTE: This method is only available for collections included in finalized blocks.
// While consensus nodes verify that collections are not repeated within the same fork,
// each different fork can contain a recent collection once. Therefore, we must wait for
// finality.
// CAUTION: this method is not backed by a cache and therefore comparatively slow!
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no FINALIZED block exists containing the expected collection guarantee
func (b *Blocks) BlockIDByCollectionID(collID flow.Identifier) (flow.Identifier, error) {
	guarantee, err := b.payloads.guarantees.ByCollectionID(collID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not look up guarantee: %w", err)
	}
	var blockID flow.Identifier
	err = operation.LookupBlockContainingCollectionGuarantee(b.db.Reader(), guarantee.ID(), &blockID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not look up block: %w", err)
	}
	// CAUTION: a collection can be included in multiple *unfinalized* blocks. However, the implementation
	// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
	// *and* only in the absence of byzantine collector clusters (which the mature protocol must tolerate).
	// Hence, this function should be treated as a temporary solution, which requires generalization
	// (one-to-many mapping) for soft finality and the mature protocol.
	return blockID, nil
}

// BatchIndexBlockContainingCollectionGuarantees produces mappings from the IDs of [flow.CollectionGuarantee]s to the block ID containing these guarantees.
// The caller must acquire [storage.LockIndexBlockByPayloadGuarantees] and hold it until the database write has been committed.
//
// CAUTION: a collection can be included in multiple *unfinalized* blocks. However, the implementation
// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
// *and* only in the absence of byzantine collector clusters (which the mature protocol must tolerate).
// Hence, this function should be treated as a temporary solution, which requires generalization
// (one-to-many mapping) for soft finality and the mature protocol.
//
// Expected errors during normal operations:
//   - [storage.ErrAlreadyExists] if any collection guarantee is already indexed
func (b *Blocks) BatchIndexBlockContainingCollectionGuarantees(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, guaranteeIDs []flow.Identifier) error {
	return operation.BatchIndexBlockContainingCollectionGuarantees(lctx, rw, blockID, guaranteeIDs)
}
