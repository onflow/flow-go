// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

func Genesis() *Block {
	header := flow.Header{
		Number:    0,
		ChainID:   "",
		Timestamp: time.Now(),
		ParentID:  flow.ZeroID,
	}

	payload := Payload{
		Collection: &flow.LightCollection{},
	}

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

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
type Payload struct {
	Collection *flow.LightCollection
}

// Hash returns the hash of the payload, simply the ID of the collection.
func (p Payload) Hash() flow.Identifier {
	return p.Collection.ID()
}
