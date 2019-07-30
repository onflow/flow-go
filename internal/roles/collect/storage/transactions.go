package storage

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func (d *DatabaseStorage) InsertTransaction(tx types.SignedTransaction) error {
	// TODO: implement InsertTransaction
	return nil
}

func (d *DatabaseStorage) ContainsTransaction(hash crypto.Hash) bool {
	// TODO: implement ContainsTransaction
	return false
}
