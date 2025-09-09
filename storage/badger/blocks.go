package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Blocks implements a simple block storage around a badger DB.
type Blocks struct {
	db       *badger.DB
	headers  *Headers
	payloads *Payloads
}

// NewBlocks ...
func NewBlocks(db *badger.DB, headers *Headers, payloads *Payloads) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

func (b *Blocks) StoreTx(proposal *flow.Proposal) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		blockID := proposal.Block.ID()
		err := b.headers.storeTx(blockID, proposal.Block.ToHeader(), proposal.ProposerSigData)(tx)
		if err != nil {
			return fmt.Errorf("could not store header %v: %w", blockID, err)
		}
		err = b.payloads.storeTx(blockID, &proposal.Block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not store payload: %w", err)
		}
		return nil
	}
}

func (b *Blocks) retrieveTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Block, error) {
	return func(tx *badger.Txn) (*flow.Block, error) {
		header, err := b.headers.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve header: %w", err)
		}
		payload, err := b.payloads.retrieveTx(blockID)(tx)
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
}

func (b *Blocks) retrieveProposalTx(blockID flow.Identifier) func(*badger.Txn) (*flow.Proposal, error) {
	return func(tx *badger.Txn) (*flow.Proposal, error) {
		block, err := b.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve block body: %w", err)
		}
		sig, err := b.headers.sigs.retrieveTx(blockID)(tx)
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
}

// Store ...
func (b *Blocks) Store(proposal *flow.Proposal) error {
	return operation.RetryOnConflictTx(b.db, transaction.Update, b.StoreTx(proposal))
}

// ByID ...
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()
	return b.retrieveTx(blockID)(tx)
}

func (b *Blocks) ProposalByID(blockID flow.Identifier) (*flow.Proposal, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()
	return b.retrieveProposalTx(blockID)(tx)
}

// ByHeight returns block by height
//
// Expected errors during normal operations:
// - storage.ErrNotFound if no block is found for the given height
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()

	blockID, err := b.headers.retrieveIdByHeightTx(height)(tx)
	if err != nil {
		return nil, err
	}
	return b.retrieveTx(blockID)(tx)
}

func (b *Blocks) ProposalByHeight(height uint64) (*flow.Proposal, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()

	blockID, err := b.headers.retrieveIdByHeightTx(height)(tx)
	if err != nil {
		return nil, err
	}
	return b.retrieveProposalTx(blockID)(tx)
}

// ByCollectionID ...
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	guarantee, err := b.payloads.guarantees.ByCollectionID(collID)
	if err != nil {
		return nil, fmt.Errorf("could not look up guarantee: %w", err)
	}

	err = b.db.View(operation.LookupCollectionGuaranteeBlock(guarantee.ID(), &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollectionGuarantees creates an index `guaranteeID->blockID` for each guarantee
// which appears in the block.
// No errors are expected during normal operation.
func (b *Blocks) IndexBlockForCollectionGuarantees(blockID flow.Identifier, guaranteeIDs []flow.Identifier) error {
	for _, guaranteeID := range guaranteeIDs {
		err := operation.RetryOnConflict(b.db.Update, operation.SkipDuplicates(operation.IndexCollectionGuaranteeBlock(guaranteeID, blockID)))
		if err != nil {
			return fmt.Errorf("could not index block (id=%x) by guarantee (id=%x): %w", blockID, guaranteeID, err)
		}
	}
	return nil
}
