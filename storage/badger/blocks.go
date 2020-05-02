// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Blocks implements a simple read-only block storage around a badger DB.
type Blocks struct {
	db       *badger.DB
	headers  *Headers
	payloads *Payloads
}

func NewBlocks(db *badger.DB) *Blocks {
	b := &Blocks{
		db:       db,
		headers:  NewHeaders(db),
		payloads: NewPayloads(db),
	}
	return b
}

func (b *Blocks) Store(block *flow.Block) error {
	err := b.headers.Store(block.Header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
	}
	err = b.payloads.Store(block.ID(), block.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload: %w", err)
	}
	return nil
}

func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}
	payload, err := b.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve payload: %w", err)
	}
	block := flow.Block{
		Header:  header,
		Payload: payload,
	}
	return &block, nil
}

func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.RetrieveNumber(height, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

func (b *Blocks) ByCollectionID(collectionID flow.Identifier) (*flow.Block, error) {
	var blockID flow.Identifier
	err := b.db.View(operation.LookupBlockIDByCollectionID(collectionID, &blockID))
	if err != nil {
		return nil, fmt.Errorf("could not look up block: %w", err)
	}
	return b.ByID(blockID)
}

func (b *Blocks) IndexByGuarantees(blockID flow.Identifier) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		return procedure.IndexBlockByGuarantees(blockID)(tx)
	})
	return err
}
