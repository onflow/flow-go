package data

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type DAL interface {
	InsertTransaction(types.SignedTransaction) error
	ContainsTransaction(hash crypto.Hash) bool
}

type DatabaseDAL struct{}

func New() *DatabaseDAL {
	return &DatabaseDAL{}
}
