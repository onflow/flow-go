package storage

import (
	"errors"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// MockStorage is an in-memory storage implementation.
type MockStorage struct {
	transactions map[string]types.Transaction
}

// NewMockStorage initializes and returns a new MockStorage.
func NewMockStorage() Storage {
	return &MockStorage{
		transactions: make(map[string]types.Transaction),
	}
}

// InsertTransaction inserts a signed transaction into storage.
func (d *MockStorage) InsertTransaction(tx types.Transaction) error {
	d.transactions[string(tx.Hash())] = tx
	return nil
}

// GetTransaction returns the transaction with the provided hash.
//
// This function returns error if the hash does not exist in storage.
func (d *MockStorage) GetTransaction(hash crypto.Hash) (types.Transaction, error) {
	tx, exists := d.transactions[string(hash)]
	if !exists {
		return types.Transaction{}, errors.New("transaction does not exist")
	}

	return tx, nil
}

// ContainsTransaction returns true if a transaction with the given hash exists
// in storage and false otherwise.
func (d *MockStorage) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := d.transactions[string(hash)]
	return exists
}
