package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/utils/logging"
)

type Collections struct {
	db           storage.DB
	transactions *Transactions

	// TODO: Add caching -- this might be relatively frequently queried within the AN;
	//       likely predominantly with requests about recent transactions.
	//       Note that we already have caching for transactions.
}

var _ storage.Collections = (*Collections)(nil)

func NewCollections(db storage.DB, transactions *Transactions) *Collections {

	c := &Collections{
		db:           db,
		transactions: transactions,
	}
	return c
}

// Store stores a collection in the database.
// any error returned are exceptions
func (c *Collections) Store(collection *flow.Collection) (*flow.LightCollection, error) {
	light := collection.Light()
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err := operation.UpsertCollection(rw.Writer(), light)
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

	if err != nil {
		return nil, err
	}
	return light, nil
}

// ByID returns the collection with the given ID, including all
// transactions within the collection.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) ByID(colID flow.Identifier) (*flow.Collection, error) {
	var (
		light flow.LightCollection
		txs   []*flow.TransactionBody
	)

	err := operation.RetrieveCollection(c.db.Reader(), colID, &light)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve collection: %w", err)
	}

	txs = make([]*flow.TransactionBody, 0, len(light.Transactions))
	for _, txID := range light.Transactions {
		tx, err := c.transactions.ByID(txID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve transaction %v: %w", txID, err)
		}

		txs = append(txs, tx)
	}

	collection, err := flow.NewCollection(flow.UntrustedCollection{Transactions: txs})
	if err != nil {
		return nil, fmt.Errorf("could not construct collection: %w", err)
	}

	return collection, nil
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

			err = c.transactions.RemoveBatch(rw, txID)
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

// BatchStoreAndIndexByTransaction stores a collection and indexes it by transaction ID within a batch.
//
// CAUTION: current approach is NOT BFT and needs to be revised in the future.
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
// TODO: eventually we need to handle Byzantine clusters
//
// No errors are expected during normal operations
func (c *Collections) BatchStoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection, rw storage.ReaderBatchWriter) (*flow.LightCollection, error) {
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
	// - Note, this approach works because this is the only place where [operation.IndexCollectionByTransaction]
	//   is used in the code base to index collection by transaction.
	if !lctx.HoldsLock(storage.LockInsertCollection) {
		return nil, fmt.Errorf("missing lock: %v", storage.LockInsertCollection)
	}

	light := collection.Light()
	collectionID := light.ID()

	err := operation.UpsertCollection(rw.Writer(), light)
	if err != nil {
		return nil, fmt.Errorf("could not insert collection: %w", err)
	}

	for _, txID := range light.Transactions {
		var differentColTxIsIn flow.Identifier
		// The following is not BFT, because we can't handle the case where a transaction is included
		// in multiple collections. As long as we only have significantly less than 1/3 byzantine
		// collectors in the overall population (across all clusters) this should not happen.
		// TODO: For now we log a warning, but eventually we need to handle Byzantine clusters
		err := operation.LookupCollectionByTransaction(rw.GlobalReader(), txID, &differentColTxIsIn)
		if err == nil {
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
		err = operation.IndexCollectionByTransaction(lctx, rw.Writer(), txID, collectionID)
		if err != nil {
			return nil, fmt.Errorf("could not insert transaction ID: %w", err)
		}
	}

	// Store individual transactions
	for _, tx := range collection.Transactions {
		err = c.transactions.storeTx(rw, tx)
		if err != nil {
			return nil, fmt.Errorf("could not insert transaction: %w", err)
		}
	}

	return light, nil
}

// StoreAndIndexByTransaction stores a collection and indexes it by transaction ID.
// It's concurrent-safe.
//
// CAUTION: current approach is NOT BFT and needs to be revised in the future.
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
// TODO: eventually we need to handle Byzantine clusters
//
// No errors are expected during normal operation.
func (c *Collections) StoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection) (*flow.LightCollection, error) {
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
	var light *flow.LightCollection
	err := c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		var err error
		light, err = c.BatchStoreAndIndexByTransaction(lctx, collection, rw)
		return err
	})
	return light, err
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
