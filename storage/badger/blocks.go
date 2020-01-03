// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
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

func (b *Blocks) ByHash(hash crypto.Hash) (*flow.Block, error) {

	var block *flow.Block
	err := b.db.View(func(tx *badger.Txn) error {

		var err error
		block, err = b.retrieveBlock(tx, hash)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
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
		var hash crypto.Hash
		err := operation.RetrieveHash(number, &hash)(tx)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return fmt.Errorf("could not retrieve hash: %w", err)
		}

		block, err = b.retrieveBlock(tx, hash)
		if err != nil {
			if err == storage.NotFoundErr {
				return err
			}
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		return nil
	})

	return block, err
}

func (b *Blocks) retrieveBlock(tx *badger.Txn, hash crypto.Hash) (*flow.Block, error) {

	// get the header
	var header flow.Header
	err := operation.RetrieveHeader(hash, &header)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the new identities
	var identities flow.IdentityList
	err = operation.RetrieveIdentities(hash, &identities)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}

	// get the collection guarantees
	var guarantees []*flow.CollectionGuarantee
	err = operation.RetrieveCollectionGuaranteesByBlockHash(hash, &guarantees)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retreive collection guarantees: %w", err)
	}

	// create the block
	block := &flow.Block{
		Header:               header,
		NewIdentities:        identities,
		CollectionGuarantees: guarantees,
	}

	return block, nil
}

func (b *Blocks) Save(block *flow.Block) error {

	err := b.db.Update(func(tx *badger.Txn) error {

		err := operation.PersistHeader(&block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not save header: %w", err)
		}

		err = operation.PersistIdentities(block.Hash(), block.NewIdentities)(tx)
		if err != nil {
			return fmt.Errorf("could not save block: %w", err)
		}

		for _, gc := range block.CollectionGuarantees {
			err = operation.PersistCollectionGuarantee(gc)(tx)
			if err != nil {
				return fmt.Errorf("could not save guaranteed collection: %w", err)
			}
			err = operation.IndexCollectionGuaranteeByBlockHash(block.Hash(), gc)(tx)
			if err != nil {
				return fmt.Errorf("could not index guaranteed collection by block hash: %w", err)
			}
		}

		err = operation.PersistHash(block.Number, block.Hash())(tx)
		if err != nil {
			return fmt.Errorf("could not save block hash: %w", err)
		}

		return nil
	})
	return err

}
