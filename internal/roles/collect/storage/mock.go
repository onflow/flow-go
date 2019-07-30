package storage

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type MockStorage struct {
	transactions map[crypto.Hash]types.SignedTransaction
}

func NewMockStorage() Storage {
	return &MockStorage{
		transactions: make(map[crypto.Hash]types.SignedTransaction),
	}
}

func (d *MockStorage) InsertTransaction(tx types.SignedTransaction) error {
	d.transactions[tx.Hash()] = tx
	return nil
}

func (d *MockStorage) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := d.transactions[hash]
	return exists
}
