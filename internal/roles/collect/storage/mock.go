package storage

import (
	"errors"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// MockStorage is an in-memory storage implementation.
type MockStorage struct {
	transactions map[string]types.SignedTransaction
}

// NewMockStorage initializes and returns a new MockStorage.
func NewMockStorage() Storage {
	return &MockStorage{
		transactions: make(map[string]types.SignedTransaction),
	}
}

// InsertTransaction inserts a signed transaction into storage.
func (d *MockStorage) InsertTransaction(tx types.SignedTransaction) error {
	d.transactions[string(tx.Hash().Bytes())] = tx
	return nil
}

// GetTransaction returns the transaction with the provided hash.
//
// This function returns error if the hash does not exist in storage.
func (d *MockStorage) GetTransaction(hash crypto.Hash) (types.SignedTransaction, error) {
	tx, exists := d.transactions[string(hash.Bytes())]
	if !exists {
		return types.SignedTransaction{}, errors.New("transaction does not exist")
	}

	return tx, nil
}

// ContainsTransaction returns true if a transaction with the given hash exists
// in storage and false otherwise.
func (d *MockStorage) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := d.transactions[string(hash.Bytes())]
	return exists
}
