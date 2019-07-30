package data

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type MockDAL struct {
	transactions map[crypto.Hash]types.SignedTransaction
}

func NewMock() *MockDAL {
	return &MockDAL{
		transactions: make(map[crypto.Hash]types.SignedTransaction),
	}
}

func (d *MockDAL) InsertTransaction(tx types.SignedTransaction) error {
	d.transactions[tx.Hash()] = tx
	return nil
}

func (d *MockDAL) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := d.transactions[hash]
	return exists
}
