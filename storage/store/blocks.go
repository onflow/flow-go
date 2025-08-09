package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
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

// TODO: to be removed
func (b *Blocks) StoreTx(block *flow.Block) func(*transaction.Tx) error {
	panic("StoreTx is deprecated, use BatchStore instead")
}

// BatchStore stores a valid block in a batch.
func (b *Blocks) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, block *flow.Block) error {
	err := b.headers.storeTx(rw, block.Header)
	if err != nil {
		return fmt.Errorf("could not store header %v: %w", block.Header.ID(), err)
	}
	err = b.payloads.storeTx(lctx, rw, block.ID(), block.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload: %w", err)
	}
	return nil
}

func (b *Blocks) retrieveTx(blockID flow.Identifier) (*flow.Block, error) {
	header, err := b.headers.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}
	payload, err := b.payloads.retrieveTx(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve payload: %w", err)
	}
	block := &flow.Block{
		Header:  header,
		Payload: payload,
	}
	return block, nil
}

// ByID returns the block with the given hash. It is available for
// finalized and ambiguous blocks.
// Expected errors during normal operations:
// - storage.ErrNotFound if no block is found
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieveTx(blockID)
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
	return b.retrieveTx(blockID)
}

// ByCollectionID returns the block for the given collection ID.
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupCollectionBlock(b.db.Reader(), collID, &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollections indexes the block each collection was
// included in. This should not be called when finalizing a block
func (b *Blocks) IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error {
	return b.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		for _, collID := range collIDs {
			err := operation.IndexCollectionBlock(rw.Writer(), collID, blockID)
			if err != nil {
				return fmt.Errorf("could not index collection block (%x): %w", collID, err)
			}
		}
		return nil
	})
}
