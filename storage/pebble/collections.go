package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type Collections struct {
	db           *pebble.DB
	transactions *Transactions
}

func NewCollections(db *pebble.DB, transactions *Transactions) *Collections {
	c := &Collections{
		db:           db,
		transactions: transactions,
	}
	return c
}

func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	err := operation.InsertCollection(collection)(c.db)
	if err != nil {
		return fmt.Errorf("could not insert collection: %w", err)
	}

	return nil
}

func (c *Collections) Store(collection *flow.Collection) error {
	light := collection.Light()
	return operation.BatchUpdate(c.db, func(ttx operation.PebbleReaderWriter) error {
		err := operation.InsertCollection(&light)(ttx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, tx := range collection.Transactions {
			err = c.transactions.storeTx(tx)(ttx)
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

	err := operation.RetrieveCollection(colID, &light)(c.db)
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

	return &collection, nil
}

func (c *Collections) LightByID(colID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := operation.RetrieveCollection(colID, &collection)(c.db)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (c *Collections) Remove(colID flow.Identifier) error {
	err := operation.RemoveCollection(colID)(c.db)
	if err != nil {
		return fmt.Errorf("could not remove collection: %w", err)
	}
	return nil
}

func (c *Collections) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	return operation.BatchUpdate(c.db, func(tx operation.PebbleReaderWriter) error {

		err := operation.InsertCollection(collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, txID := range collection.Transactions {
			err = operation.IndexCollectionByTransaction(txID, collection.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not insert transaction ID: %w", err)
			}
		}

		return nil
	})
}

func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection
	collID := &flow.Identifier{}
	err := operation.RetrieveCollectionID(txID, collID)(c.db)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection id: %w", err)
	}

	err = operation.RetrieveCollection(*collID, &collection)(c.db)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	return &collection, nil
}
