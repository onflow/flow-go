package mocks

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type MockCollectionStore struct {
	byID map[flow.Identifier]*flow.Collection
}

func NewMockCollectionStore() *MockCollectionStore {
	return &MockCollectionStore{
		byID: make(map[flow.Identifier]*flow.Collection),
	}
}

func (m *MockCollectionStore) ByID(id flow.Identifier) (*flow.Collection, error) {
	c, ok := m.byID[id]
	if !ok {
		return nil, fmt.Errorf("collection %s not found: %w", id, storage.ErrNotFound)
	}
	return c, nil
}

func (m *MockCollectionStore) Store(c *flow.Collection) error {
	m.byID[c.ID()] = c
	return nil
}

func (m *MockCollectionStore) StoreLightAndIndexByTransaction(collection *flow.LightCollection) error {
	panic("StoreLightIndexByTransaction not implemented")
}

func (m *MockCollectionStore) StoreLight(collection *flow.LightCollection) error {
	panic("StoreLight not implemented")
}

func (m *MockCollectionStore) Remove(id flow.Identifier) error {
	delete(m.byID, id)
	return nil
}

func (m *MockCollectionStore) LightByID(id flow.Identifier) (*flow.LightCollection, error) {
	panic("LightByID not implemented")
}

func (m *MockCollectionStore) LightByTransactionID(id flow.Identifier) (*flow.LightCollection, error) {
	panic("LightByTransactionID not implemented")
}
