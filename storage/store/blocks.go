package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
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

// NewBlocks ...
func NewBlocks(db storage.DB, headers *Headers, payloads *Payloads) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

// BatchStore stores a valid block in a batch.
//
// Expected errors during normal operations:
// - [storage.ErrAlreadyExists] if some block with the same ID has already been stored
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
// Expected errors during normal operations:
// - storage.ErrNotFound if no block is found
func (b *Blocks) retrieve(blockID flow.Identifier) (*flow.Block, error) {
	header, err := b.headers.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}
	payload, err := b.payloads.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve payload: %w", err)
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
// - storage.ErrNotFound if no block is found
func (b *Blocks) retrieveProposal(blockID flow.Identifier) (*flow.Proposal, error) {
	block, err := b.retrieve(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve block body: %w", err)
	}
	sig, err := b.headers.sigs.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve proposer signature: %w", err)
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
// Error returns:
//   - storage.ErrNotFound if no block with the corresponding ID was found
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieve(blockID)
}

// ProposalByID returns the block with the given ID, along with the proposer's signature on it.
// It is available for all incorporated blocks (validated blocks that have been appended to any
// of the known forks) no matter whether the block has been finalized or not.
//
// Error returns:
//   - storage.ErrNotFound if no block with the corresponding ID was found
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func (b *Blocks) ProposalByID(blockID flow.Identifier) (*flow.Proposal, error) {
	return b.retrieveProposal(blockID)
}

// ByView returns the block with the given view. It is only available for certified blocks.
// certified blocks are the blocks that have received QC. Hotstuff guarantees that for each view,
// at most one block is certified. Hence, the return value of `ByView` is guaranteed to be unique
// even for non-finalized blocks.
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no certified block is known at given view.
func (b *Blocks) ByView(view uint64) (*flow.Block, error) {
	blockID, err := b.headers.BlockIDByView(view)
	if err != nil {
		return nil, err
	}
	return b.ByID(blockID)
}

// ProposalByView returns the block proposal with the given view. It is only available for certified blocks.
//
// Expected errors during normal operations:
//   - `storage.ErrNotFound` if no certified block is known at given view.
func (b *Blocks) ProposalByView(view uint64) (*flow.Proposal, error) {
	blockID, err := b.headers.BlockIDByView(view)
	if err != nil {
		return nil, err
	}
	return b.retrieveProposal(blockID)
}

// ByHeight returns the block at the given height. It is only available
// for finalized blocks.
//
// Error returns:
//   - storage.ErrNotFound if no block for the corresponding height was found
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return b.retrieve(blockID)
}

// ProposalByHeight returns the block at the given height, along with the proposer's
// signature on it. It is only available for finalized blocks.
//
// Error returns:
//   - storage.ErrNotFound if no block proposal for the corresponding height was found
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value
func (b *Blocks) ProposalByHeight(height uint64) (*flow.Proposal, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return b.retrieveProposal(blockID)
}

// ByCollectionID returns the block for the given [flow.CollectionGuarantee] ID.
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
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	guarantee, err := b.payloads.guarantees.ByCollectionID(collID)
	if err != nil {
		return nil, fmt.Errorf("could not look up guarantee: %w", err)
	}
	var blockID flow.Identifier
	err = operation.LookupBlockContainingCollectionGuarantee(b.db.Reader(), guarantee.ID(), &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// BatchIndexBlockContainingCollectionGuarantees produces mappings from the IDs of [flow.CollectionGuarantee]s to the block ID containing these guarantees.
// The caller must acquire a storage.LockIndexCollectionByBlock lock.
// Error returns:
//   - storage.ErrAlreadyExists if any collection ID has already been indexed
func (b *Blocks) BatchIndexBlockContainingCollectionGuarantees(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, guaranteeIDs []flow.Identifier) error {
	return operation.BatchIndexBlockContainingCollectionGuarantees(lctx, rw, blockID, guaranteeIDs)
}
