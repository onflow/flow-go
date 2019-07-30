package storage

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type Storage interface {
	InsertTransaction(types.SignedTransaction) error
	ContainsTransaction(crypto.Hash) bool
}

type DatabaseStorage struct{}

func NewDatabaseStorage() Storage {
	return &DatabaseStorage{}
}
