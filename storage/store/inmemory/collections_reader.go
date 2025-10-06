package inmemory

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type CollectionsReader struct {
	collections                    map[flow.Identifier]flow.Collection
	lightCollections               map[flow.Identifier]*flow.LightCollection
	transactionIDToLightCollection map[flow.Identifier]*flow.LightCollection

	transactions *TransactionsReader // Reference to Transactions to store txs when storing collections
}

var _ storage.CollectionsReader = (*CollectionsReader)(nil)

func NewCollections(collections []*flow.Collection) *CollectionsReader {
	collectionsMap := make(map[flow.Identifier]flow.Collection)
	lightCollections := make(map[flow.Identifier]*flow.LightCollection)
	transactionIDToLightCollection := make(map[flow.Identifier]*flow.LightCollection)

	for _, collection := range collections {
		light := collection.Light()
		collectionID := light.ID()
		collectionsMap[collectionID] = *collection
		lightCollections[collectionID] = light
		for _, txID := range light.Transactions {
			transactionIDToLightCollection[txID] = light
		}
	}

	return &CollectionsReader{
		collections:                    collectionsMap,
		lightCollections:               lightCollections,
		transactionIDToLightCollection: transactionIDToLightCollection,
	}
}

// ByID returns the collection with the given ID, including all transactions within the collection.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no light collection was found.
func (c *CollectionsReader) ByID(collID flow.Identifier) (*flow.Collection, error) {
	val, ok := c.collections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

// LightByID returns collection with the given ID. Only retrieves transaction hashes.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no light collection was found.
func (c *CollectionsReader) LightByID(collID flow.Identifier) (*flow.LightCollection, error) {
	val, ok := c.lightCollections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// LightByTransactionID returns the collection for the given transaction ID. Only retrieves transaction hashes.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if no light collection was found.
func (c *CollectionsReader) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	val, ok := c.transactionIDToLightCollection[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}
