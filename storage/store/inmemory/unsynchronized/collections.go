package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Collections struct {
	//TODO: we don't need a mutex here as we have a guarantee by design
	// that we write data only once and it happens before the future reads.
	// We decided to leave a mutex for some time during active development.
	// It'll be removed in the future.
	lock                           sync.RWMutex
	collections                    map[flow.Identifier]*flow.Collection
	lightCollections               map[flow.Identifier]*flow.LightCollection
	transactionIDToLightCollection map[flow.Identifier]*flow.LightCollection

	transactions *Transactions // Reference to Transactions to store txs when storing collections
}

var _ storage.Collections = (*Collections)(nil)

func NewCollections(transactions *Transactions) *Collections {
	return &Collections{
		collections:                    make(map[flow.Identifier]*flow.Collection),
		lightCollections:               make(map[flow.Identifier]*flow.LightCollection),
		transactionIDToLightCollection: make(map[flow.Identifier]*flow.LightCollection),
		transactions:                   transactions,
	}
}

// ByID returns the collection with the given ID, including all transactions within the collection.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) ByID(collID flow.Identifier) (*flow.Collection, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	val, ok := c.collections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// LightByID returns collection with the given ID. Only retrieves transaction hashes.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) LightByID(collID flow.Identifier) (*flow.LightCollection, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	val, ok := c.lightCollections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// LightByTransactionID returns the collection for the given transaction ID. Only retrieves transaction hashes.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no light collection was found.
func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	val, ok := c.transactionIDToLightCollection[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store inserts the collection keyed by ID and all constituent transactions.
// No errors are expected during normal operation.
func (c *Collections) Store(collection *flow.Collection) (*flow.LightCollection, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.collections[collection.ID()] = collection
	light := collection.Light()
	return light, nil
}

// StoreAndIndexByTransaction inserts the light collection (only
// transaction IDs) and adds a transaction id index for each of the
// transactions within the collection (transaction_id->collection_id).
//
// CAUTION: current approach is NOT BFT and needs to be revised in the future.
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
// TODO: eventually we need to handle Byzantine clusters
//
// No errors are expected during normal operation.
func (c *Collections) StoreAndIndexByTransaction(_ lockctx.Proof, collection *flow.Collection) (*flow.LightCollection, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.collections[collection.ID()] = collection
	light := collection.Light()
	c.lightCollections[light.ID()] = light
	for _, txID := range light.Transactions {
		c.transactionIDToLightCollection[txID] = light
	}

	for _, tx := range collection.Transactions {
		if err := c.transactions.Store(tx); err != nil {
			return nil, fmt.Errorf("could not index transaction: %w", err)
		}
	}

	return light, nil
}

// Remove removes the collection and all constituent transactions.
// No errors are expected during normal operation.
func (c *Collections) Remove(collID flow.Identifier) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.collections, collID)
	delete(c.lightCollections, collID)
	for txID, coll := range c.transactionIDToLightCollection {
		if coll.ID() == collID {
			delete(c.transactionIDToLightCollection, txID)
		}
	}

	return nil
}

// Data returns a copy of stored collections
func (c *Collections) Data() []flow.Collection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	out := make([]flow.Collection, 0, len(c.collections))
	for _, coll := range c.collections {
		out = append(out, *coll)
	}
	return out
}

// LightCollections returns a copy of stored light collections
func (c *Collections) LightCollections() []flow.LightCollection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	out := make([]flow.LightCollection, 0, len(c.lightCollections))
	for _, coll := range c.lightCollections {
		out = append(out, *coll)
	}
	return out
}

// BatchStoreAndIndexByTransaction stores a light collection and indexes it by transaction ID within a batch operation.
//
// CAUTION: current approach is NOT BFT and needs to be revised in the future.
// Honest clusters ensure a transaction can only belong to one collection. However, in rare
// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
// produce multiple finalized collections (aka guaranteed collections) containing the same
// transaction repeatedly.
// TODO: eventually we need to handle Byzantine clusters
//
// This method is not implemented and will always return an error.
func (c *Collections) BatchStoreAndIndexByTransaction(_ lockctx.Proof, _ *flow.Collection, _ storage.ReaderBatchWriter) (*flow.LightCollection, error) {
	return nil, fmt.Errorf("not implemented")
}
