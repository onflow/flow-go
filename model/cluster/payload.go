package cluster

import (
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
)

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
type Payload struct {

	// Collection is the collection being created.
	Collection flow.Collection

	// ReferenceEpoch is the epoch counter to use as reference. This is only
	// valid for root blocks, since these are generated in advance of a reference
	// block being available.
	// TODO needs more thought: https://github.com/dapperlabs/flow-go/issues/4655
	ReferenceEpoch uint64

	// ReferenceBlockID is the ID of a reference block on the main chain. It
	// is defined as the ID of the reference block with the lowest height
	// from all transactions within the collection.
	//
	// This determines when the collection expires, using the same expiry rules
	// as transactions. It is also used as the reference point for committee
	// state (staking, etc.) when validating the containing block.
	ReferenceBlockID flow.Identifier
}

// EmptyPayload returns a payload with an empty collection and the given
// reference block ID.
func EmptyPayload(refID flow.Identifier) Payload {
	return PayloadFromTransactions(refID)
}

// PayloadFromTransactions creates a payload given a reference block ID and a
// list of transaction hashes.
func PayloadFromTransactions(refID flow.Identifier, transactions ...*flow.TransactionBody) Payload {
	// avoid a nil transaction list
	if len(transactions) == 0 {
		transactions = []*flow.TransactionBody{}
	}
	return Payload{
		Collection: flow.Collection{
			Transactions: transactions,
		},
		ReferenceBlockID: refID,
	}
}

// Hash returns the hash of the payload.
func (p Payload) Hash() flow.Identifier {
	return flow.MakeID(p)
}

func (p Payload) Fingerprint() []byte {
	return fingerprint.Fingerprint(struct {
		Collection       []byte
		ReferenceBlockID flow.Identifier
	}{
		Collection:       p.Collection.Fingerprint(),
		ReferenceBlockID: p.ReferenceBlockID,
	})
}
