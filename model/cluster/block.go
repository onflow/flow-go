// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Genesis() *Block {
	header := flow.Header{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime(),
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload(flow.ZeroID)

	block := &Block{
		Header:  header,
		Payload: payload,
	}
	block.SetPayload(payload)

	return block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	flow.Header
	Payload
}

// SetPayload sets the payload and payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = payload
	b.PayloadHash = payload.Hash()
}

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
type Payload struct {

	// Collection is the collection being created.
	Collection flow.Collection

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

// Hash returns the hash of the payload, simply the ID of the collection.
func (p Payload) Hash() flow.Identifier {
	return flow.MakeID(p)
}

// PendingBlock is a wrapper type representing a block that cannot yet be
// processed. The block header, payload, and sender ID are stored together
// while waiting for the block to become processable.
type PendingBlock struct {
	OriginID flow.Identifier
	Header   *flow.Header
	Payload  *Payload
}
