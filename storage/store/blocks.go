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
func (b *Blocks) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, block *flow.Block) error {
	err := b.headers.storeTx(lctx, rw, block.Header)
	if err != nil {
		return fmt.Errorf("could not store header %v: %w", block.Header.ID(), err)
	}
	err = b.payloads.storeTx(lctx, rw, block.ID(), block.Payload)
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
	return b.retrieve(blockID)
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

// ByCollectionID returns the *finalized** block that contains the collection with the given ID.
//
// Expected errors during normal operations:
// - storage.ErrNotFound if finalized block is known that contains the collection
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupCollectionGuaranteeBlock(b.db.Reader(), collID, &blockID)
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
			err := operation.IndexCollectionGuaranteeBlock(rw.Writer(), collID, blockID)
			if err != nil {
				return fmt.Errorf("could not index collection block (%x): %w", collID, err)
			}
		}
		return nil
	})
}
