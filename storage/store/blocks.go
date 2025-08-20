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

// ByID returns the block with the given hash. It is available for
// finalized and pending blocks.
// Expected errors during normal operations:
// - storage.ErrNotFound if no block is found
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieve(blockID)
}

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

// ByHeight returns the block at the given height. It is only available
// for finalized blocks.
//
// Expected errors during normal operations:
// - storage.ErrNotFound if no block is found for the given height
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return b.retrieve(blockID)
}

func (b *Blocks) ProposalByHeight(height uint64) (*flow.Proposal, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return b.retrieveProposal(blockID)
}

// ByCollectionID returns the *finalized** block that contains the collection with the given ID.
//
// Expected errors during normal operations:
// - storage.ErrNotFound if finalized block is known that contains the collection
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	guarantee, err := b.payloads.guarantees.ByCollectionID(collID)
	if err != nil {
		return nil, fmt.Errorf("could not look up guarantee: %w", err)
	}

	err = operation.LookupCollectionGuaranteeBlock(b.db.Reader(), guarantee.ID(), &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollectionGuarantees creates an index `guaranteeID->blockID` for each guarantee
// which appears in the block.
// CAUTION: a collection can be included in multiple *unfinalized* blocks. However, the implementation
// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
// *and* only in the absence of byzantine collector clusters (which the mature protocol must tolerate).
// Hence, this function should be treated as a temporary solution, which requires generalization
// (one-to-many mapping) for soft finality and the mature protocol.
//
// No errors expected during normal operation.
func (b *Blocks) IndexBlockForCollectionGuarantees(blockID flow.Identifier, guaranteeIDs []flow.Identifier) error {
	return b.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, guaranteeID := range guaranteeIDs {
			err := operation.IndexCollectionGuaranteeBlock(rw.Writer(), guaranteeID, blockID)
			if err != nil {
				return fmt.Errorf("could not index collection block (%x): %w", guaranteeID, err)
			}
		}
		return nil
	})
}
