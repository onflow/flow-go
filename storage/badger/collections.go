package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Collections struct {
	db *badger.DB
}

func NewCollections(db *badger.DB) *Collections {
	c := Collections{
		db: db,
	}
	return &c
}

func (c *Collections) ByFingerprint(hash flow.Fingerprint) (*flow.Collection, error) {
	var (
		light      flow.LightCollection
		collection flow.Collection
	)

	err := c.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveCollection(hash, &light)(btx)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return storage.NotFoundErr
			}
			return fmt.Errorf("could not retrieve collection: %w", err)
		}

		for _, txHash := range light.Transactions {
			var transaction flow.Transaction
			err = operation.RetrieveTransaction(txHash, &transaction)(btx)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					return storage.NotFoundErr
				}
				return fmt.Errorf("could not retrieve transaction: %w", err)
			}

			collection.Transactions = append(collection.Transactions, transaction.TransactionBody)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) Save(collection *flow.Collection) error {
	return c.db.Update(func(btx *badger.Txn) error {
		err := operation.PersistCollection(collection.Light())(btx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, txBody := range collection.Transactions {
			tx := flow.Transaction{TransactionBody: txBody}

			err = operation.PersistTransaction(&tx)(btx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *Collections) Remove(hash flow.Fingerprint) error {
	return c.db.Update(func(btx *badger.Txn) error {
		err := operation.RemoveCollection(hash)(btx)
		if err != nil {
			return fmt.Errorf("could not remove collection: %w", err)
		}
		return nil
	})
}
