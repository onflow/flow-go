// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	headerBody := flow.HeaderBody{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	}

	//TODO(Uliana: malleability immutable): decide if we need separate constuctor or do not use constructor here at all
	//nolint:structwrite
	return &Block{
		Header:  headerBody,
		Payload: *NewEmptyPayload(flow.ZeroID),
	}
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
//
//structwrite:immutable - mutations allowed only within the constructor
type Block struct {
	Header  flow.HeaderBody
	Payload Payload
}

// NewBlock creates a new block in collection node cluster consensus.
//
// Parameters:
// - headerBody: the header fields to use for the block
// - payload: the payload to associate with the block
func NewBlock(
	headerBody flow.HeaderBody,
	payload Payload,
) (*Block, error) {
	return &Block{
		Header:  headerBody,
		Payload: payload,
	}, nil
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return b.ToHeader().ID()
}

// ToHeader converts the block into a compact [flow.Header] representation,
// where the payload is compressed to a hash reference.
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderBody:  b.Header,
		PayloadHash: b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           Block
	ProposerSigData []byte
}
