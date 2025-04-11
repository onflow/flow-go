package unsynchronized

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Collections struct {
	collections                    map[flow.Identifier]*flow.Collection
	lightCollections               map[flow.Identifier]*flow.LightCollection
	transactionIDToLightCollection map[flow.Identifier]*flow.LightCollection
}

func NewCollections() *Collections {
	return &Collections{
		collections:                    make(map[flow.Identifier]*flow.Collection),
		lightCollections:               make(map[flow.Identifier]*flow.LightCollection),
		transactionIDToLightCollection: make(map[flow.Identifier]*flow.LightCollection),
	}
}

var _ storage.Collections = (*Collections)(nil)

func (c *Collections) ByID(collID flow.Identifier) (*flow.Collection, error) {
	val, ok := c.collections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (c *Collections) LightByID(collID flow.Identifier) (*flow.LightCollection, error) {
	val, ok := c.lightCollections[collID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (c *Collections) LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error) {
	val, ok := c.transactionIDToLightCollection[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (c *Collections) Store(collection *flow.Collection) error {
	c.collections[collection.ID()] = collection
	return nil
}

func (c *Collections) StoreLight(collection *flow.LightCollection) error {
	c.lightCollections[collection.ID()] = collection
	return nil
}

func (c *Collections) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	c.lightCollections[collection.ID()] = collection

	for _, txID := range collection.Transactions {
		c.transactionIDToLightCollection[txID] = collection
	}

	return nil
}

func (c *Collections) Remove(collID flow.Identifier) error {
	delete(c.collections, collID)
	delete(c.lightCollections, collID)
	for txID, coll := range c.transactionIDToLightCollection {
		if coll.ID() == collID {
			delete(c.transactionIDToLightCollection, txID)
		}
	}

	return nil
}

func (c *Collections) AddToBatch(batch storage.ReaderBatchWriter) error {
	writer := batch.Writer()

	for _, coll := range c.collections {
		light := coll.Light()
		err := operation.UpsertCollection(writer, &light)
		if err != nil {
			return fmt.Errorf("could not persist collection: %w", err)
		}
	}

	for _, coll := range c.lightCollections {
		err := operation.UpsertCollection(writer, coll)
		if err != nil {
			return fmt.Errorf("could not persist collection: %w", err)
		}
	}

	return nil
}
