package store

import (
	"fmt"

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
func (b *Blocks) BatchStore(rw storage.ReaderBatchWriter, block *flow.Block) error {
	// require LockInsertBlock
	return b.BatchStoreWithStoringResults(rw, block, make(map[flow.Identifier]*flow.ExecutionResult))
}

// BatchStoreWithStoringResults stores multiple blocks as a batch.
// The additional storingResults parameter helps verify that each receipt in the block
// refers to a known result. This check is essential during bootstrapping
// when multiple blocks are stored together in a batch.
func (b *Blocks) BatchStoreWithStoringResults(rw storage.ReaderBatchWriter, block *flow.Block, storingResults map[flow.Identifier]*flow.ExecutionResult) error {
	// require LockInsertBlock
	err := b.headers.storeTx(rw, block.Header)
	if err != nil {
		return fmt.Errorf("could not store header %v: %w", block.Header.ID(), err)
	}
	err = b.payloads.storeTx(rw, block.ID(), block.Payload, storingResults)
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
	block := &flow.Block{
		Header:  header,
		Payload: payload,
	}
	return block, nil
}

// ByID returns the block with the given hash. It is available for
// finalized and pending blocks.
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

// ByCollectionID returns the *finalized** block that contains the collection with the given ID.
//
// Expected errors during normal operations:
// - storage.ErrNotFound if finalized block is known that contains the collection
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupCollectionBlock(b.db.Reader(), collID, &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollections indexes the block each collection was included in.
// CAUTION: a collection can be included in multiple *unfinalized* blocks. However, the implementation
// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
// *and* only in the absence of byzantine collector clusters (which the mature protocol must tolerate).
// Hence, this function should be treated as a temporary solution, which requires generalization
// (one-to-many mapping) for soft finality and the mature protocol. 
//
// No errors expected during normal operation.
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
