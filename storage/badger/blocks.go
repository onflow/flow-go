// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Blocks implements a simple block storage around a badger DB.
type Blocks struct {
	db       *badger.DB
	headers  *Headers
	payloads *Payloads
}

func NewBlocks(db *badger.DB, headers *Headers, payloads *Payloads) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  headers,
		payloads: payloads,
	}
	return b
}

func (b *Blocks) storeTx(block *flow.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
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

func (b *Blocks) Store(block *flow.Block) error {
	return operation.RetryOnConflict(b.db.Update, b.storeTx(block))
}

func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	return b.retrieveTx(blockID)(b.db.NewTransaction(false))
}

func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

func (b *Blocks) ByCollectionID(collID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.LookupCollectionBlock(collID, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

func (b *Blocks) IndexBlockForCollections(blockID flow.Identifier, collIDs []flow.Identifier) error {
	for _, collID := range collIDs {
		err := operation.RetryOnConflict(b.db.Update, operation.SkipDuplicates(operation.IndexCollectionBlock(collID, blockID)))
		if err != nil {
			return fmt.Errorf("could not index collection block (%x): %w", collID, err)
		}
	}
	return nil
}
