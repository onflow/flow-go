// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	header := &flow.HeaderFields{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload(flow.ZeroID)

	block := &Block{
		Header: header,
	}
	block.SetPayload(payload)

	return block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	// TODO: (Uliana) rename to HeaderFields or to smth new after merge
	Header  *flow.HeaderFields
	Payload *Payload
}

// ID returns a collision-resistant hash of the Block struct.
func (b *Block) ID() flow.Identifier {
	return flow.MakeID(b)
}

// TODO: (Uliana) rename to Header()
// ToHeader hashes the payload of the block.
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderFields: *b.Header,
		PayloadHash:  b.Payload.Hash(),
	}
}

// TODO: (Uliana) remove usages of this; include the payload in struct initialization, or as an argument to a builder or builder function
// SetPayload sets the payload and payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = &payload
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
// TODO(malleability, #7100) clean up types
type BlockProposal struct {
	Block           *Block
	ProposerSigData []byte
}
