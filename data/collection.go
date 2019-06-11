package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// Collection represents a "collection" of Transactions.
type Collection struct {
	TransactionHashes []crypto.Hash
}

// Hash computes the hash over the necessary Collection data.
func (c Collection) Hash() crypto.Hash {
	bytes := EncodeAsBytes(c.TransactionHashes)
	return crypto.NewHash(bytes)
}