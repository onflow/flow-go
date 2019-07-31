package storage

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/data/keyvalue"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type Storage interface {
	InsertTransaction(types.SignedTransaction) error
	ContainsTransaction(crypto.Hash) bool
}

type DatabaseStorage struct {
	db keyvalue.DBConnector
}

func NewDatabaseStorage(db keyvalue.DBConnector) Storage {
	fmt.Println("migrating up")
	err := db.MigrateUp()
	fmt.Println(err)

	return &DatabaseStorage{db}
}
