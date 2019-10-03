package storage

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/data/keyvalue"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// Storage provides methods to store and retrieve data required by a collection node.
type Storage interface {
	// InsertTransaction inserts a signed transaction into storage.
	InsertTransaction(types.Transaction) error
	// GetTransaction returns the transaction with the provided hash.
	//
	// This function returns error if the hash does not exist in storage.
	GetTransaction(crypto.Hash) (types.Transaction, error)
	// ContainsTransaction returns true if a transaction with the given hash exists
	// in storage and false otherwise.
	ContainsTransaction(crypto.Hash) bool
}

// DatabaseStorage is a storage implementation backed by a key-value database.
type DatabaseStorage struct {
	db keyvalue.DBConnector
}

// NewDatabaseStorage returns a DatabaseStorage instance backed by the provided database.
func NewDatabaseStorage(db keyvalue.DBConnector) Storage {
	return &DatabaseStorage{db}
}

// InsertTransaction inserts a signed transaction into storage.
func (d *DatabaseStorage) InsertTransaction(tx types.Transaction) error {
	// TODO: implement InsertTransaction
	return nil
}

// GetTransaction returns the transaction with the provided hash.
//
// This function returns error if the hash does not exist in storage.
func (d *DatabaseStorage) GetTransaction(crypto.Hash) (types.Transaction, error) {
	// TODO: implement GetTransaction
	return types.Transaction{}, nil
}

// ContainsTransaction returns true if a transaction with the given hash exists
// in storage and false otherwise.
func (d *DatabaseStorage) ContainsTransaction(hash crypto.Hash) bool {
	// TODO: implement ContainsTransaction
	return false
}
