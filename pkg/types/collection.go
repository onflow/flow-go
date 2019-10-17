package types

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type Collection struct {
	Transactions *[]Transaction
}

func (col *Collection) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	var txData = []byte("Collection")
	for _, tx := range *col.Transactions {
		txData = append(txData, tx.CanonicalEncoding()...)
	}
	return hasher.ComputeHash(txData)
}
