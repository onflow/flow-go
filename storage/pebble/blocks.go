package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// Blocks implements a simple block storage around a pebble DB.
type Blocks struct {
	db       *pebble.DB
	headers  *Headers
	payloads *Payloads
}

var _ storage.Blocks = (*Blocks)(nil)

// NewBlocks ...
func NewBlocks(db *pebble.DB, headers *Headers, payloads *Payloads) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

// Ignored
func (b *Blocks) StoreTx(block *flow.Block) func(*transaction.Tx) error {
	return nil
}

func (b *Blocks) StoreBatch(block *flow.Block) func(storage.PebbleReaderBatchWriter) error {
	return b.storeTx(block)
}

func (b *Blocks) storeTx(block *flow.Block) func(storage.PebbleReaderBatchWriter) error {
	return func(rw storage.PebbleReaderBatchWriter) error {
		_, tx := rw.ReaderWriter()
		err := b.headers.storeTx(block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not store header %v: %w", block.Header.ID(), err)
		}
		err = b.payloads.storeTx(block.ID(), block.Payload)(rw)
		if err != nil {
			return fmt.Errorf("could not store payload: %w", err)
		}
		return nil
	}
}

func (b *Blocks) retrieveTx(blockID flow.Identifier) func(pebble.Reader) (*flow.Block, error) {
	return func(tx pebble.Reader) (*flow.Block, error) {
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

// Store ...
func (b *Blocks) Store(block *flow.Block) error {
	return b.storeTx(block)(operation.NewPebbleReaderBatchWriter(b.db))
}

// ByID ...
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieveTx(blockID)(b.db)
}

// ByHeight ...
func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	blockID, err := b.headers.retrieveIdByHeightTx(height)(b.db)
	if err != nil {
		return nil, err
	}
	return b.retrieveTx(blockID)(b.db)
}

// ByCollectionID ...
func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := operation.LookupCollectionBlock(collID, &blockID)(b.db)
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

// IndexBlockForCollections ...
func (b *Blocks) IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error {
	for _, collID := range collIDs {
		err := operation.IndexCollectionBlock(collID, blockID)(b.db)
		if err != nil {
			return fmt.Errorf("could not index collection block (%x): %w", collID, err)
		}
	}
	return nil
}

// InsertLastFullBlockHeightIfNotExists inserts the last full block height
// Calling this function multiple times is a no-op and returns no expected errors.
func (b *Blocks) InsertLastFullBlockHeightIfNotExists(height uint64) error {
	return operation.InsertLastCompleteBlockHeightIfNotExists(height)(b.db)
}

// UpdateLastFullBlockHeight upsert (update or insert) the last full block height
func (b *Blocks) UpdateLastFullBlockHeight(height uint64) error {
	return operation.InsertLastCompleteBlockHeight(height)(b.db)
}

// GetLastFullBlockHeight ...
func (b *Blocks) GetLastFullBlockHeight() (uint64, error) {
	var h uint64
	err := operation.RetrieveLastCompleteBlockHeight(&h)(b.db)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve LastFullBlockHeight: %w", err)
	}
	return h, nil
}
