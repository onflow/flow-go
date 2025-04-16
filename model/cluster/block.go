// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/onflow/flow-go/model/flow"
)

func Genesis() *Block {
	header := &flow.Header{
		View:      0,
		ChainID:   "cluster",
		Timestamp: flow.GenesisTime,
		ParentID:  flow.ZeroID,
	}

	// TODO: Uliana: refactor all usages HeaderFields() after PR #7100 will be fixed
	return NewBlock(header.HeaderFields(), EmptyPayload(flow.ZeroID))
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	Header  *flow.HeaderFields
	Payload *Payload
}

func NewBlock(
	header flow.HeaderFields,
	payload Payload,
) *Block {
	return &Block{
		Header:  &header,
		Payload: &payload,
	}
}

// ToHeader return flow.Header data for cluster.Block
func (b *Block) ToHeader() *flow.Header {
	return &flow.Header{
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
		PayloadHash:        b.Payload.Hash(),
	}
}

// BlockProposal represents a signed proposed block in collection node cluster consensus.
// TODO(malleability, #7100) clean up types
type BlockProposal struct {
	Block           *Block
	ProposerSigData []byte
}

// ID returns the ID of the underlying block header.
func (b *Block) ID() flow.Identifier {
	return flow.MakeID(b)
}
