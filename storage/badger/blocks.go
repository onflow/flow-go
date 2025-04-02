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
	sigs     *ProposalSignatures
}

// NewBlocks ...
func NewBlocks(db *badger.DB, headers *Headers, payloads *Payloads, sigs *ProposalSignatures) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
		sigs:     sigs,
	}
	return b
}

func (b *Blocks) StoreTx(block *flow.BlockProposal) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		blockID := block.Block.ID()
		err := b.headers.storeTx(block.Block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not store header %v: %w", blockID, err)
		}
		err = b.payloads.storeTx(blockID, block.Block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not store payload: %w", err)
		}
		err = b.sigs.StoreTx(blockID, block.ProposerSigData)(tx)
		if err != nil {
			return fmt.Errorf("could not store proposer signature: %w", err)
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
		block := &flow.Block{
			Header:  header,
			Payload: payload,
		}
		return block, nil
	}
}

func (b *Blocks) retrieveProposalTx(blockID flow.Identifier) func(*badger.Txn) (*flow.BlockProposal, error) {
	return func(tx *badger.Txn) (*flow.BlockProposal, error) {
		header, err := b.headers.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve header: %w", err)
		}
		payload, err := b.payloads.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve payload: %w", err)
		}
		sig, err := b.sigs.retrieveTx(blockID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve proposer signature: %w", err)
		}
		proposal := &flow.BlockProposal{
			Block: &flow.Block{
				Header:  header,
				Payload: payload,
			},
			ProposerSigData: sig,
		}
		return proposal, nil
	}
}

// Store ...
func (b *Blocks) Store(block *flow.BlockProposal) error {
	return operation.RetryOnConflictTx(b.db, transaction.Update, b.StoreTx(block))
}

// ByID ...
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()
	return b.retrieveTx(blockID)(tx)
}

func (b *Blocks) ProposalByID(blockID flow.Identifier) (*flow.BlockProposal, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()
	return b.retrieveProposalTx(blockID)(tx)
}

// ByHeight ...
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()

	blockID, err := b.headers.retrieveIdByHeightTx(height)(tx)
	if err != nil {
		return nil, err
	}
	return b.retrieveTx(blockID)(tx)
}

// ByCollectionID ...
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.LookupCollectionBlock(collID, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollections ...
func (b *Blocks) IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error {
	for _, collID := range collIDs {
		err := operation.RetryOnConflict(b.db.Update, operation.SkipDuplicates(operation.IndexCollectionBlock(collID, blockID)))
		if err != nil {
			return fmt.Errorf("could not index collection block (%x): %w", collID, err)
		}
	}
	return nil
}
