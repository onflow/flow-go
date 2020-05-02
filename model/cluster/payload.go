package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
type Payload struct {
	Collection flow.Collection
}

func EmptyPayload() *Payload {
	return PayloadFromTransactions()
}

// PayloadFromTransactions creates a payload given a list of transaction hashes.
func PayloadFromTransactions(transactions ...*flow.TransactionBody) *Payload {
	// avoid a nil transaction list
	if len(transactions) == 0 {
		transactions = []*flow.TransactionBody{}
	}
	return &Payload{
		Collection: flow.Collection{
			Transactions: transactions,
		},
	}
}

// Hash returns the hash of the payload, simply the ID of the collection.
func (p Payload) Hash() flow.Identifier {
	return p.Collection.ID()
}
