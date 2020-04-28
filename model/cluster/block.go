// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Genesis() *Block {
	header := flow.Header{
		View:      0,
		ChainID:   "",
		Timestamp: flow.GenesisTime(),
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload()

	header.PayloadHash = payload.Hash()

	block := &Block{
		Header:  header,
		Payload: payload,
	}

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

	// Collection is the collection being proposed as part of this block.
	Collection flow.Collection

	// ReferenceBlockID is the ID of a block on the main chain (ie. that run by
	// consensus nodes). Since canonical staking information is maintained on
	// the main chain, we need to link cluster blocks to blocks on the main
	// chain in order to have a reference point for assessing validity of
	// cluster blocks.
	//
	// TODO: set this in builder, update storage layer etc. -- not used yet
	//
	//TODO: currently this is not checked by the proposal engine. For safety in
	// Byzantine environments, we need additional rules for this field to ensure
	// it remains up-to-date with the main chain.
	ReferenceBlockID flow.Identifier
}

// EmptyPayload returns a payload containing an empty collection and an un-set
// reference block ID.
func EmptyPayload() Payload {
	return PayloadFromTransactions()
}

// PayloadFromTransactions creates a payload given a list of transaction hashes.
func PayloadFromTransactions(transactions ...*flow.TransactionBody) Payload {
	// avoid a nil transaction list
	if len(transactions) == 0 {
		transactions = []*flow.TransactionBody{}
	}
	return Payload{
		Collection: flow.Collection{
			Transactions: transactions,
		},
		ReferenceBlockID: flow.ZeroID,
	}
}

// Hash returns the hash of the payload.
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
