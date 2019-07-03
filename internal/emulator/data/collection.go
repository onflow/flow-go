package data

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// Collection is a set of transactions.
type Collection struct {
	TransactionHashes []crypto.Hash
}

// Hash computes the hash over the necessary collection data.
func (c Collection) Hash() crypto.Hash {
	bytes := EncodeAsBytes(c.TransactionHashes)
	return crypto.NewHash(bytes)
}
