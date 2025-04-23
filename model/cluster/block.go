// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			View:      0,
			ChainID:   "cluster",
			Timestamp: flow.GenesisTime,
			ParentID:  flow.ZeroID,
		},
	}

	return NewBlock(header.HeaderFields(), EmptyPayload(flow.ZeroID))
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  *flow.HeaderBody
	Payload *Payload
}

// NewBlock creates a new block in collection node cluster consensus.
//
// Parameters:
// - headerBody: the header fields to use for the block
// - payload: the payload to associate with the block
func NewBlock(
	headerBody flow.HeaderBody,
	payload Payload,
) *Block {
	return &Block{
		Header:  &headerBody,
		Payload: &payload,
	}
}

// ID returns a collision-resistant hash of the cluster.Block struct.
func (b *Block) ID() flow.Identifier {
	return flow.MakeID(b)
}

// ToHeader return flow.Header data for cluster.Block
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:            b.Header.ChainID,
			ParentID:           b.Header.ParentID,
			Height:             b.Header.Height,
			Timestamp:          b.Header.Timestamp,
			View:               b.Header.View,
			ParentView:         b.Header.ParentView,
			ParentVoterIndices: b.Header.ParentVoterIndices,
			ParentVoterSigData: b.Header.ParentVoterSigData,
			ProposerID:         b.Header.ProposerID,
			LastViewTC:         b.Header.LastViewTC,
		},
		PayloadHash: b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
type BlockProposal struct {
	Block           *Block
	ProposerSigData []byte
}
