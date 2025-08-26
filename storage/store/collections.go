package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/utils/logging"
)

type Collections struct {
	db           storage.DB
	transactions *Transactions
	// TODO(7355): lockctx
	indexingByTx *sync.Mutex

	// TODO: Add caching -- this might be relatively frequently queried within the AN;
	//       likely predominantly with requests about recent transactions.
	//       Note that we already have caching for transactions.
}

var _ storage.Collections = (*Collections)(nil)

func NewCollections(db storage.DB, transactions *Transactions) *Collections {

	c := &Collections{
		db:           db,
		transactions: transactions,
		indexingByTx: new(sync.Mutex),
	}
	return c
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

		for _, tx := range collection.Transactions {
			err = c.transactions.storeTx(rw, tx)
			if err != nil {
				return fmt.Errorf("could not insert transaction: %w", err)
			}
		}

		return nil
	})
}

// ByID returns the collection with the given ID, including all
// transactions within the collection.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
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
			return nil, fmt.Errorf("could not retrieve transaction %v: %w", txID, err)
		}

		collection.Transactions = append(collection.Transactions, tx)
	}

	return &collection, nil
}

// LightByID returns a reduced representation of the collection with the given ID.
// The reduced collection references the constituent transactions by their hashes.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) LightByID(colID flow.Identifier) (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := operation.RetrieveCollection(c.db.Reader(), colID, &collection)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	return &collection, nil
}

// Remove removes a collection from the database, including all constituent transactions and
// indices inserted by Store.
// Remove does not error if the collection does not exist
// Note: this method should only be called for collections included in blocks below sealed height
// No errors are expected during normal operation.
func (c *Collections) Remove(colID flow.Identifier) error {
	col, err := c.LightByID(colID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// already removed
			return nil
		}

		return fmt.Errorf("could not retrieve collection: %w", err)
	}

	err = c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// remove transaction indices
		for _, txID := range col.Transactions {
			err = operation.RemoveCollectionTransactionIndices(rw.Writer(), txID)
			if err != nil {
				return fmt.Errorf("could not remove collection payload indices: %w", err)
			}
			// Honest clusters ensure a transaction can only belong to one collection. However, in rare
			// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
			// produce multiple finalized collections (aka guaranteed collections) containing the same
			// transaction repeadely.
			// TODO: For now we log a warning, but eventually we need to handle Byzantine clusters
			err = operation.RemoveTransaction(rw.Writer(), txID)
			if err != nil {
				return fmt.Errorf("could not remove transaction: %w", err)
			}
		}

		// remove the collection
		return operation.RemoveCollection(rw.Writer(), colID)
	})
	if err != nil {
		return fmt.Errorf("could not remove collection: %w", err)
	}
	return nil
}

// batchStoreLightAndIndexByTransaction stores a light collection and indexes it by transaction ID within a batch.
// This is the common implementation used by both StoreLightAndIndexByTransaction and BatchStoreLightAndIndexByTransaction.
// No errors are expected during normal operations
func (c *Collections) batchStoreLightAndIndexByTransaction(collection *flow.LightCollection, rw storage.ReaderBatchWriter) error {
	collectionID := collection.ID()

	err := operation.UpsertCollection(rw.Writer(), collection)
	if err != nil {
		return fmt.Errorf("could not insert collection: %w", err)
	}

	for _, txID := range collection.Transactions {
		var differentColTxIsIn flow.Identifier
		err := operation.LookupCollectionByTransaction(rw.GlobalReader(), txID, &differentColTxIsIn)
		if err == nil {
			// Honest clusters ensure a transaction can only belong to one collection. However, in rare
			// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
			// produce multiple finalized collections (aka guaranteed collections) containing the same
			// transaction repeadely.
			// TODO: For now we log a warning, but eventually we need to handle Byzantine clusters
			if collectionID != differentColTxIsIn {
				log.Error().
					Str(logging.KeySuspicious, "true").
					Hex("transaction hash", txID[:]).
					Hex("previously persisted collection containing transactions", differentColTxIsIn[:]).
					Hex("newly encountered collection containing transactions", collectionID[:]).
					Msgf("sanity check failed: transaction contained in multiple collections -- this is a symptom of a byzantine collector cluster (or a bug)")
			}
			continue
		}

		// the indexingByTx lock has ensured we are the only process indexing collection by transaction
		err = operation.UnsafeIndexCollectionByTransaction(rw.Writer(), txID, collectionID)
		if err != nil {
			return fmt.Errorf("could not insert transaction ID: %w", err)
		}
	}

	return nil
}

// StoreLightAndIndexByTransaction inserts the light collection (only
// transaction IDs) and adds a transaction id index for each of the
// transactions within the collection (transaction_id->collection_id).
//
// NOTE: Currently it is possible in rare circumstances for two collections
// to be guaranteed which both contain the same transaction (see https://github.com/dapperlabs/flow-go/issues/3556).
// The second of these will revert upon reaching the execution node, so
// this doesn't impact the execution state, but it can result in the Access
// node processing two collections which both contain the same transaction (see https://github.com/dapperlabs/flow-go/issues/5337).
// To handle this, we skip indexing the affected transaction when inserting
// the transaction_id->collection_id index when an index for the transaction
// already exists.
//
// No errors are expected during normal operation.
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

	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// TODO(7355): lockctx
		rw.Lock(c.indexingByTx)
		return c.batchStoreLightAndIndexByTransaction(collection, rw)
	})
}

// LightByTransactionID returns a reduced representation of the collection
// holding the given transaction ID. The reduced collection references the
// constituent transactions by their hashes.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	collID := &flow.Identifier{}
	err := operation.LookupCollectionByTransaction(c.db.Reader(), txID, collID)
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

// BatchStoreLightAndIndexByTransaction stores a light collection and indexes it by transaction ID within a batch operation.
// No errors are expected during normal operations
func (c *Collections) BatchStoreLightAndIndexByTransaction(collection *flow.LightCollection, batch storage.ReaderBatchWriter) error {
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
	batch.AddCallback(func(err error) {
		defer c.indexingByTx.Unlock()
	})

	if err := c.batchStoreLightAndIndexByTransaction(collection, batch); err != nil {
		return err
	}

	return nil
}
