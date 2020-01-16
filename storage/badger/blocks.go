// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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

func (b *Blocks) ByID(blockID flow.Identifier) (*flow.Block, error) {

	var block *flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		var err error
		block, err = b.retrieveBlock(tx, blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		return nil
	})

	return block, err
}

func (b *Blocks) ByNumber(number uint64) (*flow.Block, error) {

	var block *flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		// get the hash
		var blockID flow.Identifier
		err := operation.RetrieveBlockID(number, &blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block ID: %w", err)
		}

		block, err = b.retrieveBlock(tx, blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		return nil
	})

	return block, err
}

func (b *Blocks) retrieveBlock(tx *badger.Txn, blockID flow.Identifier) (*flow.Block, error) {

	// get the header
	var header flow.Header
	err := operation.RetrieveHeader(blockID, &header)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the new identities
	var identities flow.IdentityList
	err = operation.RetrieveIdentities(blockID, &identities)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}

	// get the collection guarantees
	var guarantees []*flow.CollectionGuarantee
	err = operation.RetrieveGuarantees(blockID, &guarantees)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection guarantees: %w", err)
	}

	// create the block content
	content := flow.Content{
		Identities: identities,
		Guarantees: guarantees,
	}

	// deduce the block payload
	payload := content.Payload()

	// create the block
	block := &flow.Block{
		Header:  header,
		Payload: payload,
		Content: content,
	}

	return block, nil
}

func (b *Blocks) Store(block *flow.Block) error {

	err := b.db.Update(func(tx *badger.Txn) error {

		err := operation.PersistHeader(&block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not save header: %w", err)
		}

		err = operation.PersistIdentities(block.ID(), block.Identities)(tx)
		if err != nil {
			return fmt.Errorf("could not save block: %w", err)
		}

		for _, cg := range block.Guarantees {
			err = operation.PersistGuarantee(cg)(tx)
			if err != nil {
				return fmt.Errorf("could not save collection guarantee: %w", err)
			}
			err = operation.IndexGuarantee(block.ID(), cg)(tx)
			if err != nil {
				return fmt.Errorf("could not index collection guarantee by block ID: %w", err)
			}
		}

		return nil
	})

	return err
}
