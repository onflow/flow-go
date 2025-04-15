package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Collections struct {
	db           storage.DB
	transactions *Transactions
	indexingByTx sync.Mutex
}

var _ storage.Collections = &Collections{}

func NewCollections(db storage.DB, transactions *Transactions) *Collections {
	c := &Collections{
		db:           db,
		transactions: transactions,
		indexingByTx: sync.Mutex{},
	}
	return c
}

// StoreLight stores a light collection in the database.
// any error returned are exceptions
func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertCollection(rw.Writer(), collection)
	})

	if err != nil {
		return fmt.Errorf("could not insert collection: %w", err)
	}

	return nil
}

// Store stores a collection in the database.
// any error returned are exceptions
func (c *Collections) Store(collection *flow.Collection) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		light := collection.Light()
		err := operation.UpsertCollection(rw.Writer(), &light)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for i, tx := range collection.Transactions {
			txID := light.Transactions[i]
			err = c.transactions.storeTx(rw, txID, tx)
			if err != nil {
				return fmt.Errorf("could not insert transaction: %w", err)
			}
		}

		return nil
	})
}

// ByID retrieves a collection by its ID.
func (c *Collections) ByID(colID flow.Identifier) (*flow.Collection, error) {
	var (
		light      flow.LightCollection
		collection flow.Collection
	)

	err := operation.RetrieveCollection(c.db.Reader(), colID, &light)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	collection.Transactions = make([]*flow.TransactionBody, 0, len(light.Transactions))

	for i, txID := range light.Transactions {
		tx, err := c.transactions.ByID(txID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve transaction %v: %w", txID, err)
		}

		collection.Transactions[i] = tx
	}

	return &collection, nil
}

// LightByID retrieves a light collection by its ID.
func (c *Collections) LightByID(colID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := operation.RetrieveCollection(c.db.Reader(), colID, &collection)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	return &collection, nil
}

// Remove removes a collection from the database.
// Remove does not error if the collection does not exist
// any error returned are exceptions
func (c *Collections) Remove(colID flow.Identifier) error {
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveCollection(rw.Writer(), colID)
	})
	if err != nil {
		return fmt.Errorf("could not remove collection: %w", err)
	}
	return nil
}

// StoreLightAndIndexByTransaction stores a light collection and indexes it by transaction ID.
// It's concurrent-safe.
// any error returned are exceptions
func (c *Collections) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	// - This lock is to ensure there is no race condition when indexing collection by transaction ID
	// - The access node uses this index to report the transaction status. It's done by first
	//   find the collection for a given transaction ID, and then find the block by the collection,
	//   and then find the status of the block.
	// - since a transaction can belong to multiple collections, when indexing collection by transaction ID,
	//   if we overwrite the previous collection ID that was indexed by the same transaction ID, the access node
	//   will return different collection for the same transaction, and the transaction result status will be
	//   inconsistent.
	// - therefore, we need to check if the transaction is already indexed by a collection, and to
	//   make sure there is no dirty read, we need to use a lock to protect the indexing operation.
	// - Note, this approach works because this is the only place where UnsafeIndexCollectionByTransaction
	//   is used in the code base to index collection by transaction.
	c.indexingByTx.Lock()
	defer c.indexingByTx.Unlock()

	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err := operation.UpsertCollection(rw.Writer(), collection)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		for _, txID := range collection.Transactions {
			var differentColTxIsIn flow.Identifier
			err := operation.RetrieveCollectionID(rw.GlobalReader(), txID, &differentColTxIsIn)
			if err == nil {
				// transaction is already indexed by a different collection, we should not index it again
				// so that the access node will always return the same collection for a given transaction
				// and return a consistent transaction result status.
				continue
			}

			if errors.Is(err, storage.ErrNotFound) {
				// no other collection has this transaction, index this collection by this tx
				err = operation.UnsafeIndexCollectionByTransaction(rw.Writer(), txID, collection.ID())
				if err != nil {
					return fmt.Errorf("could not insert transaction ID: %w", err)
				}
				continue
			}

			return fmt.Errorf("could not retrieve collection by transaction id %v: %w", txID, err)
		}

		return nil
	})
}

// LightByTransactionID retrieves a light collection by a transaction ID.
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
