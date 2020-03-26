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
	db *badger.DB
}

func NewBlocks(db *badger.DB) *Blocks {
	b := &Blocks{
		db: db,
	}
	return b
}

func (b *Blocks) Store(block *flow.Block) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		return procedure.InsertBlock(block)(tx)
	})
	return err
}

func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {
	var block flow.Block
	err := b.db.View(func(tx *badger.Txn) error {
		return procedure.RetrieveBlock(blockID, &block)(tx)
	})
	return &block, err
}

func (b *Blocks) ByHeight(height uint64) (*flow.Block, error) {
	var block flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		// retrieve the block ID by height
		var blockID flow.Identifier
		err := operation.RetrieveNumber(height, &blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block ID: %w", err)
		}

		// retrieve the block by block ID
		var block flow.Block
		err = procedure.RetrieveBlock(blockID, &block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		return nil
	})

	return &block, err
}

func (b *Blocks) ByCollectionID(collectionID flow.Identifier) (*flow.Block, error) {
	var block flow.Block
	err := b.db.View(func(tx *badger.Txn) error {
		return procedure.RetrieveBlockByCollectionID(collectionID, &block)(tx)
	})
	return &block, err
}

func (b *Blocks) IndexByGuarantees(blockID flow.Identifier) error {
	err := b.db.Update(func(tx *badger.Txn) error {
		return procedure.IndexBlockByGuarantees(blockID)(tx)
	})
	return err
}
