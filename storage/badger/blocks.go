// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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

func (b *Blocks) StoreTx(block *flow.Block) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		err := b.headers.storeTx(block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not store header: %w", err)
		}
		err = b.payloads.storeTx(block.ID(), block.Payload)(tx)
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
		block := &flow.Block{
			Header:  header,
			Payload: payload,
		}
		return block, nil
	}
}

// Store ...
func (b *Blocks) Store(block *flow.Block) error {
	return operation.RetryOnConflictTx(b.db, transaction.Update, b.StoreTx(block))
}

// ByID ...
func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()
	return b.retrieveTx(blockID)(tx)
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

// UpdateLastFullBlockHeight upsert (update or insert) the last full block height
func (b *Blocks) UpdateLastFullBlockHeight(height uint64) error {
	return operation.RetryOnConflict(b.db.Update, func(tx *badger.Txn) error {

		// try to update
		err := operation.UpdateLastCompleteBlockHeight(height)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not update LastFullBlockHeight: %w", err)
		}

		// if key does not exist, try insert.
		err = operation.InsertLastCompleteBlockHeight(height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert LastFullBlockHeight: %w", err)
		}

		return nil
	})
}

// GetLastFullBlockHeight ...
func (b *Blocks) GetLastFullBlockHeight() (uint64, error) {
	var h uint64
	err := b.db.View(operation.RetrieveLastCompleteBlockHeight(&h))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve LastFullBlockHeight: %w", err)
	}
	return h, nil
}
