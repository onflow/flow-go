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

func (b *Blocks) StoreTx(block *flow.Block) func(*transaction.Tx) error {
	panic("StoreTx is deprecated, use BatchStore instead")
}

func (b *Blocks) BatchStore(rw storage.ReaderBatchWriter, block *flow.Block) error {
	return b.BatchStoreWithStoringResults(rw, block, make(map[flow.Identifier]*flow.ExecutionResult))
}

func (b *Blocks) BatchStoreWithStoringResults(rw storage.ReaderBatchWriter, block *flow.Block, storingResults map[flow.Identifier]*flow.ExecutionResult) error {
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

// Store ...
func (b *Blocks) Store(block *flow.Block) error {
	return b.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return b.BatchStore(rw, block)
	})
}

// ByID ...
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieveTx(blockID)
}

// ByHeight ...
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)
	if err != nil {
		return nil, err
	}
	return b.retrieveTx(blockID)
}

// ByCollectionID ...
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupCollectionBlock(b.db.Reader(), collID, &blockID)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollections ...
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
