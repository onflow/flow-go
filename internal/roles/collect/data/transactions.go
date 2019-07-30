package data

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func (d *DatabaseDAL) InsertTransaction(tx types.SignedTransaction) error {
	// TODO: implement InsertTransaction
	return nil
}

func (d *DatabaseDAL) ContainsTransaction(hash crypto.Hash) bool {
	// TODO: implement ContainsTransaction
	return false
}
