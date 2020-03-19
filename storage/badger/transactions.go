package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Transactions struct {
	db *badger.DB
}

func NewTransactions(db *badger.DB) *Transactions {
	t := Transactions{
		db: db,
	}
	return &t
}

func (t *Transactions) Store(tx *flow.TransactionBody) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertTransaction(tx)(btx)
		if err != nil {
			return fmt.Errorf("could not insert transaction: %w", err)
		}
		return nil
	})
}

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {

	var tx flow.TransactionBody
	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveTransaction(txID, &tx)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction: %w", err)
		}
		return nil
	})

	return &tx, err
}

// CollectionID returns the collection for the given transaction ID
func (t *Transactions) CollectionID(txID flow.Identifier) (flow.Identifier, error) {
	collectionID := &flow.Identifier{}
	err := t.db.View(func(btx *badger.Txn) error {
		err := operation.LookupCollectionID(txID, collectionID)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve collection id: %w", err)
		}
		return nil
	})

	return *collectionID, err
}

// StoreCollectionID stores the collection ID for the given transaction ID
func (t *Transactions) StoreCollectionID(txID, collectionID flow.Identifier) error {
	return t.db.Update(func(btx *badger.Txn) error {
		err := operation.IndexCollectionByTransaction(txID, collectionID)(btx)
		if err != nil {
			return fmt.Errorf("could not insert collection ID: %w", err)
		}
		return nil
	})
}
