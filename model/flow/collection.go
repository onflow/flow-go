// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

type Collection struct {
	Transactions []*Transaction
}

// Hash returns the canonical hash of this collection.
func (c *Collection) Hash() crypto.Hash {
	return hash.DefaultHasher.ComputeHash(c.Encode())
}

// Encode returns the canonical encoding of this collection.
func (c *Collection) Encode() []byte {
	w := wrapCollection(*c)
	return encoding.DefaultEncoder.MustEncode(&w)
}

type collectionWrapper struct {
	Transactions []transactionWrapper
}

func wrapCollection(c Collection) collectionWrapper {
	transactions := make([]transactionWrapper, 0, len(c.Transactions))

	for i, tx := range c.Transactions {
		transactions[i] = wrapTransaction(*tx)
	}

	return collectionWrapper{
		Transactions: transactions,
	}
}
