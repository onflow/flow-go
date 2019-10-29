// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
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
