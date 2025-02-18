package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Collections struct {
	db           storage.DB
	transactions *Transactions
}

func NewCollections(db storage.DB, transactions *Transactions) *Collections {
	c := &Collections{
		db:           db,
		transactions: transactions,
	}
	return c
}

func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.InsertCollection(rw.Writer(), collection)
	})

	if err != nil {
		return fmt.Errorf("could not insert collection: %w", err)
	}

	return nil
}

func (c *Collections) Store(collection *flow.Collection) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		light := collection.Light()
		err := operation.InsertCollection(rw.Writer(), &light)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, tx := range collection.Transactions {
			err = c.transactions.storeTx(rw, tx)
			if err != nil {
				return fmt.Errorf("could not insert transaction: %w", err)
			}
		}

		return nil
	})
}

func (c *Collections) ByID(colID flow.Identifier) (*flow.Collection, error) {
	var (
		light      flow.LightCollection
		collection flow.Collection
	)

	err := operation.RetrieveCollection(c.db.Reader(), colID, &light)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	for _, txID := range light.Transactions {
		tx, err := c.transactions.ByID(txID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve transaction: %w", err)
		}

		collection.Transactions = append(collection.Transactions, tx)
	}

	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) LightByID(colID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := operation.RetrieveCollection(c.db.Reader(), colID, &collection)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) Remove(colID flow.Identifier) error {
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveCollection(rw.Writer(), colID)
	})
	if err != nil {
		return fmt.Errorf("could not remove collection: %w", err)
	}
	return nil
}

func (c *Collections) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err := operation.InsertCollection(rw.Writer(), collection)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, txID := range collection.Transactions {
			err = operation.IndexCollectionByTransaction(rw.Writer(), txID, collection.ID())
			if err != nil {
				return fmt.Errorf("could not insert transaction ID: %w", err)
			}
		}

		return nil
	})
}

func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	collID := &flow.Identifier{}
	err := operation.RetrieveCollectionID(c.db.Reader(), txID, collID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection id: %w", err)
	}

	var collection flow.LightCollection
	err = operation.RetrieveCollection(c.db.Reader(), *collID, &collection)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	return &collection, nil
}
