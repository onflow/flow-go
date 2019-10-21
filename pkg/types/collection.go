package types

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type Collection struct {
	Transactions []*Transaction
}

func (col *Collection) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	for _, tx := range col.Transactions {
		hasher.Add(tx.CanonicalEncoding())
	}
	return hasher.SumHash()
}
