package unsynchronized

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Collections struct {
	collMutex   sync.RWMutex
	collections map[flow.Identifier]*flow.Collection

	lightCollMutex   sync.RWMutex
	lightCollections map[flow.Identifier]*flow.LightCollection

	txToLightCollMutex             sync.RWMutex
	transactionIDToLightCollection map[flow.Identifier]*flow.LightCollection
}

var _ storage.Collections = (*Collections)(nil)

func NewCollections() *Collections {
	return &Collections{
		collections:                    make(map[flow.Identifier]*flow.Collection),
		lightCollections:               make(map[flow.Identifier]*flow.LightCollection),
		transactionIDToLightCollection: make(map[flow.Identifier]*flow.LightCollection),
	}
}

// ByID returns the collection with the given ID, including all transactions within the collection.
// Returns storage.ErrNotFound if collection wasn't found.
func (c *Collections) ByID(collID flow.Identifier) (*flow.Collection, error) {
	c.collMutex.RLock()
	defer c.collMutex.RUnlock()

	val, ok := c.collections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// LightByID returns collection with the given ID. Only retrieves transaction hashes.
// Returns storage.ErrNotFound if collection wasn't found.
func (c *Collections) LightByID(collID flow.Identifier) (*flow.LightCollection, error) {
	c.lightCollMutex.RLock()
	defer c.lightCollMutex.RUnlock()

	val, ok := c.lightCollections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// LightByTransactionID returns the collection for the given transaction ID. Only retrieves transaction hashes.
// Returns storage.ErrNotFound if collection wasn't found.
func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	c.txToLightCollMutex.RLock()
	defer c.txToLightCollMutex.RUnlock()

	val, ok := c.transactionIDToLightCollection[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store inserts the collection keyed by ID and all constituent transactions.
// Returns no errors during normal operation.
func (c *Collections) Store(collection *flow.Collection) error {
	c.collMutex.Lock()
	defer c.collMutex.Unlock()

	c.collections[collection.ID()] = collection
	return nil
}

// StoreLight inserts the collection. It does not insert, nor check existence of, the constituent transactions.
// Returns no errors during normal operation.
func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	c.lightCollMutex.Lock()
	defer c.lightCollMutex.Unlock()

	c.lightCollections[collection.ID()] = collection
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
// Returns no errors during normal operation.
func (c *Collections) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	c.txToLightCollMutex.Lock()
	defer c.txToLightCollMutex.Unlock()

	c.lightCollections[collection.ID()] = collection
	for _, txID := range collection.Transactions {
		c.transactionIDToLightCollection[txID] = collection
	}

	return nil
}

// Remove removes the collection and all constituent transactions.
// Returns no errors during normal operation.
func (c *Collections) Remove(collID flow.Identifier) error {
	c.collMutex.Lock()
	delete(c.collections, collID)
	c.collMutex.Unlock()

	c.lightCollMutex.Lock()
	delete(c.lightCollections, collID)
	c.lightCollMutex.Unlock()

	c.txToLightCollMutex.Lock()
	for txID, coll := range c.transactionIDToLightCollection {
		if coll.ID() == collID {
			delete(c.transactionIDToLightCollection, txID)
		}
	}
	c.txToLightCollMutex.Unlock()

	return nil
}
